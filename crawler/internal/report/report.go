// Package report は crawl_logs から日次の KPI レポート（reports/YYYY-MM-DD.md）を組み立てる。
// KPI の定義は docs/02-kpi.md。夜間 Routine とオーナーが読む前提で、判定と根拠を先頭に出す。
package report

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// JST はレポートの日付とタイムスタンプの基準。
var JST = time.FixedZone("JST", 9*60*60)

// Site は sites テーブルの行。
type Site struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// LogError は crawl_logs.errors の 1 要素。
type LogError struct {
	URL     string `json:"url,omitempty"`
	Message string `json:"message"`
}

// CrawlLog は crawl_logs の 1 行。
type CrawlLog struct {
	ID             int64
	SiteID         string
	CrawledOn      string
	StartedAt      time.Time
	FinishedAt     *time.Time
	Status         string
	RequestCount   int
	OfferCount     int
	ParsedCount    int
	ErrorCount     int
	AbortReason    string
	Errors         []LogError
	CrawlerVersion string
}

// Source はレポートの元データ。supabase.Client が満たす。
type Source interface {
	Sites(ctx context.Context) ([]Site, error)
	// CrawlLogs は crawled_on が from〜to（両端含む、YYYY-MM-DD）の行を日付・id 順に返す。
	CrawlLogs(ctx context.Context, from, to string) ([]CrawlLog, error)
	// EmptyRewardCount は crawledOn に掲載されていた案件のうち、現在有効な還元額の reward_raw が空の件数（真の抽出失敗）。
	EmptyRewardCount(ctx context.Context, siteID, crawledOn string) (int, error)
	// ChangedCount は crawledOn に始まった区間の数（還元額が変わった案件 + 新規案件。ADR-0005 の変化率の実測）。
	ChangedCount(ctx context.Context, siteID, crawledOn string) (int, error)
}

// KPI の閾値（docs/02-kpi.md、ADR-0003）。
const (
	successRateTarget = 0.95 // クロール成功率
	parsedRateTarget  = 0.99 // 抽出精度
	dropAlertRatio    = 0.5  // 前日比でこの割合未満なら異常（ADR-0003 の検知定義）
	historyDays       = 14
)

// SiteDay は 1 サイトの当日の結果。Log は当日の最後の実行（同日の再実行は最後を採用）。
type SiteDay struct {
	Site         Site
	Log          *CrawlLog
	PrevOffers   int  // 前日（最後の実行）の案件数。無ければ -1
	EmptyRewards int  // reward_raw が空の行数（真の抽出失敗）
	EmptyKnown   bool // EmptyRewards を取得できたか
	Changed      int  // 当日に始まった区間の数（還元額の変化 + 新規）
	ChangedKnown bool // Changed を取得できたか
}

// ParsedRate は数値化率。案件 0 件なら 0。
func (d SiteDay) ParsedRate() float64 {
	if d.Log == nil || d.Log.OfferCount == 0 {
		return 0
	}
	return float64(d.Log.ParsedCount) / float64(d.Log.OfferCount)
}

// Window はクロール成功率の集計。サイト×日で数え、その日の最後の実行が success か partial なら成功。
// 分母はサイトごとに「最初の実行日以降」の日だけ（稼働前の日を失敗に数えない）。
type Window struct {
	Days    int
	Total   int
	Success int
}

// Rate は成功率。分母 0 なら -1。
func (w Window) Rate() float64 {
	if w.Total == 0 {
		return -1
	}
	return float64(w.Success) / float64(w.Total)
}

// Report は 1 日分のレポート。
type Report struct {
	Date        string
	GeneratedAt time.Time
	Sites       []SiteDay
	Week        Window
	Month       Window
	// History は直近 historyDays 日分の、日付 → サイト ID → 案件数（最後の実行）。
	History  []HistoryRow
	Logs     []CrawlLog // 当日の全実行
	Warnings []string
}

// HistoryRow は履歴表の 1 行。
type HistoryRow struct {
	Date   string
	Offers map[string]int // サイト ID → 案件数。無い日は含まれない
}

