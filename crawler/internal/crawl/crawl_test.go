package crawl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/fetch"
	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/site"
)

// fakeFetcher は URL → 応答の対応表。ネットワークには出ない。
type fakeFetcher struct {
	responses map[string]fakeResponse
	calls     []string
	stopped   *fetch.StopError
}

type fakeResponse struct {
	body []byte
	err  error
}

func (f *fakeFetcher) Get(_ context.Context, u string) ([]byte, error) {
	if f.stopped != nil {
		return nil, f.stopped
	}
	f.calls = append(f.calls, u)
	r, ok := f.responses[u]
	if !ok {
		return nil, &fetch.HTTPError{StatusCode: 404, URL: u}
	}
	if r.err != nil {
		var stop *fetch.StopError
		if errors.As(r.err, &stop) {
			f.stopped = stop
		}
		return nil, r.err
	}
	return r.body, nil
}

func (f *fakeFetcher) Requests() int { return len(f.calls) }

// fakeStore は呼び出しを記録する。
type fakeStore struct {
	started    bool
	saved      []Offer
	saveErr    error
	finished   *Summary
	finishedID int64
}

func (s *fakeStore) StartCrawlLog(context.Context, string, string, time.Time, string) (int64, error) {
	s.started = true
	return 7, nil
}

func (s *fakeStore) SaveOffers(_ context.Context, _, _ string, offers []Offer) error {
	s.saved = offers
	return s.saveErr
}

