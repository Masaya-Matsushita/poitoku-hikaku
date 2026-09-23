// Package supabase は Supabase の REST API（PostgREST）で offers / offer_snapshots / crawl_logs に
// 書き込む。認証は secret key（RLS を通らない）。読み取り専用の publishable key では書けない。
package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/crawl"
)

const (
	// pageSize は既存 offers を読む時の 1 リクエストあたりの行数（PostgREST の既定上限が 1000）。
	pageSize = 1000
	// chunkSize は upsert / insert の 1 リクエストあたりの行数。
	chunkSize = 500
	timeout   = 60 * time.Second
)

// Client は 1 プロジェクト向けの REST クライアント。
type Client struct {
	baseURL    string
	key        string
	httpClient *http.Client
}

// New は SUPABASE_URL と SUPABASE_SECRET_KEY からクライアントを作る。
func New(baseURL, secretKey string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || secretKey == "" {
		return nil, fmt.Errorf("supabase: SUPABASE_URL と SUPABASE_SECRET_KEY が必要")
	}
	if u, err := url.Parse(baseURL); err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("supabase: SUPABASE_URL が不正: %q", baseURL)
	}
	return &Client{baseURL: baseURL, key: secretKey, httpClient: &http.Client{Timeout: timeout}}, nil
}

// APIError は 2xx 以外の応答。本文（先頭のみ）を含める。
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("supabase: %s %s → HTTP %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, headers map[string]string, in, out any) error {
	_, raw, err := c.doRaw(ctx, method, path, query, headers, in)
	if err != nil {
		return err
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("supabase: %s %s の応答を解釈できない: %w", method, path, err)
		}
	}
	return nil
}

// doRaw はリクエストを送り、2xx なら応答ヘッダと本文を返す。2xx 以外は APIError。
func (c *Client) doRaw(ctx context.Context, method, path string, query url.Values, headers map[string]string, in any) (http.Header, []byte, error) {
	u := c.baseURL + "/rest/v1/" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, nil, fmt.Errorf("supabase: %w", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, nil, fmt.Errorf("supabase: %w", err)
	}
	req.Header.Set("apikey", c.key)
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("supabase: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("supabase: %s %s: %w", method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := string(raw)
		if len(msg) > 500 {
			msg = msg[:500] + "…"
		}
		return nil, nil, &APIError{Method: method, Path: path, StatusCode: resp.StatusCode, Body: msg}
	}
	return resp.Header, raw, nil
}

// count は条件に合う行数を返す（PostgREST の Prefer: count=exact と Content-Range を使い、本文は 1 行だけ取る）。
func (c *Client) count(ctx context.Context, path string, query url.Values) (int, error) {
	h, _, err := c.doRaw(ctx, http.MethodGet, path, query,
		map[string]string{"Prefer": "count=exact", "Range-Unit": "items", "Range": "0-0"}, nil)
	if err != nil {
		return 0, err
	}
	cr := h.Get("Content-Range") // "0-0/123" または "*/0"
	i := strings.LastIndexByte(cr, '/')
	if i < 0 {
		return 0, fmt.Errorf("supabase: %s の Content-Range が不正: %q", path, cr)
	}
	n, err := strconv.Atoi(cr[i+1:])
	if err != nil {
		return 0, fmt.Errorf("supabase: %s の Content-Range が不正: %q", path, cr)
	}
	return n, nil
}

// StartCrawlLog は crawl_logs に status=running の行を作る。
func (c *Client) StartCrawlLog(ctx context.Context, siteID, crawledOn string, startedAt time.Time, version string) (int64, error) {
	payload := map[string]any{
		"site_id":    siteID,
		"crawled_on": crawledOn,
		"started_at": startedAt.UTC().Format(time.RFC3339),
		"status":     "running",
	}
	if version != "" {
		payload["crawler_version"] = version
	}
	var rows []struct {
		ID int64 `json:"id"`
	}
	err := c.do(ctx, http.MethodPost, "crawl_logs", url.Values{"select": {"id"}},
		map[string]string{"Prefer": "return=representation"}, payload, &rows)
	if err != nil {
		return 0, err
	}
	if len(rows) != 1 {
		return 0, fmt.Errorf("supabase: crawl_logs の作成応答が %d 行", len(rows))
	}
	return rows[0].ID, nil
}

