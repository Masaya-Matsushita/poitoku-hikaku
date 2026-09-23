package report

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeSource struct {
	sites []Site
	logs  []CrawlLog
	empty map[string]int // "site|date" → 件数
	err   error
}

func (f *fakeSource) Sites(context.Context) ([]Site, error) { return f.sites, f.err }

func (f *fakeSource) CrawlLogs(_ context.Context, from, to string) ([]CrawlLog, error) {
	var out []CrawlLog
	for _, l := range f.logs {
		if l.CrawledOn >= from && l.CrawledOn <= to {
			out = append(out, l)
		}
	}
	SortLogs(out)
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
		empty: map[string]int{"moppy|2026-09-24": 2},
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
	if m.Log == nil || m.Log.ID != 5 || m.PrevOffers != 1791 || !m.EmptyKnown || m.EmptyRewards != 2 {
		t.Errorf("moppy = %+v", m)
	}
	if h.Log == nil || h.Log.Status != "aborted" || h.PrevOffers != 2773 {
		t.Errorf("hapitas = %+v", h)
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
		"ハピタス: 案件数が前日比 50% 未満（2773 → 900）",
		"モッピー: 一部エラー（1 件）",
		"モッピー: 数値化率 98.8% が目標 99% を下回る",
		"モッピー: 真の抽出失敗（reward_raw が空）が 2 件",
		"モッピー: 案件数が前日より減少（1791 → 1700）",
		"直近 7 日のクロール成功率 80.0% が目標 95% を下回る（4 / 5 サイト×日）",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("警告に %q が無い:\n%s", want, joined)
		}
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

func TestNum(t *testing.T) {
	for in, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1791: "1,791", 1234567: "1,234,567"} {
		if got := num(in); got != want {
			t.Errorf("num(%d) = %q, want %q", in, got, want)
		}
	}
}