func (s *fakeStore) FinishCrawlLog(_ context.Context, id int64, sum Summary) error {
	s.finishedID = id
	s.finished = &sum
	return nil
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "moppy", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func moppyDef(t *testing.T) *site.Definition {
	t.Helper()
	d, err := site.LoadByID(filepath.Join("..", "..", "sites"), "moppy")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// 2 カテゴリだけのメニュー。href の host は使われない（クエリだけ見る）。
const smallMenu = `<ul>
<li><a href="https://pc.moppy.jp/category/list.php?parent_category=2" data-ga-label="広告ジャンルで探す - クレジットカード">x</a></li>
<li><a href="https://pc.moppy.jp/category/list.php?parent_category=3&amp;child_category=43" data-ga-label="広告ジャンルで探す - 金融 - 証券会社">y</a></li>
<li><a href="#" data-ga-label="広告ジャンルで探す - 金融">z</a></li>
</ul>`

func listURL(t *testing.T, d *site.Definition, parent, child string, page int) string {
	t.Helper()
	u, err := d.RenderURL(map[string]string{"parent_category": parent, "child_category": child}, page)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func fixedNow() func() time.Time {
	// 2026-09-22 18:30 UTC = 2026-09-23 03:30 JST → crawled_on は JST の日付
	return func() time.Time { return time.Date(2026, 9, 22, 18, 30, 0, 0, time.UTC) }
}

func TestRunCrawlsCategoriesWithPagingAndDedupe(t *testing.T) {
	d := moppyDef(t)
	f := &fakeFetcher{responses: map[string]fakeResponse{
		"https://pc.moppy.jp/robots.txt":                 {body: fixture(t, "robots.txt")},
		"https://pc.moppy.jp/ajax/category/get_menu.php": {body: []byte(smallMenu)},
		listURL(t, d, "2", "0", 1):                       {body: fixture(t, "list_credit_p1.html")},   // 5 ページと申告
		listURL(t, d, "2", "0", 2):                       {body: fixture(t, "list_shopping_p1.html")}, // 別の 30 件
		listURL(t, d, "2", "0", 3):                       {body: fixture(t, "list_empty.html")},       // 0 件で打ち切り
		listURL(t, d, "3", "43", 1):                      {body: fixture(t, "list_credit_p1.html")},   // 全件重複
		listURL(t, d, "3", "43", 2):                      {body: fixture(t, "list_empty.html")},
	}}
	st := &fakeStore{}

	res, err := Run(context.Background(), d, f, st, Options{Version: "abc1234", Now: fixedNow()})
	if err != nil {
		t.Fatal(err)
	}
	s := res.Summary
	if s.CrawledOn != "2026-09-23" {
		t.Errorf("CrawledOn = %s（JST で切る）", s.CrawledOn)
	}
	if s.Status != "success" {
		t.Errorf("Status = %s, errors = %v", s.Status, s.Errors)
	}
	if s.CategoryCount != 2 {
		t.Errorf("CategoryCount = %d", s.CategoryCount)
	}
	// robots + menu + (p1,p2,p3) + (p1,p2) = 7
	if s.RequestCount != 7 || len(f.calls) != 7 {
		t.Errorf("RequestCount = %d, calls = %v", s.RequestCount, f.calls)
	}
	if s.PageCount != 5 {
		t.Errorf("PageCount = %d", s.PageCount)
	}
	if s.OfferCount != 60 || s.ParsedCount != 60 {
		t.Errorf("OfferCount = %d, ParsedCount = %d, want 60 / 60（重複排除）", s.OfferCount, s.ParsedCount)
	}
	if len(st.saved) != 60 || !st.started || st.finishedID != 7 || st.finished == nil {
		t.Errorf("store: saved=%d started=%v finishedID=%d", len(st.saved), st.started, st.finishedID)
	}
	for _, o := range st.saved {
		if !strings.HasPrefix(o.URL, "https://pc.moppy.jp/") || strings.Contains(o.URL, "track_ref") {
			t.Errorf("URL が正規化されていない: %s", o.URL)
		}
		if o.ExternalID == "" || o.Name == "" || o.RewardRaw == "" {
			t.Errorf("欠けがある: %+v", o)
		}
	}
	// 先に見たカテゴリのラベルが付く
	if st.saved[0].Category != "クレジットカード" {
		t.Errorf("Category = %q", st.saved[0].Category)
	}
	if st.finished.Status != "success" || st.finished.FinishedAt.IsZero() {
		t.Errorf("finished = %+v", st.finished)
	}
}

func TestRunRecordsHTTPErrorAndContinues(t *testing.T) {
	d := moppyDef(t)
	f := &fakeFetcher{responses: map[string]fakeResponse{
		"https://pc.moppy.jp/robots.txt":                 {body: fixture(t, "robots.txt")},
		"https://pc.moppy.jp/ajax/category/get_menu.php": {body: []byte(smallMenu)},
		// parent 2 は 404（対応表に無い）、parent 3 は正常
		listURL(t, d, "3", "43", 1): {body: fixture(t, "list_shopping_p1.html")},
		listURL(t, d, "3", "43", 2): {body: fixture(t, "list_empty.html")},
	}}
	st := &fakeStore{}
	res, err := Run(context.Background(), d, f, st, Options{Now: fixedNow()})
	if err != nil {
		t.Fatal(err)
	}
	s := res.Summary
	if s.Status != "partial" || s.ErrorCount != 1 || len(s.Errors) != 1 {
		t.Errorf("Status = %s, ErrorCount = %d, Errors = %v", s.Status, s.ErrorCount, s.Errors)
	}
	if s.OfferCount != 30 || len(st.saved) != 30 {
		t.Errorf("OfferCount = %d", s.OfferCount)
	}
	if !strings.Contains(s.Errors[0].Message, "404") {
		t.Errorf("Errors[0] = %+v", s.Errors[0])
	}
}

func TestRunAbortsOnCircuitBreakerAndSavesPartialData(t *testing.T) {
	d := moppyDef(t)
	stop := &fetch.StopError{Reason: "http_429", StatusCode: 429, URL: listURL(t, d, "2", "0", 2)}
	f := &fakeFetcher{responses: map[string]fakeResponse{
		"https://pc.moppy.jp/robots.txt":                 {body: fixture(t, "robots.txt")},
		"https://pc.moppy.jp/ajax/category/get_menu.php": {body: []byte(smallMenu)},
		listURL(t, d, "2", "0", 1):                       {body: fixture(t, "list_credit_p1.html")},
		listURL(t, d, "2", "0", 2):                       {err: stop},
		listURL(t, d, "3", "43", 1):                      {body: fixture(t, "list_shopping_p1.html")},
	}}
	st := &fakeStore{}
	res, err := Run(context.Background(), d, f, st, Options{Now: fixedNow()})
	if err != nil {
		t.Fatal(err)
	}
	s := res.Summary
	if s.Status != "aborted" || s.AbortReason != "http_429" {
		t.Errorf("Status = %s, AbortReason = %s", s.Status, s.AbortReason)
	}
	// 停止後は次のカテゴリを取りに行かない
	if s.RequestCount != 4 {
		t.Errorf("RequestCount = %d, calls = %v", s.RequestCount, f.calls)
	}
	if s.OfferCount != 30 || len(st.saved) != 30 {
		t.Errorf("途中までのデータが保存されていない: OfferCount = %d, saved = %d", s.OfferCount, len(st.saved))
	}
}

func TestRunAbortsWhenRobotsDisallows(t *testing.T) {
	d := moppyDef(t)
	f := &fakeFetcher{responses: map[string]fakeResponse{
		"https://pc.moppy.jp/robots.txt":                 {body: []byte("User-agent: *\nDisallow: /ajax/\n")},
		"https://pc.moppy.jp/ajax/category/get_menu.php": {body: []byte(smallMenu)},
	}}
	st := &fakeStore{}
	res, err := Run(context.Background(), d, f, st, Options{Now: fixedNow()})
	if err != nil {
		t.Fatal(err)
	}
	s := res.Summary
	if s.Status != "aborted" || s.AbortReason != "robots_disallow" {
		t.Errorf("Status = %s, AbortReason = %s, Errors = %v", s.Status, s.AbortReason, s.Errors)
	}
	if s.RequestCount != 1 {
		t.Errorf("robots.txt 以外を取得している: %v", f.calls)
	}
	if st.saved != nil {
		t.Error("保存が呼ばれた")
	}
}

func TestRunTreatsMissingRobotsAsAllowAll(t *testing.T) {
	d := moppyDef(t)
	f := &fakeFetcher{responses: map[string]fakeResponse{
		// robots.txt は 404（対応表に無い）
		"https://pc.moppy.jp/ajax/category/get_menu.php": {body: []byte(smallMenu)},
		listURL(t, d, "2", "0", 1):                       {body: fixture(t, "list_empty.html")},
		listURL(t, d, "3", "43", 1):                      {body: fixture(t, "list_empty.html")},
	}}
	res, err := Run(context.Background(), d, f, &fakeStore{}, Options{Now: fixedNow()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.AbortReason != "" {
		t.Errorf("AbortReason = %s", res.Summary.AbortReason)
	}
	// 0 件なので failed
	if res.Summary.Status != "failed" || res.Summary.OfferCount != 0 {
		t.Errorf("Status = %s, OfferCount = %d", res.Summary.Status, res.Summary.OfferCount)
	}
}

func TestRunAbortsWhenDiscoveryIsEmpty(t *testing.T) {
	d := moppyDef(t)
	f := &fakeFetcher{responses: map[string]fakeResponse{
		"https://pc.moppy.jp/robots.txt":                 {body: fixture(t, "robots.txt")},
		"https://pc.moppy.jp/ajax/category/get_menu.php": {body: []byte("<html><body>menu changed</body></html>")},
	}}
	res, err := Run(context.Background(), d, f, &fakeStore{}, Options{Now: fixedNow()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Status != "aborted" || res.Summary.AbortReason != "discovery_empty" {
		t.Errorf("Status = %s, AbortReason = %s", res.Summary.Status, res.Summary.AbortReason)
	}
}

func TestRunMarksFailedWhenSaveFails(t *testing.T) {
	d := moppyDef(t)
	f := &fakeFetcher{responses: map[string]fakeResponse{
		"https://pc.moppy.jp/robots.txt":                 {body: fixture(t, "robots.txt")},
		"https://pc.moppy.jp/ajax/category/get_menu.php": {body: []byte(smallMenu)},
		listURL(t, d, "2", "0", 1):                       {body: fixture(t, "list_credit_p1.html")},
		listURL(t, d, "2", "0", 2):                       {body: fixture(t, "list_empty.html")},
		listURL(t, d, "3", "43", 1):                      {body: fixture(t, "list_empty.html")},
	}}
	st := &fakeStore{saveErr: errors.New("db down")}
	res, err := Run(context.Background(), d, f, st, Options{Now: fixedNow()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Status != "failed" || res.Summary.ErrorCount != 1 {
		t.Errorf("Status = %s, Errors = %v", res.Summary.Status, res.Summary.Errors)
	}
	if st.finished == nil || st.finished.Status != "failed" {
		t.Error("crawl_logs が failed で締められていない")
	}
}

func TestStatus(t *testing.T) {
	cases := []struct {
		s        Summary
		saveFail bool
		want     string
	}{
		{Summary{OfferCount: 10}, false, "success"},
		{Summary{OfferCount: 10, ErrorCount: 1}, false, "partial"},
		{Summary{OfferCount: 0}, false, "failed"},
		{Summary{OfferCount: 10}, true, "failed"},
		{Summary{OfferCount: 10, AbortReason: "http_429"}, false, "aborted"},
	}
	for _, c := range cases {
		if got := status(c.s, c.saveFail); got != c.want {
			t.Errorf("status(%+v, %v) = %s, want %s", c.s, c.saveFail, got, c.want)
		}
	}
}