// Build は date（YYYY-MM-DD、JST）のレポートを組み立てる。windowDays は成功率の長い方の窓（通常 30）。
func Build(ctx context.Context, src Source, date string, now time.Time, windowDays int) (*Report, error) {
	day, err := time.ParseInLocation("2006-01-02", date, JST)
	if err != nil {
		return nil, fmt.Errorf("report: 日付が不正: %w", err)
	}
	if windowDays < historyDays {
		windowDays = historyDays
	}
	sites, err := src.Sites(ctx)
	if err != nil {
		return nil, fmt.Errorf("report: sites を読めない: %w", err)
	}
	from := day.AddDate(0, 0, -(windowDays - 1)).Format("2006-01-02")
	logs, err := src.CrawlLogs(ctx, from, date)
	if err != nil {
		return nil, fmt.Errorf("report: crawl_logs を読めない: %w", err)
	}

	// 日付 × サイト → その日の最後の実行
	last := map[string]map[string]*CrawlLog{}
	firstDay := map[string]string{}
	for i := range logs {
		l := &logs[i]
		if last[l.CrawledOn] == nil {
			last[l.CrawledOn] = map[string]*CrawlLog{}
		}
		last[l.CrawledOn][l.SiteID] = l // 日付・id 順なので後勝ち＝最後の実行
		if f, ok := firstDay[l.SiteID]; !ok || l.CrawledOn < f {
			firstDay[l.SiteID] = l.CrawledOn
		}
	}

	r := &Report{Date: date, GeneratedAt: now.In(JST)}
	prevDate := day.AddDate(0, 0, -1).Format("2006-01-02")
	for _, s := range sites {
		if !s.IsActive {
			continue
		}
		d := SiteDay{Site: s, Log: last[date][s.ID], PrevOffers: -1}
		if p := last[prevDate][s.ID]; p != nil {
			d.PrevOffers = p.OfferCount
		}
		if d.Log != nil {
			n, err := src.EmptyRewardCount(ctx, s.ID, date)
			if err != nil {
				r.Warnings = append(r.Warnings, fmt.Sprintf("%s: 真の抽出失敗の件数を取得できなかった（%v）", s.Name, err))
			} else {
				d.EmptyRewards, d.EmptyKnown = n, true
			}
			ch, err := src.ChangedCount(ctx, s.ID, date)
			if err != nil {
				r.Warnings = append(r.Warnings, fmt.Sprintf("%s: 還元額の変化件数を取得できなかった（%v）", s.Name, err))
			} else {
				d.Changed, d.ChangedKnown = ch, true
			}
		}
		r.Sites = append(r.Sites, d)
	}
	for _, l := range logs {
		if l.CrawledOn == date {
			r.Logs = append(r.Logs, l)
		}
	}

	r.Week = window(day, 7, r.Sites, last, firstDay)
	r.Month = window(day, windowDays, r.Sites, last, firstDay)
	for i := historyDays - 1; i >= 0; i-- {
		dt := day.AddDate(0, 0, -i).Format("2006-01-02")
		row := HistoryRow{Date: dt, Offers: map[string]int{}}
		for sid, l := range last[dt] {
			row.Offers[sid] = l.OfferCount
		}
		r.History = append(r.History, row)
	}
	r.Warnings = append(r.Warnings, warnings(r)...)
	return r, nil
}

func window(day time.Time, days int, sites []SiteDay, last map[string]map[string]*CrawlLog, firstDay map[string]string) Window {
	w := Window{Days: days}
	for i := 0; i < days; i++ {
		dt := day.AddDate(0, 0, -i).Format("2006-01-02")
		for _, s := range sites {
			f, started := firstDay[s.Site.ID]
			if !started || dt < f {
				continue
			}
			w.Total++
			if l := last[dt][s.Site.ID]; l != nil && (l.Status == "success" || l.Status == "partial") {
				w.Success++
			}
		}
	}
	return w
}

