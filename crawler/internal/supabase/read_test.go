package supabase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newReadServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apikey") != "sb_secret_test" {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" prefer="+r.Header.Get("Prefer")+" range="+r.Header.Get("Range"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/v1/sites":
			w.Write([]byte(`[{"id":"hapitas","name":"ハピタス","is_active":true},{"id":"moppy","name":"モッピー","is_active":true}]`))
		case "/rest/v1/crawl_logs":
			w.Write([]byte(`[{"id":3,"site_id":"moppy","crawled_on":"2026-09-23","started_at":"2026-09-22T18:00:10+00:00","finished_at":"2026-09-22T18:08:02+00:00","status":"success","request_count":158,"offer_count":1791,"parsed_count":1785,"error_count":0,"abort_reason":null,"errors":[],"crawler_version":"5cad0a6"},` +
				`{"id":4,"site_id":"hapitas","crawled_on":"2026-09-23","started_at":"2026-09-22T18:00:12+00:00","finished_at":null,"status":"running","request_count":0,"offer_count":0,"parsed_count":0,"error_count":0,"abort_reason":null,"errors":[],"crawler_version":null}]`))
		case "/rest/v1/offers":
			q := r.URL.Query()
			switch {
			case q.Get("select") == "name,category" && q.Get("site_id") == "eq.moppy" && q.Get("last_seen_on") == "eq.2026-09-22":
				w.Write([]byte(`[{"name":"案件A","category":"アプリ"},{"name":"案件B","category":null}]`))
			case q.Get("site_id") == "eq.moppy" && q.Get("first_seen_on") == "eq.2026-09-23":
				w.Header().Set("Content-Range", "0-0/3")
				w.Write([]byte(`[{"id":1}]`))
			case q.Get("site_id") == "eq.moppy" && q.Get("last_seen_on") == "eq.2026-09-22":
				w.Header().Set("Content-Range", "0-0/94")
				w.Write([]byte(`[{"id":1}]`))
			default:
				w.Header().Set("Content-Range", "*/0")
				w.Write([]byte(`[]`))
			}
		case "/rest/v1/offer_snapshots":
			q := r.URL.Query()
			switch {
			case strings.HasPrefix(q.Get("select"), "offer_id,reward_raw,reward_points,offers") && q.Get("offers.site_id") == "eq.moppy" && q.Get("valid_from") == "eq.2026-09-23":
				w.Write([]byte(`[{"offer_id":"o1","reward_raw":"1,500P","reward_points":1500,"offers":{"name":"案件A"}},` +
					`{"offer_id":"o2","reward_raw":"2%","reward_points":null,"offers":{"name":"案件B"}},` +
					`{"offer_id":"o3","reward_raw":"300P","reward_points":300,"offers":{"name":"新規案件"}}]`))
			case q.Get("select") == "offer_id,reward_raw,reward_points" && q.Get("valid_to") == "not.is.null":
				// valid_to の新しい順。o1 は古い区間（800P）より新しい区間（1,000P）を採る
				w.Write([]byte(`[{"offer_id":"o1","reward_raw":"1,000P","reward_points":1000},` +
					`{"offer_id":"o2","reward_raw":"1,000P","reward_points":1000},` +
					`{"offer_id":"o1","reward_raw":"800P","reward_points":800}]`))
			case q.Get("offers.site_id") == "eq.moppy" && q.Get("valid_to") == "is.null":
				w.Header().Set("Content-Range", "0-0/2")
				w.Write([]byte(`[{"id":1}]`))
			case q.Get("offers.site_id") == "eq.moppy" && q.Get("valid_from") == "eq.2026-09-23":
				w.Header().Set("Content-Range", "0-0/57")
				w.Write([]byte(`[{"id":1}]`))
			default:
				w.Header().Set("Content-Range", "*/0")
				w.Write([]byte(`[]`))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestReadSitesAndCrawlLogs(t *testing.T) {
	srv, seen := newReadServer(t)
	c, _ := New(srv.URL, "sb_secret_test")
	ctx := context.Background()

	sites, err := c.Sites(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 || sites[0].ID != "hapitas" || sites[1].Name != "モッピー" || !sites[1].IsActive {
		t.Errorf("sites = %+v", sites)
	}

	logs, err := c.CrawlLogs(ctx, "2026-08-25", "2026-09-23")
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("logs = %d", len(logs))
	}
	if logs[0].ID != 3 || logs[0].OfferCount != 1791 || logs[0].CrawlerVersion != "5cad0a6" || logs[0].FinishedAt == nil || logs[0].AbortReason != "" {
		t.Errorf("logs[0] = %+v", logs[0])
	}
	if logs[1].Status != "running" || logs[1].FinishedAt != nil || logs[1].CrawlerVersion != "" {
		t.Errorf("logs[1] = %+v", logs[1])
	}
	if !strings.Contains((*seen)[1], "and=%28crawled_on.gte.2026-08-25%2Ccrawled_on.lte.2026-09-23%29") || !strings.Contains((*seen)[1], "order=crawled_on.asc%2Cid.asc") {
		t.Errorf("crawl_logs のクエリ = %s", (*seen)[1])
	}
}

func TestEmptyRewardCountUsesContentRange(t *testing.T) {
	srv, seen := newReadServer(t)
	c, _ := New(srv.URL, "sb_secret_test")
	ctx := context.Background()

	n, err := c.EmptyRewardCount(ctx, "moppy", "2026-09-23")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
	q := (*seen)[0]
	for _, want := range []string{"select=id%2Coffers%21inner%28site_id%2Clast_seen_on%29", "valid_to=is.null", "reward_raw=eq.", "offers.site_id=eq.moppy", "offers.last_seen_on=eq.2026-09-23", "prefer=count=exact", "range=0-0"} {
		if !strings.Contains(q, want) {
			t.Errorf("クエリに %q が無い: %s", want, q)
		}
	}

	n, err = c.EmptyRewardCount(ctx, "hapitas", "2026-09-23")
	if err != nil || n != 0 {
		t.Errorf("0 件の Content-Range（*/0）を扱えない: n=%d err=%v", n, err)
	}

	n, err = c.ChangedCount(ctx, "moppy", "2026-09-23")
	if err != nil || n != 57 {
		t.Errorf("ChangedCount = %d, %v, want 57", n, err)
	}
	q = (*seen)[2]
	for _, want := range []string{"valid_from=eq.2026-09-23", "offers.site_id=eq.moppy", "prefer=count=exact"} {
		if !strings.Contains(q, want) {
			t.Errorf("ChangedCount のクエリに %q が無い: %s", want, q)
		}
	}
}

func TestNewAndGoneOfferCount(t *testing.T) {
	srv, seen := newReadServer(t)
	c, _ := New(srv.URL, "sb_secret_test")
	ctx := context.Background()

	n, err := c.NewOfferCount(ctx, "moppy", "2026-09-23")
	if err != nil || n != 3 {
		t.Errorf("NewOfferCount = %d, %v, want 3", n, err)
	}
	n, err = c.GoneOfferCount(ctx, "moppy", "2026-09-22")
	if err != nil || n != 94 {
		t.Errorf("GoneOfferCount = %d, %v, want 94", n, err)
	}
	for i, want := range [][]string{
		{"/rest/v1/offers?", "first_seen_on=eq.2026-09-23", "site_id=eq.moppy", "prefer=count=exact", "range=0-0"},
		{"/rest/v1/offers?", "last_seen_on=eq.2026-09-22", "site_id=eq.moppy", "prefer=count=exact", "range=0-0"},
	} {
		for _, w := range want {
			if !strings.Contains((*seen)[i], w) {
				t.Errorf("クエリ %d に %q が無い: %s", i, w, (*seen)[i])
			}
		}
	}
}

func TestGoneOfferDetails(t *testing.T) {
	srv, seen := newReadServer(t)
	c, _ := New(srv.URL, "sb_secret_test")
	ctx := context.Background()

	got, err := c.GoneOfferDetails(ctx, "moppy", "2026-09-22")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "案件A" || got[0].Category != "アプリ" || got[1].Name != "案件B" || got[1].Category != "" {
		t.Errorf("got = %+v", got)
	}
	q := (*seen)[0]
	for _, want := range []string{"select=name%2Ccategory", "site_id=eq.moppy", "last_seen_on=eq.2026-09-22", "order=name.asc"} {
		if !strings.Contains(q, want) {
			t.Errorf("クエリに %q が無い: %s", want, q)
		}
	}
}

func TestRewardChanges(t *testing.T) {
	srv, seen := newReadServer(t)
	c, _ := New(srv.URL, "sb_secret_test")

	got, err := c.RewardChanges(context.Background(), "moppy", "2026-09-23")
	if err != nil {
		t.Fatal(err)
	}
	// 新規案件（直前の区間なし）は含まれない
	if len(got) != 2 {
		t.Fatalf("got = %+v", got)
	}
	a, b := got[0], got[1]
	if a.Name != "案件A" || a.PrevRaw != "1,000P" || a.NewRaw != "1,500P" || a.PrevPoints == nil || *a.PrevPoints != 1000 || a.NewPoints == nil || *a.NewPoints != 1500 {
		t.Errorf("案件A = %+v", a)
	}
	if b.Name != "案件B" || b.NewPoints != nil || b.PrevPoints == nil {
		t.Errorf("案件B = %+v", b)
	}
	for i, want := range [][]string{
		{"/rest/v1/offer_snapshots?", "valid_from=eq.2026-09-23", "offers.site_id=eq.moppy", "offers%21inner%28name%2Csite_id%29", "order=id.asc", "limit=1000", "offset=0"},
		{"/rest/v1/offer_snapshots?", "offer_id=in.%28o1%2Co2%2Co3%29", "valid_to=not.is.null", "order=valid_to.desc%2Cid.desc"},
	} {
		for _, w := range want {
			if !strings.Contains((*seen)[i], w) {
				t.Errorf("クエリ %d に %q が無い: %s", i, w, (*seen)[i])
			}
		}
	}
}

func TestCountRejectsMissingContentRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "sb_secret_test")
	if _, err := c.EmptyRewardCount(context.Background(), "moppy", "2026-09-23"); err == nil {
		t.Error("Content-Range 無しでエラーにならない")
	}
}
