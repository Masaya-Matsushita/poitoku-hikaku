// Package crawl は 1 サイト分のクロールを組み立てる：robots.txt の検証 → カテゴリの発見 →
// 一覧のページ送り → 正規化・重複排除 → 保存 → crawl_logs への記録。
// HTTP は fetch、抽出は extract、保存は Store 実装（supabase / dry-run）に任せる。
package crawl

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/extract"
	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/fetch"
	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/policy"
	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/robots"
	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/site"
)

// Fetcher は HTTP 取得の抽象。fetch.Client が満たす。
type Fetcher interface {
	Get(ctx context.Context, url string) ([]byte, error)
	Requests() int
}

// Offer は保存する 1 案件。保存するのは案件名・還元額・URL・カテゴリのみ（AGENTS.md）。
type Offer struct {
	ExternalID    string
	Name          string
	URL           string
	Category      string
	RewardRaw     string
	RewardPoints  *int64
	RewardPercent *float64
}

// ErrorEntry は crawl_logs.errors に入れる要約。
type ErrorEntry struct {
	URL     string `json:"url,omitempty"`
	Message string `json:"message"`
}

// Summary は crawl_logs の 1 行に対応する実行結果。
type Summary struct {
	SiteID       string       `json:"site_id"`
	CrawledOn    string       `json:"crawled_on"`
	StartedAt    time.Time    `json:"started_at"`
	FinishedAt   time.Time    `json:"finished_at"`
	Status       string       `json:"status"`
	RequestCount int          `json:"request_count"`
	OfferCount   int          `json:"offer_count"`
	ParsedCount  int          `json:"parsed_count"`
	ErrorCount   int          `json:"error_count"`
	AbortReason  string       `json:"abort_reason,omitempty"`
	Errors       []ErrorEntry `json:"errors"`
	// 以下は crawl_logs には入れないが報告に使う
	CategoryCount int `json:"category_count"`
	PageCount     int `json:"page_count"`
}

// Store は保存先の抽象。supabase.Client と DryRunStore が満たす。
type Store interface {
	// StartCrawlLog は status=running の行を作り、その id を返す。
	StartCrawlLog(ctx context.Context, siteID, crawledOn string, startedAt time.Time, version string) (int64, error)
	// SaveOffers は offers を upsert し、offer_snapshots を crawledOn の日付で書く。
	SaveOffers(ctx context.Context, siteID, crawledOn string, offers []Offer) error
	// FinishCrawlLog は StartCrawlLog で作った行を最終結果で更新する。
	FinishCrawlLog(ctx context.Context, id int64, s Summary) error
}

// Options は実行時の設定。
type Options struct {
	// Version はクローラーの識別（git SHA）。crawl_logs.crawler_version。
	Version string
	// Now は日付の基準。省略時は time.Now。
	Now func() time.Time
	// Logf は進捗ログ。省略時は捨てる。
	Logf func(format string, args ...any)
	// MaxErrorEntries は crawl_logs.errors に残す件数の上限。省略時 50。
	MaxErrorEntries int
}

// JST は crawled_on の基準タイムゾーン。GitHub Actions は UTC なので明示する。
var JST = time.FixedZone("JST", 9*60*60)

// Result は Run の戻り値。Offers は保存したものと同じ。
type Result struct {
	Summary Summary
	Offers  []Offer
}

type category struct {
	label  string
	params map[string]string
}

// abort はサーキットブレーカー等でクロールを途中で打ち切ることを表す内部エラー。
type abort struct {
	reason string
	err    error
}

func (a *abort) Error() string { return fmt.Sprintf("%s: %v", a.reason, a.err) }

