package supabase

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	// 日付の範囲は and=(gte,lte) の 1 条件にまとめる
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

// NewOfferCount は crawledOn に初めて観測した案件（offers.first_seen_on = crawledOn）の数を返す（report.Source）。
func (c *Client) NewOfferCount(ctx context.Context, siteID, crawledOn string) (int, error) {
	return c.count(ctx, "offers", url.Values{
		"select":        {"id"},
		"site_id":       {"eq." + siteID},
		"first_seen_on": {"eq." + crawledOn},
	})
}

// GoneOfferCount は prevDate を最後に観測されなくなった案件（offers.last_seen_on = prevDate）の数を返す（report.Source）。
// 翌日のクロールで再び観測された案件は last_seen_on が進むので含まれない。
func (c *Client) GoneOfferCount(ctx context.Context, siteID, prevDate string) (int, error) {
	return c.count(ctx, "offers", url.Values{
		"select":       {"id"},
		"site_id":      {"eq." + siteID},
		"last_seen_on": {"eq." + prevDate},
	})
}

// GoneOfferDetails は GoneOfferCount と同じ条件（offers.last_seen_on = prevDate）の案件の
// 名前とカテゴリを名前順で返す（report.Source）。日次レポートの「消えた案件」の内訳に使う。
func (c *Client) GoneOfferDetails(ctx context.Context, siteID, prevDate string) ([]report.GoneOffer, error) {
	var rows []struct {
		Name     string  `json:"name"`
		Category *string `json:"category"`
	}
	err := c.do(ctx, http.MethodGet, "offers",
		url.Values{
			"select":       {"name,category"},
			"site_id":      {"eq." + siteID},
			"last_seen_on": {"eq." + prevDate},
			"order":        {"name.asc"},
		}, nil, nil, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]report.GoneOffer, 0, len(rows))
	for _, r := range rows {
		g := report.GoneOffer{Name: r.Name}
		if r.Category != nil {
			g.Category = *r.Category
		}
		out = append(out, g)
	}
	return out, nil
}

// rewardChangePageSize は 1 回の取得行数。PostgREST の既定の上限（1000）に合わせる。
const rewardChangePageSize = 1000

// rewardChangeChunk は直前の区間を引く時に 1 リクエストへ入れる offer_id の数（URL を長くしすぎない）。
const rewardChangeChunk = 100

// getAll は limit / offset で全ページを読み、rows の各ページを handle に渡す。order は必須（ページ境界を安定させる）。
func getAll[T any](ctx context.Context, c *Client, path string, q url.Values, handle func([]T)) error {
	for offset := 0; ; offset += rewardChangePageSize {
		qq := url.Values{}
		for k, v := range q {
			qq[k] = v
		}
		qq.Set("limit", fmt.Sprint(rewardChangePageSize))
		qq.Set("offset", fmt.Sprint(offset))
		var rows []T
		if err := c.do(ctx, http.MethodGet, path, qq, nil, nil, &rows); err != nil {
			return err
		}
		handle(rows)
		if len(rows) < rewardChangePageSize {
			return nil
		}
	}
}

// RewardChanges は crawledOn に始まった区間のうち、同じ案件に直前の区間（valid_to が null でない最新）があるものの
// 前後の還元額を返す（report.Source）。新規案件（直前の区間が無い）は含まない。
func (c *Client) RewardChanges(ctx context.Context, siteID, crawledOn string) ([]report.RewardChange, error) {
	type current struct {
		OfferID      string `json:"offer_id"`
		RewardRaw    string `json:"reward_raw"`
		RewardPoints *int   `json:"reward_points"`
		Offers       struct {
			Name string `json:"name"`
		} `json:"offers"`
	}
	var cur []current
	err := getAll(ctx, c, "offer_snapshots", url.Values{
		"select":         {"offer_id,reward_raw,reward_points,offers!inner(name,site_id)"},
		"valid_from":     {"eq." + crawledOn},
		"offers.site_id": {"eq." + siteID},
		"order":          {"id.asc"},
	}, func(rows []current) { cur = append(cur, rows...) })
	if err != nil {
		return nil, err
	}

	type previous struct {
		OfferID      string `json:"offer_id"`
		RewardRaw    string `json:"reward_raw"`
		RewardPoints *int   `json:"reward_points"`
	}
	prev := map[string]previous{} // offer_id → 直前の区間（valid_to の新しい順に読むので最初の 1 件）
	for start := 0; start < len(cur); start += rewardChangeChunk {
		end := min(start+rewardChangeChunk, len(cur))
		ids := make([]string, 0, end-start)
		for _, r := range cur[start:end] {
			ids = append(ids, r.OfferID)
		}
		err := getAll(ctx, c, "offer_snapshots", url.Values{
			"select":   {"offer_id,reward_raw,reward_points"},
			"offer_id": {"in.(" + strings.Join(ids, ",") + ")"},
			"valid_to": {"not.is.null"},
			"order":    {"valid_to.desc,id.desc"},
		}, func(rows []previous) {
			for _, p := range rows {
				if _, ok := prev[p.OfferID]; !ok {
					prev[p.OfferID] = p
				}
			}
		})
		if err != nil {
			return nil, err
		}
	}

	out := make([]report.RewardChange, 0, len(cur))
	for _, r := range cur {
		p, ok := prev[r.OfferID]
		if !ok {
			continue
		}
		out = append(out, report.RewardChange{
			Name: r.Offers.Name, PrevRaw: p.RewardRaw, NewRaw: r.RewardRaw,
			PrevPoints: p.RewardPoints, NewPoints: r.RewardPoints,
		})
	}
	return out, nil
}
