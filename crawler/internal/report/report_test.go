package report

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

type fakeSource struct {
	sites   []Site
	logs    []CrawlLog
	empty   map[string]int // "site|date" → 件数
	changed map[string]int // "site|date" → 件数
	newOnes map[string]int // "site|date" → first_seen_on = date の件数
	gone    map[string]int // "site|date" → last_seen_on = date の件数
	err     error
	// churnErr は NewOfferCount / GoneOfferCount だけを失敗させる
	churnErr error

	goneDetails    map[string][]GoneOffer // "site|prevDate" → 消えた案件の名前・カテゴリ
	goneDetailsErr error                  // GoneOfferDetails だけを失敗させる
}

func (f *fakeSource) GoneOfferDetails(_ context.Context, siteID, prevDate string) ([]GoneOffer, error) {
	return f.goneDetails[siteID+"|"+prevDate], f.goneDetailsErr
}

func (f *fakeSource) ChangedCount(_ context.Context, siteID, date string) (int, error) {
	return f.changed[siteID+"|"+date], nil
}

func (f *fakeSource) NewOfferCount(_ context.Context, siteID, date string) (int, error) {
	return f.newOnes[siteID+"|"+date], f.churnErr
}

func (f *fakeSource) GoneOfferCount(_ context.Context, siteID, prevDate string) (int, error) {
	return f.gone[siteID+"|"+prevDate], f.churnErr
}

func (f *fakeSource) Sites(context.Context) ([]Site, error) { return f.sites, f.err }

