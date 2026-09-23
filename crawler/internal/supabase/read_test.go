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
		case "/rest/v1/offer_snapshots":
			if r.URL.Query().Get("offers.site_id") == "eq.moppy" {
				w.Header().Set("Content-Range", "0-0/2")
				w.Write([]byte(`[{"id":1}]`))
				return
			}
			w.Header().Set("Content-Range", "*/0")
			w.Write([]byte(`[]`))
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
	for _, want := range []string{"select=id%2Coffers%21inner%28site_id%29", "crawled_on=eq.2026-09-23", "reward_raw=eq.", "offers.site_id=eq.moppy", "prefer=count=exact", "range=0-0"} {
		if !strings.Contains(q, want) {
			t.Errorf("クエリに %q が無い: %s", want, q)
		}
	}

	n, err = c.EmptyRewardCount(ctx, "hapitas", "2026-09-23")
	if err != nil || n != 0 {
		t.Errorf("0 件の Content-Range（*/0）を扱えない: n=%d err=%v", n, err)
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
