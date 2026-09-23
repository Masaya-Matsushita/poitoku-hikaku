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

// fakeDB は PostgREST の必要最小限を真似る。offers の id は "id:" + url で決める。
type fakeDB struct {
	existing []map[string]string // offers の既存行（id,url,first_seen_on,last_seen_on）
	current  []map[string]any    // offer_snapshots の現在有効な行（id,offer_id,valid_from,reward_raw）
}

func newServer(t *testing.T, db fakeDB) (*httptest.Server, *[]call) {
	t.Helper()
	var calls []call
	page := func(r *http.Request, n int) (int, int) {
		var from, to int
		fmt.Sscanf(r.Header.Get("Range"), "%d-%d", &from, &to)
		if to >= n {
			to = n - 1
		}
		return from, to
	}
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
			from, to := page(r, len(db.existing))
			out := []map[string]string{}
			for i := from; i <= to; i++ {
				out = append(out, db.existing[i])
			}
			json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v1/offer_snapshots":
			from, to := page(r, len(db.current))
			out := []map[string]any{}
			for i := from; i <= to; i++ {
				out = append(out, db.current[i])
			}
			json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/v1/offers":
			var rows []map[string]any
			if err := json.Unmarshal(body, &rows); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			out := make([]map[string]string, 0, len(rows))
			for _, row := range rows {
				u := row["url"].(string)
				out = append(out, map[string]string{"id": "id:" + u, "url": u})
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPatch && r.URL.Path == "/rest/v1/offer_snapshots":
			w.WriteHeader(http.StatusNoContent)
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

func offerRowFixture(url, first, last string) map[string]string {
	return map[string]string{"id": "id:" + url, "url": url, "first_seen_on": first, "last_seen_on": last}
}

func currentFixture(id int64, url, validFrom, reward string) map[string]any {
	return map[string]any{"id": id, "offer_id": "id:" + url, "valid_from": validFrom, "reward_raw": reward}
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
	srv, calls := newServer(t, fakeDB{})
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

// 区間方式（ADR-0005）：変化した案件だけ前の区間を閉じて新しい区間を作り、同じ還元額なら何もしない。
func TestSaveOffersWritesOnlyChanges(t *testing.T) {
	const (
		uA = "https://pc.moppy.jp/ad/detail.php?site_id=1" // 還元額そのまま
		uB = "https://pc.moppy.jp/ad/detail.php?site_id=2" // 1.5% → 2.0%
		uC = "https://pc.moppy.jp/ad/detail.php?site_id=3" // 新規
		uD = "https://pc.moppy.jp/ad/detail.php?site_id=4" // 新規、数値化できない
		uE = "https://pc.moppy.jp/ad/detail.php?site_id=5" // 変化。last_seen_on が区間開始より前という不整合 → 開始日で閉じる
	)
	db := fakeDB{
		existing: []map[string]string{
			offerRowFixture(uA, "2026-09-01", "2026-09-22"),
			offerRowFixture(uB, "2026-09-10", "2026-09-20"),
			offerRowFixture(uE, "2026-09-01", "2026-09-05"),
		},
		current: []map[string]any{
			currentFixture(11, uA, "2026-09-01", "10,000P"),
			currentFixture(12, uB, "2026-09-10", "1.5%"),
			currentFixture(15, uE, "2026-09-10", "500P"),
		},
	}
	srv, calls := newServer(t, db)
	c, _ := New(srv.URL, "sb_secret_test")

	pts := int64(10000)
	pct := 2.0
	pts300 := int64(300)
	pts800 := int64(800)
	offers := []crawl.Offer{
		{ExternalID: "1", Name: "A", URL: uA, Category: "クレカ", RewardRaw: "10,000P", RewardPoints: &pts},
		{ExternalID: "2", Name: "B", URL: uB, RewardRaw: "2.0%", RewardPercent: &pct},
		{ExternalID: "3", Name: "C", URL: uC, RewardRaw: "300P", RewardPoints: &pts300},
		{Name: "D", URL: uD, RewardRaw: ""},
		{ExternalID: "5", Name: "E", URL: uE, RewardRaw: "800P", RewardPoints: &pts800},
	}
	if err := c.SaveOffers(context.Background(), "moppy", "2026-09-23", offers); err != nil {
		t.Fatal(err)
	}

	// GET offers → GET 現在の区間 → POST offers（upsert）→ PATCH（閉じる、日付ごと）→ POST 区間
	var seq []string
	for _, cl := range *calls {
		seq = append(seq, cl.method+" "+strings.TrimPrefix(cl.path, "/rest/v1/"))
	}
	want := []string{"GET offers", "GET offer_snapshots", "POST offers", "PATCH offer_snapshots", "PATCH offer_snapshots", "POST offer_snapshots"}
	if strings.Join(seq, ",") != strings.Join(want, ",") {
		t.Fatalf("呼び出し順 = %v\nwant %v", seq, want)
	}

	getCur := (*calls)[1]
	for _, s := range []string{"valid_to=is.null", "offers.site_id=eq.moppy", "select=id%2Coffer_id%2Cvalid_from%2Creward_raw%2Coffers%21inner%28site_id%29"} {
		if !strings.Contains(getCur.query, s) {
			t.Errorf("現在の区間のクエリに %q が無い: %s", s, getCur.query)
		}
	}

	up := (*calls)[2]
	if !strings.Contains(up.query, "on_conflict=site_id%2Curl") || up.prefer != "resolution=merge-duplicates,return=representation" {
		t.Errorf("upsert call = %+v", up)
	}
	var rows []map[string]any
	json.Unmarshal(up.body, &rows)
	if len(rows) != 5 {
		t.Fatalf("upsert rows = %d", len(rows))
	}
	if rows[0]["first_seen_on"] != "2026-09-01" || rows[0]["last_seen_on"] != "2026-09-23" || rows[0]["category"] != "クレカ" {
		t.Errorf("既存行の payload = %v", rows[0])
	}
	if rows[2]["first_seen_on"] != "2026-09-23" || rows[2]["category"] != nil {
		t.Errorf("新規行の payload = %v", rows[2])
	}
	if rows[3]["external_id"] != nil {
		t.Errorf("external_id 空は null で送る: %v", rows[3])
	}

	// 閉じる：B は前回観測日 2026-09-20、E は last_seen_on（09-05）が開始日（09-10）より前なので 09-10。日付順
	p1, p2 := (*calls)[3], (*calls)[4]
	var b1, b2 map[string]any
	json.Unmarshal(p1.body, &b1)
	json.Unmarshal(p2.body, &b2)
	if p1.query != "id=in.%2815%29" || b1["valid_to"] != "2026-09-10" || p1.prefer != "return=minimal" {
		t.Errorf("PATCH 1 = %s %v", p1.query, b1)
	}
	if p2.query != "id=in.%2812%29" || b2["valid_to"] != "2026-09-20" {
		t.Errorf("PATCH 2 = %s %v", p2.query, b2)
	}

	// 新しい区間：B, C, D, E（A は無し）。valid_from = 当日、valid_to と crawled_on は送らない
	ins := (*calls)[5]
	if ins.prefer != "return=minimal" || ins.query != "" {
		t.Errorf("insert call = %+v", ins)
	}
	json.Unmarshal(ins.body, &rows)
	if len(rows) != 4 {
		t.Fatalf("insert rows = %d: %v", len(rows), rows)
	}
	ids := []string{}
	for _, r := range rows {
		ids = append(ids, r["offer_id"].(string))
		if r["valid_from"] != "2026-09-23" {
			t.Errorf("valid_from = %v", r["valid_from"])
		}
		for _, forbidden := range []string{"valid_to", "crawled_on"} {
			if _, has := r[forbidden]; has {
				t.Errorf("%s を送っている: %v", forbidden, r)
			}
		}
	}
	if strings.Join(ids, " ") != "id:"+uB+" id:"+uC+" id:"+uD+" id:"+uE {
		t.Errorf("insert offer_ids = %v", ids)
	}
	if rows[0]["reward_percent"] != 2.0 || rows[0]["reward_points"] != nil {
		t.Errorf("B = %v", rows[0])
	}
	if rows[2]["reward_raw"] != "" || rows[2]["reward_points"] != nil || rows[2]["reward_percent"] != nil {
		t.Errorf("D = %v", rows[2])
	}
}

func TestSaveOffersIsIdempotentWhenNothingChanged(t *testing.T) {
	const u = "https://hapitas.jp/item/detail/itemid/1/"
	db := fakeDB{
		existing: []map[string]string{offerRowFixture(u, "2026-09-23", "2026-09-23")},
		current:  []map[string]any{currentFixture(1, u, "2026-09-23", "1,100pt")},
	}
	srv, calls := newServer(t, db)
	c, _ := New(srv.URL, "sb_secret_test")
	pts := int64(1100)
	if err := c.SaveOffers(context.Background(), "hapitas", "2026-09-23", []crawl.Offer{{Name: "x", URL: u, RewardRaw: "1,100pt", RewardPoints: &pts}}); err != nil {
		t.Fatal(err)
	}
	for _, cl := range *calls {
		if cl.path == "/rest/v1/offer_snapshots" && cl.method != http.MethodGet {
			t.Errorf("変化が無いのに snapshots を書いた: %s %s", cl.method, cl.query)
		}
	}
	if len(*calls) != 3 {
		t.Errorf("calls = %d（GET offers, GET snapshots, POST offers のみのはず）", len(*calls))
	}
}

func TestSaveOffersPaginatesAndChunks(t *testing.T) {
	// 既存 1001 件（2 ページ）、全件の還元額が変わる → 閉じるのは 200 件ずつ 6 回、新規区間は 500 件ずつ 3 回
	n := pageSize + 1
	db := fakeDB{}
	offers := make([]crawl.Offer, 0, n)
	for i := 0; i < n; i++ {
		u := fmt.Sprintf("https://x/%d", i)
		db.existing = append(db.existing, offerRowFixture(u, "2026-01-01", "2026-09-22"))
		db.current = append(db.current, currentFixture(int64(i+1), u, "2026-01-01", "1P"))
		offers = append(offers, crawl.Offer{Name: "n", URL: u, RewardRaw: "2P"})
	}
	srv, calls := newServer(t, db)
	c, _ := New(srv.URL, "sb_secret_test")
	if err := c.SaveOffers(context.Background(), "moppy", "2026-09-23", offers); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, cl := range *calls {
		counts[cl.method+" "+strings.TrimPrefix(cl.path, "/rest/v1/")]++
	}
	want := map[string]int{"GET offers": 2, "GET offer_snapshots": 2, "POST offers": 3, "PATCH offer_snapshots": 6, "POST offer_snapshots": 3}
	for k, v := range want {
		if counts[k] != v {
			t.Errorf("%s = %d, want %d", k, counts[k], v)
		}
	}
	if (*calls)[1].rng != fmt.Sprintf("%d-%d", pageSize, 2*pageSize-1) {
		t.Errorf("offers の 2 ページ目の Range = %q", (*calls)[1].rng)
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