func (f *fakeSource) CrawlLogs(_ context.Context, from, to string) ([]CrawlLog, error) {
	var out []CrawlLog
	for _, l := range f.logs {
		if l.CrawledOn >= from && l.CrawledOn <= to {
			out = append(out, l)
		}
	}
	// Source の契約どおり日付・id 順で返す
	sort.Slice(out, func(i, j int) bool {
		if out[i].CrawledOn != out[j].CrawledOn {
			return out[i].CrawledOn < out[j].CrawledOn
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fakeSource) EmptyRewardCount(_ context.Context, siteID, date string) (int, error) {
	return f.empty[siteID+"|"+date], nil
}

func ts(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, JST)
	if err != nil {
		panic(err)
	}
	return t
}

func tsp(s string) *time.Time { t := ts(s); return &t }

func log(id int64, site, date, status string, offers, parsed, errs int) CrawlLog {
	return CrawlLog{
		ID: id, SiteID: site, CrawledOn: date, Status: status,
		StartedAt: ts(date + " 03:00:10"), FinishedAt: tsp(date + " 03:08:02"),
		RequestCount: 158, OfferCount: offers, ParsedCount: parsed, ErrorCount: errs, CrawlerVersion: "5cad0a6",
	}
}

// 3 日分のシナリオ：
//
//	9/22 moppy 手動 2 回（後の方を採用）、hapitas は未稼働
//	9/23 両サイト success
//	9/24 moppy は partial で案件が減少、hapitas は aborted（429）。真の抽出失敗が moppy に 2 件
func scenario() *fakeSource {
	l24h := log(6, "hapitas", "2026-09-24", "aborted", 900, 900, 1)
	l24h.AbortReason = "http_429"
	l24h.Errors = []LogError{{Message: "http_429: fetch: クロール停止（http_429, HTTP 429, https://hapitas.jp/category/x/）"}}
	l24m := log(5, "moppy", "2026-09-24", "partial", 1700, 1680, 1)
	l24m.Errors = []LogError{{URL: "https://pc.moppy.jp/ajax/category/get_list.php?parent_category=4&child_category=125", Message: "fetch: HTTP 404"}}
	return &fakeSource{
		sites: []Site{{ID: "hapitas", Name: "ハピタス", IsActive: true}, {ID: "moppy", Name: "モッピー", IsActive: true}, {ID: "old", Name: "停止中", IsActive: false}},
		logs: []CrawlLog{
			log(1, "moppy", "2026-09-22", "success", 1791, 1775, 0),
			log(2, "moppy", "2026-09-22", "success", 1791, 1785, 0),
			log(3, "moppy", "2026-09-23", "success", 1791, 1785, 0),
			log(4, "hapitas", "2026-09-23", "success", 2773, 2773, 0),
			l24m, l24h,
		},
		empty:   map[string]int{"moppy|2026-09-24": 2},
		changed: map[string]int{"moppy|2026-09-24": 57, "hapitas|2026-09-24": 900},
		// 9/24 moppy：1791 → 1700 = 新規 3 − 消えた 94。hapitas は打ち切りで 1873 件が未観測
		newOnes: map[string]int{"moppy|2026-09-24": 3},
		gone:    map[string]int{"moppy|2026-09-23": 94, "hapitas|2026-09-23": 1873},
	}
}

func TestBuildComputesKPIsAndWarnings(t *testing.T) {
	r, err := Build(context.Background(), scenario(), "2026-09-24", ts("2026-09-24 03:20:00"), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sites) != 2 {
		t.Fatalf("Sites = %d（is_active=false は除く）", len(r.Sites))
	}
	// サイトは sites の順（hapitas, moppy）
	h, m := r.Sites[0], r.Sites[1]
	if h.Site.ID != "hapitas" || m.Site.ID != "moppy" {
		t.Fatalf("順序 = %s, %s", h.Site.ID, m.Site.ID)
	}
	if m.Log == nil || m.Log.ID != 5 || m.PrevOffers != 1791 || !m.EmptyKnown || m.EmptyRewards != 2 || !m.ChangedKnown || m.Changed != 57 {
		t.Errorf("moppy = %+v", m)
	}
	if h.Log == nil || h.Log.Status != "aborted" || h.PrevOffers != 2773 {
		t.Errorf("hapitas = %+v", h)
	}
	// 新規は当日、消えたは前日の日付で引く
	if !m.ChurnKnown || m.NewOffers != 3 || m.GoneOffers != 94 || !h.ChurnKnown || h.NewOffers != 0 || h.GoneOffers != 1873 {
		t.Errorf("内訳 moppy = %d / %d, hapitas = %d / %d", m.NewOffers, m.GoneOffers, h.NewOffers, h.GoneOffers)
	}
	// 成功率（サイト×日）：moppy は 9/22〜24 の 3 日（success, success, partial → 3 成功）、
	// hapitas は 9/23〜24 の 2 日（success, aborted → 1 成功）→ 4 / 5
	if r.Week.Total != 5 || r.Week.Success != 4 {
		t.Errorf("Week = %+v, want 4 / 5", r.Week)
	}
	if r.Month.Total != 5 || r.Month.Success != 4 {
		t.Errorf("Month = %+v, want 4 / 5", r.Month)
	}
	if len(r.History) != historyDays || r.History[historyDays-1].Offers["moppy"] != 1700 {
		t.Errorf("History = %+v", r.History[historyDays-1])
	}
	// 9/22 は最後の実行（id 2）の値
	var d22 HistoryRow
	for _, h := range r.History {
		if h.Date == "2026-09-22" {
			d22 = h
		}
	}
	if d22.Offers["moppy"] != 1791 {
		t.Errorf("9/22 moppy = %v", d22.Offers)
	}
	if len(r.Logs) != 2 {
		t.Errorf("Logs = %d", len(r.Logs))
	}

	joined := strings.Join(r.Warnings, "\n")
	for _, want := range []string{
		"ハピタス: 打ち切り（http_429）",
		"ハピタス: 案件数が前日比 50% 未満（2773 → 900、新規 0・消えた 1873）",
		"モッピー: 一部エラー（1 件）",
		"モッピー: 数値化率 98.8% が目標 99% を下回る",
		"モッピー: 真の抽出失敗（reward_raw が空）が 2 件",
		"モッピー: 案件数が前日より減少（1791 → 1700、新規 3・消えた 94）",
		"直近 7 日のクロール成功率 80.0% が目標 95% を下回る（4 / 5 サイト×日）",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("警告に %q が無い:\n%s", want, joined)
		}
	}
}

func TestBuildChurn(t *testing.T) {
	// 前日の実行が無いサイト（9/22 の moppy）は内訳を取らない
	r, err := Build(context.Background(), scenario(), "2026-09-22", ts("2026-09-22 03:20:00"), 30)
	if err != nil {
		t.Fatal(err)
	}
	if m := r.Sites[1]; m.ChurnKnown {
		t.Errorf("前日の実行が無いのに内訳を取った: %+v", m)
	}

	// 取得に失敗したら警告に出し、表は「-」、減少の警告は内訳なし
	src := scenario()
	src.churnErr = errors.New("timeout")
	r, err = Build(context.Background(), src, "2026-09-24", ts("2026-09-24 03:20:00"), 30)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(r.Warnings, "\n")
	for _, want := range []string{"モッピー: 新規・消えた案件の件数を取得できなかった", "モッピー: 案件数が前日より減少（1791 → 1700）。"} {
		if !strings.Contains(joined, want) {
			t.Errorf("警告に %q が無い:\n%s", want, joined)
		}
	}
	if !strings.Contains(Render(r), "| モッピー | partial | 1,700 | -91 | - | - |") {
		t.Error("取得失敗の内訳が「-」になっていない")
	}
}

func TestBuildWithMissingDayAndNoData(t *testing.T) {
	src := scenario()
	// 9/25 は実行記録が無い日
	r, err := Build(context.Background(), src, "2026-09-25", ts("2026-09-25 03:20:00"), 30)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(r.Warnings, "\n")
	if !strings.Contains(joined, "ハピタス: 当日の実行記録が無い") || !strings.Contains(joined, "モッピー: 当日の実行記録が無い") {
		t.Errorf("未実行の警告が無い:\n%s", joined)
	}
	// 未実行の日は分母に入る（稼働開始後なので）
	if r.Week.Total != 7 || r.Week.Success != 4 {
		t.Errorf("Week = %+v, want 4 / 7", r.Week)
	}
	if !strings.Contains(Render(r), "| ハピタス | 未実行 |") {
		t.Error("サイト別表に未実行が出ていない")
	}

	// データがまったく無い
	empty := &fakeSource{sites: []Site{{ID: "moppy", Name: "モッピー", IsActive: true}}}
	r, err = Build(context.Background(), empty, "2026-01-01", ts("2026-01-01 03:00:00"), 30)
	if err != nil {
		t.Fatal(err)
	}
	if r.Week.Total != 0 || !strings.Contains(Render(r), "データなし") {
		t.Errorf("稼働前は分母 0 のはず: %+v", r.Week)
	}
}

func TestBuildRejectsBadDateAndSourceError(t *testing.T) {
	if _, err := Build(context.Background(), scenario(), "2026/09/24", time.Now(), 30); err == nil {
		t.Error("不正な日付でエラーにならない")
	}
	if _, err := Build(context.Background(), &fakeSource{err: errors.New("down")}, "2026-09-24", time.Now(), 30); err == nil {
		t.Error("sites の失敗がエラーにならない")
	}
}

// Render の出力はゴールデンファイルと一致させる。意図して変えた時は UPDATE_GOLDEN=1 go test ./internal/report/ で更新する。
func TestRenderGolden(t *testing.T) {
	r, err := Build(context.Background(), scenario(), "2026-09-24", ts("2026-09-24 03:20:00"), 30)
	if err != nil {
		t.Fatal(err)
	}
	got := Render(r)
	path := filepath.Join("testdata", "report_golden.md")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v（UPDATE_GOLDEN=1 で生成する）", err)
	}
	if got != string(want) {
		t.Errorf("ゴールデンと不一致。差分を確認して UPDATE_GOLDEN=1 で更新する。\n--- got ---\n%s", got)
	}
}