// Run は 1 サイトをクロールして保存する。戻り値の error は「crawl_logs にすら書けない」類の
// 致命的な失敗のみ。取得や保存の失敗は Summary.Status / Errors に反映して nil を返す。
func Run(ctx context.Context, def *site.Definition, f Fetcher, st Store, opts Options) (*Result, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	maxErrors := opts.MaxErrorEntries
	if maxErrors <= 0 {
		maxErrors = 50
	}

	startedAt := now()
	sum := Summary{
		SiteID:    def.ID,
		CrawledOn: startedAt.In(JST).Format("2006-01-02"),
		StartedAt: startedAt,
		Errors:    []ErrorEntry{},
	}
	logID, err := st.StartCrawlLog(ctx, def.ID, sum.CrawledOn, startedAt, opts.Version)
	if err != nil {
		return nil, fmt.Errorf("crawl: crawl_logs を開始できない: %w", err)
	}

	addError := func(u, msg string) {
		sum.ErrorCount++
		if len(sum.Errors) < maxErrors {
			sum.Errors = append(sum.Errors, ErrorEntry{URL: u, Message: msg})
		}
		logf("error: %s %s", u, msg)
	}

	var offers []Offer
	crawlErr := func() error {
		rules, err := fetchRobots(ctx, def, f)
		if err != nil {
			return err
		}
		if err := checkRobots(def, rules); err != nil {
			return err
		}

		cats, err := discover(ctx, def, f, rules)
		if err != nil {
			return err
		}
		sum.CategoryCount = len(cats)
		logf("カテゴリ %d 件を発見", len(cats))

		seen := map[string]bool{}
		for _, c := range cats {
			for page := 1; page <= def.Listing.MaxPages; page++ {
				u, err := def.RenderURL(c.params, page)
				if err != nil {
					addError("", err.Error())
					break
				}
				if !allowed(rules, u) {
					return &abort{reason: "robots_disallow", err: fmt.Errorf("%s は robots.txt で禁止", u)}
				}
				body, err := f.Get(ctx, u)
				if err != nil {
					var stop *fetch.StopError
					if errors.As(err, &stop) {
						return &abort{reason: stop.Reason, err: err}
					}
					if ctx.Err() != nil {
						return &abort{reason: "canceled", err: ctx.Err()}
					}
					addError(u, err.Error())
					break // このカテゴリの残りページは諦めて次へ
				}
				sum.PageCount++
				doc, err := extract.Parse(body)
				if err != nil {
					addError(u, err.Error())
					break
				}
				items, skipped, err := extract.Items(doc, def.Listing)
				if err != nil {
					return fmt.Errorf("crawl: %w", err)
				}
				if skipped > 0 {
					addError(u, fmt.Sprintf("案件名か URL が取れない要素が %d 件", skipped))
				}
				if len(items) == 0 {
					break
				}
				for _, it := range items {
					o, err := toOffer(def, it, c.label)
					if err != nil {
						addError(u, err.Error())
						continue
					}
					if seen[o.URL] {
						continue
					}
					seen[o.URL] = true
					offers = append(offers, o)
				}
				last, err := extract.LastPage(doc, def.Listing)
				if err != nil {
					return fmt.Errorf("crawl: %w", err)
				}
				logf("%s p%d/%d: %d 件（累計 %d）", c.label, page, last, len(items), len(offers))
				if page >= last {
					break
				}
			}
		}
		return nil
	}()

	var ab *abort
	switch {
	case crawlErr == nil:
	case errors.As(crawlErr, &ab):
		sum.AbortReason = ab.reason
		addError("", ab.Error())
	default:
		addError("", crawlErr.Error())
	}

	sum.OfferCount = len(offers)
	for _, o := range offers {
		if o.RewardPoints != nil || o.RewardPercent != nil {
			sum.ParsedCount++
		}
	}
	sum.RequestCount = f.Requests()

	saveErr := error(nil)
	if len(offers) > 0 {
		saveErr = st.SaveOffers(ctx, def.ID, sum.CrawledOn, offers)
		if saveErr != nil {
			addError("", "保存に失敗: "+saveErr.Error())
		}
	}

	sum.FinishedAt = now()
	sum.Status = status(sum, saveErr != nil)
	if err := st.FinishCrawlLog(ctx, logID, sum); err != nil {
		return &Result{Summary: sum, Offers: offers}, fmt.Errorf("crawl: crawl_logs を更新できない: %w", err)
	}
	return &Result{Summary: sum, Offers: offers}, nil
}

// status は crawl_logs.status を決める。
// aborted: サーキットブレーカー等で打ち切り / failed: データ取得なし or 保存失敗 /
// partial: 一部エラー / success: 全件正常
func status(s Summary, saveFailed bool) string {
	switch {
	case s.AbortReason != "":
		return "aborted"
	case s.OfferCount == 0 || saveFailed:
		return "failed"
	case s.ErrorCount > 0:
		return "partial"
	default:
		return "success"
	}
}

