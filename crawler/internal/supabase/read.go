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

// EmptyRewardCount は crawledOn に掲載されていた案件（offers.last_seen_on = crawledOn）のうち、
// 現在有効な還元額（valid_to が null）の reward_raw が空、つまり真の抽出失敗の件数を返す（report.Source）。
func (c *Client) EmptyRewardCount(ctx context.Context, siteID, crawledOn string) (int, error) {
	return c.count(ctx, "offer_snapshots", url.Values{
		"select":              {"id,offers!inner(site_id,last_seen_on)"},
		"valid_to":            {"is.null"},
		"reward_raw":          {"eq."},
		"offers.site_id":      {"eq." + siteID},
		"offers.last_seen_on": {"eq." + crawledOn},
	})
}

// ChangedCount は crawledOn に始まった区間の数 = 還元額が変わった案件と新規案件の合計を返す（report.Source）。
// ADR-0005 の容量試算（1 日あたりの変化率）を実測するための指標。
func (c *Client) ChangedCount(ctx context.Context, siteID, crawledOn string) (int, error) {
	return c.count(ctx, "offer_snapshots", url.Values{
		"select":         {"id,offers!inner(site_id)"},
		"valid_from":     {"eq." + crawledOn},
		"offers.site_id": {"eq." + siteID},
	})
}