// warnings は「要確認」に載せる項目を返す。順序は固定（サイト順 → 全体）。
func warnings(r *Report) []string {
	var out []string
	for _, d := range r.Sites {
		name := d.Site.Name
		switch {
		case d.Log == nil:
			out = append(out, fmt.Sprintf("%s: 当日の実行記録が無い（クロールが起動していない）", name))
			continue
		case d.Log.Status == "running":
			out = append(out, fmt.Sprintf("%s: status が running のまま（途中で落ちた可能性）", name))
			continue
		case d.Log.Status == "aborted":
			out = append(out, fmt.Sprintf("%s: 打ち切り（%s）。取れたのは %d 件", name, d.Log.AbortReason, d.Log.OfferCount))
		case d.Log.Status == "failed":
			out = append(out, fmt.Sprintf("%s: 失敗（案件 %d 件、エラー %d 件）", name, d.Log.OfferCount, d.Log.ErrorCount))
		case d.Log.Status == "partial":
			out = append(out, fmt.Sprintf("%s: 一部エラー（%d 件）。案件は %d 件取れている", name, d.Log.ErrorCount, d.Log.OfferCount))
		}
		if d.Log.OfferCount > 0 && d.ParsedRate() < parsedRateTarget {
			out = append(out, fmt.Sprintf("%s: 数値化率 %s が目標 99%% を下回る（%d / %d）", name, pct(d.ParsedRate()), d.Log.ParsedCount, d.Log.OfferCount))
		}
		if d.EmptyKnown && d.EmptyRewards > 0 {
			out = append(out, fmt.Sprintf("%s: 真の抽出失敗（reward_raw が空）が %d 件。セレクタか数値化ルールの修復対象", name, d.EmptyRewards))
		}
		if d.PrevOffers > 0 && float64(d.Log.OfferCount) < float64(d.PrevOffers)*dropAlertRatio {
			out = append(out, fmt.Sprintf("%s: 案件数が前日比 50%% 未満（%d → %d）。構造変化の疑い（ADR-0003）", name, d.PrevOffers, d.Log.OfferCount))
		} else if d.PrevOffers > 0 && d.Log.OfferCount < d.PrevOffers {
			out = append(out, fmt.Sprintf("%s: 案件数が前日より減少（%d → %d）。単調増加の KPI に注意", name, d.PrevOffers, d.Log.OfferCount))
		}
	}
	if r.Week.Total > 0 && r.Week.Rate() < successRateTarget {
		out = append(out, fmt.Sprintf("直近 7 日のクロール成功率 %s が目標 95%% を下回る（%d / %d サイト×日）", pct(r.Week.Rate()), r.Week.Success, r.Week.Total))
	}
	return out
}