// fetchRobots は base_url/robots.txt を取得する。404 は「制限なし」、それ以外の失敗は打ち切り。
func fetchRobots(ctx context.Context, def *site.Definition, f Fetcher) (*robots.Rules, error) {
	u, err := def.Resolve("/robots.txt")
	if err != nil {
		return nil, err
	}
	body, err := f.Get(ctx, u)
	if err != nil {
		var httpErr *fetch.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == 404 {
			return robots.Parse(nil), nil
		}
		var stop *fetch.StopError
		if errors.As(err, &stop) {
			return nil, &abort{reason: stop.Reason, err: err}
		}
		return nil, &abort{reason: "robots_unavailable", err: err}
	}
	return robots.Parse(body), nil
}

// checkRobots は起動時に、これから取得する URL（メニューと一覧テンプレート）が許可されているかを見る。
func checkRobots(def *site.Definition, rules *robots.Rules) error {
	menu, err := def.Resolve(def.Discovery.URL)
	if err != nil {
		return err
	}
	if !allowed(rules, menu) {
		return &abort{reason: "robots_disallow", err: fmt.Errorf("%s は robots.txt で禁止", menu)}
	}
	// テンプレートのプレースホルダを仮の値で埋めてパスだけ検証する
	sample := map[string]string{}
	for _, name := range placeholders(def.Listing.URLTemplate) {
		sample[name] = "1"
	}
	u, err := def.RenderURL(sample, 1)
	if err != nil {
		return err
	}
	if !allowed(rules, u) {
		return &abort{reason: "robots_disallow", err: fmt.Errorf("%s は robots.txt で禁止", u)}
	}
	return nil
}

func placeholders(tmpl string) []string {
	var names []string
	for {
		i := strings.IndexByte(tmpl, '{')
		if i < 0 {
			return names
		}
		j := strings.IndexByte(tmpl[i:], '}')
		if j < 0 {
			return names
		}
		names = append(names, tmpl[i+1:i+j])
		tmpl = tmpl[i+j+1:]
	}
}

func allowed(rules *robots.Rules, rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := u.EscapedPath()
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return rules.Allowed(policy.UserAgent, path)
}

// discover はメニューからカテゴリ（一覧ページのパラメータとラベル）を集める。
func discover(ctx context.Context, def *site.Definition, f Fetcher, rules *robots.Rules) ([]category, error) {
	u, err := def.Resolve(def.Discovery.URL)
	if err != nil {
		return nil, err
	}
	body, err := f.Get(ctx, u)
	if err != nil {
		var stop *fetch.StopError
		if errors.As(err, &stop) {
			return nil, &abort{reason: stop.Reason, err: err}
		}
		return nil, &abort{reason: "discovery_failed", err: err}
	}
	doc, err := extract.Parse(body)
	if err != nil {
		return nil, &abort{reason: "discovery_failed", err: err}
	}
	links, err := extract.Links(doc, def.Discovery)
	if err != nil {
		return nil, err
	}
	var cats []category
	seen := map[string]bool{}
	for _, l := range links {
		params, err := extract.QueryParams(l.Href)
		if err != nil {
			continue
		}
		first, err := def.RenderURL(params, 1)
		if err != nil {
			continue // テンプレートを埋められないリンクは一覧ではない
		}
		if seen[first] {
			continue
		}
		seen[first] = true
		cats = append(cats, category{label: l.Label, params: params})
	}
	if len(cats) == 0 {
		return nil, &abort{reason: "discovery_empty", err: fmt.Errorf("%s からカテゴリを 1 件も見つけられない", u)}
	}
	return cats, nil
}

func toOffer(def *site.Definition, it extract.Item, categoryLabel string) (Offer, error) {
	canonical, err := extract.Canonical(it.Href, def.Base(), def.URL)
	if err != nil {
		return Offer{}, err
	}
	r := extract.ParseReward(it.RewardRaw)
	return Offer{
		ExternalID:    extract.ExternalID(canonical, def.ExternalIDPattern()),
		Name:          it.Name,
		URL:           canonical,
		Category:      categoryLabel,
		RewardRaw:     it.RewardRaw,
		RewardPoints:  r.Points,
		RewardPercent: r.Percent,
	}, nil
}

// SortOffers は URL 順に並べる（出力の安定化用）。
func SortOffers(offers []Offer) {
	sort.Slice(offers, func(i, j int) bool { return offers[i].URL < offers[j].URL })
}