// FinishCrawlLog は実行結果で crawl_logs の行を更新する。
func (c *Client) FinishCrawlLog(ctx context.Context, id int64, s crawl.Summary) error {
	payload := map[string]any{
		"finished_at":   s.FinishedAt.UTC().Format(time.RFC3339),
		"status":        s.Status,
		"request_count": s.RequestCount,
		"offer_count":   s.OfferCount,
		"parsed_count":  s.ParsedCount,
		"error_count":   s.ErrorCount,
		"errors":        s.Errors,
	}
	if s.AbortReason != "" {
		payload["abort_reason"] = s.AbortReason
	}
	return c.do(ctx, http.MethodPatch, "crawl_logs", url.Values{"id": {fmt.Sprintf("eq.%d", id)}},
		map[string]string{"Prefer": "return=minimal"}, payload, nil)
}

type offerRow struct {
	SiteID      string  `json:"site_id"`
	ExternalID  *string `json:"external_id"`
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	Category    *string `json:"category"`
	FirstSeenOn string  `json:"first_seen_on"`
	LastSeenOn  string  `json:"last_seen_on"`
}

type snapshotRow struct {
	OfferUUID     string   `json:"offer_id"`
	CrawledOn     string   `json:"crawled_on"`
	RewardRaw     string   `json:"reward_raw"`
	RewardPoints  *int64   `json:"reward_points"`
	RewardPercent *float64 `json:"reward_percent"`
}

// SaveOffers は offers を upsert（first_seen_on は既存値を保持、last_seen_on は crawledOn）し、
// offer_snapshots に crawledOn の行を書く（同日の再実行は上書き）。
func (c *Client) SaveOffers(ctx context.Context, siteID, crawledOn string, offers []crawl.Offer) error {
	existing, err := c.existingOffers(ctx, siteID)
	if err != nil {
		return err
	}

	// URL の重複は呼び出し側で除いてある前提だが、upsert が同一行を 2 度触ると失敗するので念のため
	seen := map[string]bool{}
	rows := make([]offerRow, 0, len(offers))
	for _, o := range offers {
		if seen[o.URL] {
			continue
		}
		seen[o.URL] = true
		first := crawledOn
		if prev, ok := existing[o.URL]; ok && prev != "" {
			first = prev
		}
		row := offerRow{SiteID: siteID, Name: o.Name, URL: o.URL, FirstSeenOn: first, LastSeenOn: crawledOn}
		if o.ExternalID != "" {
			row.ExternalID = strPtr(o.ExternalID)
		}
		if o.Category != "" {
			row.Category = strPtr(o.Category)
		}
		rows = append(rows, row)
	}

	ids := map[string]string{}
	for start := 0; start < len(rows); start += chunkSize {
		end := min(start+chunkSize, len(rows))
		var out []struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		}
		err := c.do(ctx, http.MethodPost, "offers",
			url.Values{"on_conflict": {"site_id,url"}, "select": {"id,url"}},
			map[string]string{"Prefer": "resolution=merge-duplicates,return=representation"},
			rows[start:end], &out)
		if err != nil {
			return err
		}
		for _, r := range out {
			ids[r.URL] = r.ID
		}
	}

	snaps := make([]snapshotRow, 0, len(offers))
	for _, o := range offers {
		id, ok := ids[o.URL]
		if !ok {
			return fmt.Errorf("supabase: upsert 後に %s の id が返らない", o.URL)
		}
		snaps = append(snaps, snapshotRow{
			OfferUUID:     id,
			CrawledOn:     crawledOn,
			RewardRaw:     o.RewardRaw,
			RewardPoints:  o.RewardPoints,
			RewardPercent: o.RewardPercent,
		})
	}
	for start := 0; start < len(snaps); start += chunkSize {
		end := min(start+chunkSize, len(snaps))
		err := c.do(ctx, http.MethodPost, "offer_snapshots",
			url.Values{"on_conflict": {"offer_id,crawled_on"}},
			map[string]string{"Prefer": "resolution=merge-duplicates,return=minimal"},
			snaps[start:end], nil)
		if err != nil {
			return err
		}
	}
	return nil
}

// existingOffers はサイトの既存 offers を url → first_seen_on で返す。
func (c *Client) existingOffers(ctx context.Context, siteID string) (map[string]string, error) {
	out := map[string]string{}
	for from := 0; ; from += pageSize {
		var rows []struct {
			URL         string `json:"url"`
			FirstSeenOn string `json:"first_seen_on"`
		}
		err := c.do(ctx, http.MethodGet, "offers",
			url.Values{"site_id": {"eq." + siteID}, "select": {"url,first_seen_on"}, "order": {"id.asc"}},
			map[string]string{"Range-Unit": "items", "Range": fmt.Sprintf("%d-%d", from, from+pageSize-1)},
			nil, &rows)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.URL] = r.FirstSeenOn
		}
		if len(rows) < pageSize {
			return out, nil
		}
	}
}

func strPtr(s string) *string { return &s }