// Render は Markdown を返す。出力は決定的（同じ入力なら同じ文字列）。
func Render(r *Report) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# 日次レポート %s\n\n", r.Date)
	w("生成: %s（GitHub Actions `crawl.yml` の report ジョブ）。KPI の定義は `docs/02-kpi.md`、閾値の根拠は同ファイルと ADR-0003。\n\n", r.GeneratedAt.Format("2006-01-02 15:04 JST"))

	w("## 判定\n\n")
	if len(r.Warnings) == 0 {
		w("**正常**。要確認の項目はありません。\n\n")
	} else {
		w("**要確認 %d 件**\n\n", len(r.Warnings))
		for _, s := range r.Warnings {
			w("- %s\n", s)
		}
		w("\n")
	}

	w("## サイト別（%s）\n\n", r.Date)
	w("| サイト | status | 案件数 | 前日比 | 数値化率 | 真の抽出失敗 | 還元額の変化 | リクエスト | 所要 | 打ち切り理由 | 版 |\n")
	w("|---|---|---:|---:|---:|---:|---:|---:|---:|---|---|\n")
	for _, d := range r.Sites {
		if d.Log == nil {
			w("| %s | 未実行 | - | - | - | - | - | - | - | - | - |\n", d.Site.Name)
			continue
		}
		l := d.Log
		diff := "-"
		if d.PrevOffers >= 0 {
			diff = fmt.Sprintf("%+d", l.OfferCount-d.PrevOffers)
		}
		empty := "-"
		if d.EmptyKnown {
			empty = fmt.Sprintf("%d", d.EmptyRewards)
		}
		changed := "-"
		if d.ChangedKnown {
			changed = num(d.Changed)
		}
		w("| %s | %s | %s | %s | %s | %s | %s | %d | %s | %s | %s |\n",
			d.Site.Name, l.Status, num(l.OfferCount), diff, pct(d.ParsedRate()), empty, changed, l.RequestCount,
			duration(l), dash(l.AbortReason), dash(l.CrawlerVersion))
	}
	w("\n「還元額の変化」は当日に始まった区間の数（還元額が変わった案件 + 新規案件）。ADR-0005 の容量試算の前提（変化率）を実測する。\n\n")

	w("## KPI\n\n")
	w("| KPI | 目標 | 直近 7 日 | 直近 %d 日 |\n", r.Month.Days)
	w("|---|---|---|---|\n")
	w("| クロール成功率（サイト×日） | 95%% 以上 | %s | %s |\n", rate(r.Week), rate(r.Month))
	w("| 抽出精度（当日、数値化率） | 99%% 以上 | %s | |\n", parsedSummary(r))
	w("| 対象案件数 | 単調増加 | %s | |\n", offersSummary(r))
	w("| 索引ページ数 | 単調増加 | 未計測（Search Console 未連携） | |\n")
	w("| コスト | 0 円 | 支払い手段なし（構造的に 0 円） | |\n")
	w("\n")

	w("## 履歴（直近 %d 日の案件数）\n\n", historyDays)
	w("| 日付 |")
	for _, d := range r.Sites {
		w(" %s |", d.Site.Name)
	}
	w("\n|---|")
	for range r.Sites {
		w("---:|")
	}
	w("\n")
	for _, h := range r.History {
		w("| %s |", h.Date)
		for _, d := range r.Sites {
			if n, ok := h.Offers[d.Site.ID]; ok {
				w(" %s |", num(n))
			} else {
				w(" - |")
			}
		}
		w("\n")
	}
	w("\n")

	w("## 実行ログ（%s）\n\n", r.Date)
	if len(r.Logs) == 0 {
		w("crawl_logs に当日の行がありません。\n")
	} else {
		w("| id | サイト | status | 開始（JST） | 所要 | リクエスト | 案件 | 数値化 | エラー | 打ち切り理由 |\n")
		w("|---:|---|---|---|---:|---:|---:|---:|---:|---|\n")
		for _, l := range r.Logs {
			w("| %d | %s | %s | %s | %s | %d | %d | %d | %d | %s |\n",
				l.ID, l.SiteID, l.Status, l.StartedAt.In(JST).Format("15:04:05"), duration(&l),
				l.RequestCount, l.OfferCount, l.ParsedCount, l.ErrorCount, dash(l.AbortReason))
		}
		for _, l := range r.Logs {
			if len(l.Errors) == 0 {
				continue
			}
			w("\nid %d のエラー（先頭 %d 件）:\n\n", l.ID, min(5, len(l.Errors)))
			for _, e := range l.Errors[:min(5, len(l.Errors))] {
				if e.URL != "" {
					w("- `%s` %s\n", e.URL, e.Message)
				} else {
					w("- %s\n", e.Message)
				}
			}
		}
	}
	return b.String()
}

func parsedSummary(r *Report) string {
	var parts []string
	for _, d := range r.Sites {
		if d.Log == nil {
			parts = append(parts, d.Site.Name+" -")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", d.Site.Name, pct(d.ParsedRate())))
	}
	return strings.Join(parts, " / ")
}

func offersSummary(r *Report) string {
	var parts []string
	for _, d := range r.Sites {
		if d.Log == nil {
			parts = append(parts, d.Site.Name+" -")
			continue
		}
		if d.PrevOffers >= 0 {
			parts = append(parts, fmt.Sprintf("%s %s → %s", d.Site.Name, num(d.PrevOffers), num(d.Log.OfferCount)))
		} else {
			parts = append(parts, fmt.Sprintf("%s %s", d.Site.Name, num(d.Log.OfferCount)))
		}
	}
	return strings.Join(parts, " / ")
}

func rate(w Window) string {
	if w.Total == 0 {
		return "データなし"
	}
	return fmt.Sprintf("%s（%d / %d）", pct(w.Rate()), w.Success, w.Total)
}

func pct(v float64) string {
	return fmt.Sprintf("%.1f%%", v*100)
}

func num(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	return s + "," + strings.Join(parts, ",")
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func duration(l *CrawlLog) string {
	if l.FinishedAt == nil {
		return "-"
	}
	d := l.FinishedAt.Sub(l.StartedAt).Round(time.Second)
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}