func TestBuildGoneOfferBreakdown(t *testing.T) {
	src := scenario()
	src.goneDetails = map[string][]GoneOffer{
		"moppy|2026-09-23": {
			{Name: "案件A", Category: "アプリ"},
			{Name: "案件C", Category: "アプリ"},
			{Name: "案件B", Category: ""},
		},
	}
	r, err := Build(context.Background(), src, "2026-09-24", ts("2026-09-24 03:20:00"), 30)
	if err != nil {
		t.Fatal(err)
	}
	m := r.Sites[1]
	if len(m.GoneCategories) != 2 {
		t.Fatalf("GoneCategories = %+v", m.GoneCategories)
	}
	if m.GoneCategories[0].Category != "アプリ" || m.GoneCategories[0].Count != 2 || strings.Join(m.GoneCategories[0].Examples, ",") != "案件A,案件C" {
		t.Errorf("アプリ = %+v", m.GoneCategories[0])
	}
	if m.GoneCategories[1].Category != "未分類" || m.GoneCategories[1].Count != 1 {
		t.Errorf("未分類 = %+v", m.GoneCategories[1])
	}
	out := Render(r)
	if !strings.Contains(out, "### 消えた案件の内訳（モッピー）") || !strings.Contains(out, "| アプリ | 2 | 案件A、案件C |") {
		t.Errorf("消えた案件の内訳が出ていない:\n%s", out)
	}
	// hapitas は内訳未設定（0 件扱い）→ セクションを出さない
	if strings.Contains(out, "消えた案件の内訳（ハピタス）") {
		t.Error("内訳が無いサイトにセクションが出た")
	}

	src2 := scenario()
	src2.goneDetailsErr = errors.New("timeout")
	r2, err := Build(context.Background(), src2, "2026-09-24", ts("2026-09-24 03:20:00"), 30)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(r2.Warnings, "\n"), "モッピー: 消えた案件の内訳を取得できなかった") {
		t.Errorf("警告が無い: %v", r2.Warnings)
	}
}

func TestGroupGoneOffersLimitsExamplesAndSorts(t *testing.T) {
	offers := []GoneOffer{
		{Name: "え", Category: "X"}, {Name: "あ", Category: "X"}, {Name: "い", Category: "X"},
		{Name: "う", Category: "X"}, {Name: "お", Category: "X"}, {Name: "か", Category: "X"},
	}
	got := groupGoneOffers(offers)
	if len(got) != 1 || got[0].Count != 6 {
		t.Fatalf("got = %+v", got)
	}
	want := []string{"あ", "い", "う", "え", "お"}
	if len(got[0].Examples) != len(want) {
		t.Fatalf("Examples = %v", got[0].Examples)
	}
	for i, w := range want {
		if got[0].Examples[i] != w {
			t.Errorf("Examples[%d] = %s, want %s", i, got[0].Examples[i], w)
		}
	}
}

func TestNum(t *testing.T) {
	for in, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1791: "1,791", 1234567: "1,234,567"} {
		if got := num(in); got != want {
			t.Errorf("num(%d) = %q, want %q", in, got, want)
		}
	}
}
