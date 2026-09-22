package supabase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/crawl"
)

type call struct {
	method string
	path   string
	query  string
	prefer string
	rng    string
	body   []byte
}

// newServer は PostgREST の必要最小限を真似る。既存 offers は existing で与える。
func newServer(t *testing.T, existing []map[string]string) (*httptest.Server, *[]call) {
	t.Helper()
	var calls []call
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apikey") != "sb_secret_test" || r.Header.Get("Authorization") != "Bearer sb_secret_test" {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, call{
			method: r.Method, path: r.URL.Path, query: r.URL.RawQuery,
			prefer: r.Header.Get("Prefer"), rng: r.Header.Get("Range"), body: body,
		})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v1/offers":
			var from, to int
			fmt.Sscanf(r.Header.Get("Range"), "%d-%d", &from, &to)
			var page []map[string]string
			for i := from; i <= to && i < len(existing); i++ {
				page = append(page, existing[i])
			}
			if page == nil {
				page = []map[string]string{}
			}
			json.NewEncoder(w).Encode(page)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/v1/offers":
			var rows []map[string]any
			if err := json.Unmarshal(body, &rows); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			out := make([]map[string]string, 0, len(rows))
			for i, row := range rows {
				out = append(out, map[string]string{"id": fmt.Sprintf("uuid-%d", i), "url": row["url"].(string)})
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/v1/offer_snapshots":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/v1/crawl_logs":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`[{"id": 42}]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/rest/v1/crawl_logs":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
		}
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestNewValidates(t *testing.T) {
	if _, err := New("", "k"); err == nil {
		t.Error("URL 空でエラーにならない")
	}
	if _, err := New("https://x.supabase.co", ""); err == nil {
		t.Error("key 空でエラーにならない")
	}
	if _, err := New("not a url", "k"); err == nil {
		t.Error("不正な URL でエラーにならない")
	}
	c, err := New("https://x.supabase.co/", "k")
	if err != nil || c.baseURL != "https://x.supabase.co" {
		t.Errorf("末尾スラッシュが落ちていない: %v %q", err, c.baseURL)
	}
}

func TestCrawlLogLifecycle(t *testing.T) {
	srv, calls := newServer(t, nil)
	c, _ := New(srv.URL, "sb_secret_test")
	started := time.Date(2026, 9, 22, 18, 0, 0, 0, time.UTC)

	id, err := c.StartCrawlLog(context.Background(), "moppy", "2026-09-23", started, "abc1234")
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Errorf("id = %d", id)
	}
	start := (*calls)[0]
	if start.method != "POST" || start.path != "/rest/v1/crawl_logs" || start.prefer != "return=representation" {
		t.Errorf("start call = %+v", start)
	}
	var payload map[string]any
	json.Unmarshal(start.body, &payload)
	if payload["site_id"] != "moppy" || payload["crawled_on"] != "2026-09-23" || payload["status"] != "running" || payload["crawler_version"] != "abc1234" || payload["started_at"] != "2026-09-22T18:00:00Z" {
		t.Errorf("start payload = %v", payload)
	}

	sum := crawl.Summary{
		Status: "partial", FinishedAt: started.Add(10 * time.Minute),
		RequestCount: 150, OfferCount: 1200, ParsedCount: 1190, ErrorCount: 2,
		Errors: []crawl.ErrorEntry{{URL: "https://x/1", Message: "HTTP 404"}},
	}
	if err := c.FinishCrawlLog(context.Background(), id, sum); err != nil {
		t.Fatal(err)
	}
	finish := (*calls)[1]
	if finish.method != "PATCH" || finish.query != "id=eq.42" || finish.prefer != "return=minimal" {
		t.Errorf("finish call = %+v", finish)
	}
	json.Unmarshal(finish.body, &payload)
	if payload["status"] != "partial" || payload["request_count"] != float64(150) || payload["finished_at"] != "2026-09-22T18:10:00Z" {
		t.Errorf("finish payload = %v", payload)
	}
	if _, has := payload["abort_reason"]; has {
		t.Error("abort_reason が無いのに送っている")
	}
	if errs, ok := payload["errors"].([]any); !ok || len(errs) != 1 {
		t.Errorf("errors = %v", payload["errors"])
	}
}

func TestSaveOffersPreservesFirstSeenAndWritesSnapshots(t *testing.T) {
	existing := []map[string]string{{"url": "https://pc.moppy.jp/ad/detail.php?site_id=1", "first_seen_on": "2026-09-01"}}
	srv, calls := newServer(t, existing)
	c, _ := New(srv.URL, "sb_secret_test")

	pts := int64(10000)
	pct := 1.5
	offers := []crawl.Offer{
		{ExternalID: "1", Name: "既存", URL: "https://pc.moppy.jp/ad/detail.php?site_id=1", Category: "クレカ", RewardRaw: "10,000P", RewardPoints: &pts},
		{ExternalID: "2", Name: "新規", URL: "https://pc.moppy.jp/ad/detail.php?site_id=2", RewardRaw: "1.5%", RewardPercent: &pct},
		{Name: "抽出失敗", URL: "https://pc.moppy.jp/ad/detail.php?site_id=3", RewardRaw: "要確認"},
	}
	if err := c.SaveOffers(context.Background(), "moppy", "2026-09-23", offers); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 3 {
		t.Fatalf("calls = %d: %+v", len(*calls), *calls)
	}

	get := (*calls)[0]
	if get.method != "GET" || !strings.Contains(get.query, "site_id=eq.moppy") || !strings.Contains(get.query, "select=url%2Cfirst_seen_on") || get.rng != "0-999" {
		t.Errorf("get call = %+v", get)
	}

	up := (*calls)[1]
	if up.method != "POST" || up.path != "/rest/v1/offers" || !strings.Contains(up.query, "on_conflict=site_id%2Curl") || up.prefer != "resolution=merge-duplicates,return=representation" {
		t.Errorf("upsert call = %+v", up)
	}
	var rows []map[string]any
	json.Unmarshal(up.body, &rows)
	if len(rows) != 3 {
		t.Fatalf("upsert rows = %d", len(rows))
	}
	if rows[0]["first_seen_on"] != "2026-09-01" || rows[0]["last_seen_on"] != "2026-09-23" || rows[0]["site_id"] != "moppy" || rows[0]["category"] != "クレカ" || rows[0]["external_id"] != "1" {
		t.Errorf("既存行の payload = %v", rows[0])
	}
	if rows[1]["first_seen_on"] != "2026-09-23" || rows[1]["category"] != nil {
		t.Errorf("新規行の payload = %v", rows[1])
	}
	if rows[2]["external_id"] != nil {
		t.Errorf("external_id 空は null で送る: %v", rows[2])
	}

	snap := (*calls)[2]
	if snap.method != "POST" || snap.path != "/rest/v1/offer_snapshots" || !strings.Contains(snap.query, "on_conflict=offer_id%2Ccrawled_on") || snap.prefer != "resolution=merge-duplicates,return=minimal" {
		t.Errorf("snapshot call = %+v", snap)
	}
	json.Unmarshal(snap.body, &rows)
	if len(rows) != 3 {
		t.Fatalf("snapshot rows = %d", len(rows))
	}
	if rows[0]["offer_id"] != "uuid-0" || rows[0]["crawled_on"] != "2026-09-23" || rows[0]["reward_points"] != float64(10000) || rows[0]["reward_percent"] != nil {
		t.Errorf("snapshot[0] = %v", rows[0])
	}
	if rows[1]["offer_id"] != "uuid-1" || rows[1]["reward_percent"] != 1.5 || rows[1]["reward_points"] != nil {
		t.Errorf("snapshot[1] = %v", rows[1])
	}
	if rows[2]["reward_raw"] != "要確認" || rows[2]["reward_points"] != nil || rows[2]["reward_percent"] != nil {
		t.Errorf("snapshot[2] = %v", rows[2])
	}
}

func TestSaveOffersPaginatesExistingAndChunksWrites(t *testing.T) {
	existing := make([]map[string]string, 0, pageSize+1)
	for i := 0; i <= pageSize; i++ {
		existing = append(existing, map[string]string{"url": fmt.Sprintf("https://x/%d", i), "first_seen_on": "2026-01-01"})
	}
	srv, calls := newServer(t, existing)
	c, _ := New(srv.URL, "sb_secret_test")

	offers := make([]crawl.Offer, 0, chunkSize+1)
	for i := 0; i <= chunkSize; i++ {
		offers = append(offers, crawl.Offer{Name: "n", URL: fmt.Sprintf("https://x/%d", i), RewardRaw: "1P"})
	}
	if err := c.SaveOffers(context.Background(), "moppy", "2026-09-23", offers); err != nil {
		t.Fatal(err)
	}
	var gets, upserts, snaps int
	for _, cl := range *calls {
		switch {
		case cl.method == "GET":
			gets++
		case cl.path == "/rest/v1/offers":
			upserts++
		case cl.path == "/rest/v1/offer_snapshots":
			snaps++
		}
	}
	if gets != 2 || (*calls)[1].rng != fmt.Sprintf("%d-%d", pageSize, 2*pageSize-1) {
		t.Errorf("既存の読み出しが 2 ページになっていない: gets = %d, range = %q", gets, (*calls)[1].rng)
	}
	if upserts != 2 || snaps != 2 {
		t.Errorf("chunk 分割が効いていない: upserts = %d, snaps = %d", upserts, snaps)
	}
}

func TestAPIErrorCarriesStatusAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"permission denied for table offers"}`, http.StatusForbidden)
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "sb_secret_test")
	_, err := c.StartCrawlLog(context.Background(), "moppy", "2026-09-23", time.Now(), "")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 403 || !strings.Contains(apiErr.Body, "permission denied") {
		t.Fatalf("err = %v", err)
	}
}
