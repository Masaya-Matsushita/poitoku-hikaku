package supabase

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/report"
)

// Sites は sites テーブルを id 順に返す（report.Source）。
func (c *Client) Sites(ctx context.Context) ([]report.Site, error) {
	var rows []report.Site
	err := c.do(ctx, http.MethodGet, "sites",
		url.Values{"select": {"id,name,is_active"}, "order": {"id.asc"}}, nil, nil, &rows)
	return rows, err
}

// CrawlLogs は crawled_on が from〜to（両端含む）の crawl_logs を日付・id 順に返す（report.Source）。
func (c *Client) CrawlLogs(ctx context.Context, from, to string) ([]report.CrawlLog, error) {
	var rows []struct {
		ID             int64             `json:"id"`
		SiteID         string            `json:"site_id"`
		CrawledOn      string            `json:"crawled_on"`
		StartedAt      time.Time         `json:"started_at"`
		FinishedAt     *time.Time        `json:"finished_at"`
		Status         string            `json:"status"`
		RequestCount   int               `json:"request_count"`
		OfferCount     int               `json:"offer_count"`
		ParsedCount    int               `json:"parsed_count"`
		ErrorCount     int               `json:"error_count"`
		AbortReason    *string           `json:"abort_reason"`
		Errors         []report.LogError `json:"errors"`
		CrawlerVersion *string           `json:"crawler_version"`
	}
	// PostgREST は同じキーの条件を and で重ねられないので、範囲は 2 つの列指定で表す
	q := url.Values{
		"select": {"id,site_id,crawled_on,started_at,finished_at,status,request_count,offer_count,parsed_count,error_count,abort_reason,errors,crawler_version"},
		"and":    {fmt.Sprintf("(crawled_on.gte.%s,crawled_on.lte.%s)", from, to)},
		"order":  {"crawled_on.asc,id.asc"},
	}
	if err := c.do(ctx, http.MethodGet, "crawl_logs", q, nil, nil, &rows); err != nil {
		return nil, err
	}
	out := make([]report.CrawlLog, 0, len(rows))
	for _, r := range rows {
		l := report.CrawlLog{
			ID: r.ID, SiteID: r.SiteID, CrawledOn: r.CrawledOn, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
			Status: r.Status, RequestCount: r.RequestCount, OfferCount: r.OfferCount, ParsedCount: r.ParsedCount,
			ErrorCount: r.ErrorCount, Errors: r.Errors,
		}
		if r.AbortReason != nil {
			l.AbortReason = *r.AbortReason
		}
		if r.CrawlerVersion != nil {
			l.CrawlerVersion = *r.CrawlerVersion
		}
		out = append(out, l)
	}
	return out, nil
}

// EmptyRewardCount は crawledOn の offer_snapshots のうち、reward_raw が空（真の抽出失敗）の行数を
// サイトごとに返す（report.Source）。offers を内部結合して site_id で絞る。
func (c *Client) EmptyRewardCount(ctx context.Context, siteID, crawledOn string) (int, error) {
	return c.count(ctx, "offer_snapshots", url.Values{
		"select":         {"id,offers!inner(site_id)"},
		"crawled_on":     {"eq." + crawledOn},
		"reward_raw":     {"eq."},
		"offers.site_id": {"eq." + siteID},
	})
}
