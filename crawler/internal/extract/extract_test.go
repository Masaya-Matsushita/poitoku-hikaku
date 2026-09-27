package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/site"
)

func loadMoppy(t *testing.T) *site.Definition {
	t.Helper()
	d, err := site.LoadByID(filepath.Join("..", "..", "sites"), "moppy")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "moppy", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMoppyCreditCardListing(t *testing.T) {
	d := loadMoppy(t)
	doc, err := Parse(fixture(t, "list_credit_p1.html"))
	if err != nil {
		t.Fatal(err)
	}

	items, skipped, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 30 || skipped != 0 {
		t.Fatalf("items = %d, skipped = %d, want 30 / 0", len(items), skipped)
	}
	for i, it := range items {
		if it.Name == "" || it.Href == "" || it.RewardRaw == "" {
			t.Errorf("items[%d] に空欄: %+v", i, it)
		}
		if !strings.Contains(it.Href, "detail.php?site_id=") {
			t.Errorf("items[%d].Href = %q", i, it.Href)
		}
		if r := ParseReward(it.RewardRaw); r.Points == nil && r.Percent == nil {
			t.Errorf("items[%d] の還元額 %q を数値化できない", i, it.RewardRaw)
		}
	}

	last, err := LastPage(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if last != 5 {
		t.Errorf("LastPage = %d, want 5（141 件 / 30 件）", last)
	}
}

func TestMoppyShoppingListingHasPercentRewards(t *testing.T) {
	d := loadMoppy(t)
	doc, err := Parse(fixture(t, "list_shopping_p1.html"))
	if err != nil {
		t.Fatal(err)
	}
	items, _, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 30 {
		t.Fatalf("items = %d, want 30", len(items))
	}
	percent := 0
	for _, it := range items {
		if r := ParseReward(it.RewardRaw); r.Percent != nil {
			percent++
		}
	}
	if percent < 20 {
		t.Errorf("率表記の案件が %d 件しか数値化できていない", percent)
	}
	if last, _ := LastPage(doc, d.Listing); last != 3 {
		t.Errorf("LastPage = %d, want 3", last)
	}
}

// ポイント対象外の案件は .a-list__item__point が無く .a-list__item__benefit に文言が出る。
// 空文字（抽出失敗扱い）ではなく文言を拾い、0 ポイントとして数値化する。
func TestMoppyNoRewardOffersUseFallback(t *testing.T) {
	d := loadMoppy(t)
	doc, err := Parse(fixture(t, "list_furusato_p1.html"))
	if err != nil {
		t.Fatal(err)
	}
	items, skipped, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 || skipped != 0 {
		t.Fatalf("items = %d, skipped = %d, want 5 / 0", len(items), skipped)
	}
	for _, it := range items {
		if it.RewardRaw != "ポイント対象外" {
			t.Errorf("%s: RewardRaw = %q, want ポイント対象外", it.Name, it.RewardRaw)
		}
		r := ParseReward(it.RewardRaw)
		if r.Points == nil || *r.Points != 0 || r.Percent != nil {
			t.Errorf("%s: ParseReward = %s, want points=0", it.Name, fmtReward(r))
		}
	}
}

func TestMoppyEmptyPage(t *testing.T) {
	d := loadMoppy(t)
	doc, err := Parse(fixture(t, "list_empty.html"))
	if err != nil {
		t.Fatal(err)
	}
	items, skipped, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 || skipped != 0 {
		t.Errorf("items = %d, skipped = %d, want 0 / 0", len(items), skipped)
	}
	if last, _ := LastPage(doc, d.Listing); last != 1 {
		t.Errorf("LastPage = %d, want 1", last)
	}
}

func TestMoppyMenuDiscovery(t *testing.T) {
	d := loadMoppy(t)
	doc, err := Parse(fixture(t, "menu.html"))
	if err != nil {
		t.Fatal(err)
	}
	links, err := Links(doc, d.Discovery)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 93 {
		t.Fatalf("links = %d, want 93（親のみ 1 + 親子 92）", len(links))
	}
	seen := map[string]bool{}
	for _, l := range links {
		if strings.HasPrefix(l.Label, "広告ジャンルで探す") || l.Label == "" {
			t.Errorf("ラベルの整形が不十分: %q", l.Label)
		}
		params, err := QueryParams(l.Href)
		if err != nil {
			t.Fatal(err)
		}
		if params["parent_category"] == "" {
			t.Errorf("parent_category が無い: %s", l.Href)
		}
		if seen[l.Href] {
			t.Errorf("重複リンク: %s", l.Href)
		}
		seen[l.Href] = true
	}
	if links[0].Label != "クレジットカード" {
		t.Errorf("先頭のラベル = %q", links[0].Label)
	}
}

func TestCanonicalAndExternalID(t *testing.T) {
	d := loadMoppy(t)
	base := d.Base()
	re := d.ExternalIDPattern()
	cases := []struct {
		href string
		want string
		id   string
	}{
		{"https://pc.moppy.jp/ad/detail.php?site_id=160005&track_ref=car", "https://pc.moppy.jp/ad/detail.php?site_id=160005", "160005"},
		{"/ad/detail.php?track_ref=rpr&s_id=160005#top", "https://pc.moppy.jp/ad/detail.php?site_id=160005", "160005"},
		{"/shopping/detail.php?site_id=42&track_ref=x", "https://pc.moppy.jp/shopping/detail.php?site_id=42", "42"},
		{"/campaign/", "https://pc.moppy.jp/campaign/", ""},
	}
	for _, c := range cases {
		got, err := Canonical(c.href, base, d.URL)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("Canonical(%q) = %q, want %q", c.href, got, c.want)
		}
		if id := ExternalID(got, re); id != c.id {
			t.Errorf("ExternalID(%q) = %q, want %q", got, id, c.id)
		}
	}
}

func loadHapitas(t *testing.T) *site.Definition {
	t.Helper()
	d, err := site.LoadByID(filepath.Join("..", "..", "sites"), "hapitas")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func hapitasFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "hapitas", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHapitasCreditListing(t *testing.T) {
	d := loadHapitas(t)
	doc, err := Parse(hapitasFixture(t, "category_credit.html"))
	if err != nil {
		t.Fatal(err)
	}
	items, skipped, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	// #inner_catalog の 120 件だけ。ピックアップ枠（attention_word 等）は含めない
	if len(items) != 120 || skipped != 0 {
		t.Fatalf("items = %d, skipped = %d, want 120 / 0", len(items), skipped)
	}
	base, re := d.Base(), d.ExternalIDPattern()
	for i, it := range items {
		if it.Name == "" || it.Href == "" || it.RewardRaw == "" {
			t.Errorf("items[%d] に空欄: %+v", i, it)
		}
		if r := ParseReward(it.RewardRaw); r.Points == nil {
			t.Errorf("items[%d] の還元額 %q がポイントとして数値化できない", i, it.RewardRaw)
		}
		u, err := Canonical(it.Href, base, d.URL)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(u, "/apn") || !strings.HasPrefix(u, "https://hapitas.jp/item/detail/itemid/") || !strings.HasSuffix(u, "/") {
			t.Errorf("items[%d] の URL が正規化されていない: %s", i, u)
		}
		if ExternalID(u, re) == "" {
			t.Errorf("items[%d] の external_id が取れない: %s", i, u)
		}
	}
	if total, ok := Total(doc, d.Listing); !ok || total != 137 {
		t.Errorf("Total = %d, %v, want 137", total, ok)
	}
	if last, _ := LastPage(doc, d.Listing); last != 1 {
		t.Errorf("LastPage = %d, want 1（ページネーション無し）", last)
	}
}

func TestHapitasShoppingListingHasPercentRewards(t *testing.T) {
	d := loadHapitas(t)
	doc, err := Parse(hapitasFixture(t, "category_shopping_store.html"))
	if err != nil {
		t.Fatal(err)
	}
	items, _, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 80 {
		t.Fatalf("items = %d, want 80 以上", len(items))
	}
	percent, zero, points := 0, 0, 0
	for _, it := range items {
		r := ParseReward(it.RewardRaw)
		switch {
		case r.Percent != nil:
			percent++
		case r.Points != nil && *r.Points > 0:
			points++
		case r.Points != nil && *r.Points == 0:
			// 還元 0 の案件は .caption が無く、reward_when_empty で「ポイント対象外」になる
			zero++
			u, err := Canonical(it.Href, d.Base(), d.URL)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(u, "https://hapitas.jp/item/detail/itemid/") {
				t.Errorf("還元 0 の案件の URL が詳細 URL に寄っていない: %s → %s", it.Href, u)
			}
		default:
			t.Errorf("数値化できない: %q (%s)", it.RewardRaw, it.Name)
		}
	}
	if percent < 50 || zero < 20 || points == 0 {
		t.Errorf("率表記 %d 件、固定額 %d 件、還元 0 %d 件（想定：50 / 5 / 25 前後）", percent, points, zero)
	}
	if total, ok := Total(doc, d.Listing); !ok || total != 81 {
		t.Errorf("Total = %d, %v, want 81", total, ok)
	}
}

func TestHapitasNavigationDiscovery(t *testing.T) {
	d := loadHapitas(t)
	doc, err := Parse(hapitasFixture(t, "category_credit.html"))
	if err != nil {
		t.Fatal(err)
	}
	links, err := Links(doc, d.Discovery)
	if err != nil {
		t.Fatal(err)
	}
	slugs := map[string]string{}
	for _, l := range links {
		params, err := Params(l.Href, d.ParamPattern())
		if err != nil {
			t.Errorf("%s: %v", l.Href, err)
			continue
		}
		if params["slug"] == "" || l.Label == "" {
			t.Errorf("slug かラベルが空: %+v params=%v", l, params)
		}
		slugs[params["slug"]] = l.Label
	}
	if len(slugs) != 39 {
		t.Errorf("カテゴリ数 = %d, want 39", len(slugs))
	}
	if slugs["service_credit"] != "クレジットカード" {
		t.Errorf("service_credit のラベル = %q", slugs["service_credit"])
	}
}

func TestParamsWithPattern(t *testing.T) {
	re := regexp.MustCompile(`/category/(?P<slug>[a-z_]+)/`)
	params, err := Params("https://hapitas.jp/category/service_credit/apn/navigation_category/?x=1", re)
	if err != nil {
		t.Fatal(err)
	}
	if params["slug"] != "service_credit" || params["x"] != "1" {
		t.Errorf("params = %v", params)
	}
	if _, err := Params("https://hapitas.jp/special/", re); err != ErrNoParamMatch {
		t.Errorf("一致しない href のエラー = %v", err)
	}
}

func TestCanonicalPathPattern(t *testing.T) {
	d := loadHapitas(t)
	cases := []struct{ href, want string }{
		{"https://hapitas.jp/item/detail/itemid/49829/apn/", "https://hapitas.jp/item/detail/itemid/49829/"},
		{"/item/detail/itemid/1594/apn/service_credit_top", "https://hapitas.jp/item/detail/itemid/1594/"},
		{"/item/detail/itemid/7/", "https://hapitas.jp/item/detail/itemid/7/"},
		// 還元 0 の案件の遷移用 URL も同じ案件の詳細 URL に寄せる
		{"/item/redirect-to-client-if-zero-point-item/itemid/93803/apn/", "https://hapitas.jp/item/detail/itemid/93803/"},
		{"/special/x/", "https://hapitas.jp/special/x/"}, // パターンに合わなければそのまま
	}
	for _, c := range cases {
		got, err := Canonical(c.href, d.Base(), d.URL)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("Canonical(%q) = %q, want %q", c.href, got, c.want)
		}
	}
}

func TestCanonicalPathPatternWithoutTemplateKeepsFirstGroup(t *testing.T) {
	base := loadHapitas(t).Base()
	rules := site.URLRules{PathPattern: `^(/item/detail/itemid/[0-9]+/)`}
	got, err := Canonical("/item/detail/itemid/5/apn/x", base, rules)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://hapitas.jp/item/detail/itemid/5/" {
		t.Errorf("got %q", got)
	}
}

func TestCanonicalKeepsAllParamsWhenNoRules(t *testing.T) {
	base := loadMoppy(t).Base()
	got, err := Canonical("/x?b=2&a=1", base, site.URLRules{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://pc.moppy.jp/x?a=1&b=2" {
		t.Errorf("got %q", got)
	}
}

func loadChobirich(t *testing.T) *site.Definition {
	t.Helper()
	d, err := site.LoadByID(filepath.Join("..", "..", "sites"), "chobirich")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func chobirichFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "chobirich", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// サービス（会員登録・資料請求）の一覧 1 ページ目：30 件、全 6 ページ。
// 還元アップ中の案件は「<s>3,000pt</s>→3,500pt」なので、取り消し線の旧額を除いて「→3,500pt」を取る
// （矢印は残るが数値化は新額になる）。
func TestChobirichEarnListing(t *testing.T) {
	d := loadChobirich(t)
	doc, err := Parse(chobirichFixture(t, "list_earn_101_p1.html"))
	if err != nil {
		t.Fatal(err)
	}
	items, skipped, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 30 || skipped != 0 {
		t.Fatalf("items = %d, skipped = %d, want 30 / 0", len(items), skipped)
	}
	byHref := map[string]Item{}
	for i, it := range items {
		if it.Name == "" || it.Href == "" || it.RewardRaw == "" {
			t.Errorf("items[%d] に空欄: %+v", i, it)
		}
		if strings.Contains(it.Href, "#") {
			t.Errorf("items[%d] は口コミへのリンクを拾っている: %+v", i, it)
		}
		if r := ParseReward(it.RewardRaw); r.Points == nil && r.Percent == nil {
			t.Errorf("items[%d] の還元額 %q を数値化できない", i, it.RewardRaw)
		}
		byHref[it.Href] = it
	}
	it := byHref["/ad_details/50254/"]
	if r := ParseReward(it.RewardRaw); it.Name != "palsystem（パルシステム）資料請求" || it.RewardRaw != "→3,500pt" || r.Points == nil || *r.Points != 3500 {
		t.Errorf("取り消し線付きの案件 = %+v, want →3,500pt（3500 ポイント）", it)
	}

	last, err := LastPage(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if last != 6 {
		t.Errorf("LastPage = %d, want 6", last)
	}
}

// お買い物（総合通販）の一覧：率の還元（"1%" と全角の "1％"）、還元なし（空と "0"）が混在する。
func TestChobirichShoppingListing(t *testing.T) {
	d := loadChobirich(t)
	doc, err := Parse(chobirichFixture(t, "list_shop_101_p1.html"))
	if err != nil {
		t.Fatal(err)
	}
	items, skipped, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 30 || skipped != 0 {
		t.Fatalf("items = %d, skipped = %d, want 30 / 0", len(items), skipped)
	}
	percent, zero := 0, 0
	byHref := map[string]Item{}
	for i, it := range items {
		r := ParseReward(it.RewardRaw)
		switch {
		case r.Percent != nil:
			percent++
		case r.Points != nil && *r.Points == 0:
			zero++
		case r.Points == nil:
			t.Errorf("items[%d] の還元額 %q を数値化できない", i, it.RewardRaw)
		}
		byHref[it.Href] = it
	}
	if percent < 20 {
		t.Errorf("率の案件が %d 件（お買い物は大半が率）", percent)
	}
	if zero != 3 {
		t.Errorf("還元なしの案件 = %d, want 3（Amazon・ANAのふるさと納税は空、ヨリヤスは 0）", zero)
	}
	if got := byHref["/ad_details/18999"].RewardRaw; got != "ポイント対象外" {
		t.Errorf("Amazon の還元額 = %q, want ポイント対象外", got)
	}

	last, err := LastPage(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if last != 2 {
		t.Errorf("LastPage = %d, want 2（\">\" のリンクは数字でないので無視）", last)
	}
}

func TestChobirichEmptyCategory(t *testing.T) {
	d := loadChobirich(t)
	doc, err := Parse(chobirichFixture(t, "list_empty.html"))
	if err != nil {
		t.Fatal(err)
	}
	items, skipped, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 || skipped != 0 {
		t.Errorf("items = %d, skipped = %d, want 0 / 0", len(items), skipped)
	}
}

// トップページのサイドメニューから 20 カテゴリ（お買い物 11 + サービス 9）を発見する。
func TestChobirichTopDiscovery(t *testing.T) {
	d := loadChobirich(t)
	doc, err := Parse(chobirichFixture(t, "top.html"))
	if err != nil {
		t.Fatal(err)
	}
	links, err := Links(doc, d.Discovery)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, l := range links {
		params, err := Params(l.Href, d.ParamPattern())
		if err != nil {
			t.Errorf("param_pattern に合わない: %+v", l)
			continue
		}
		u, err := d.RenderURL(d.Listing.Templates()[0], params, 1)
		if err != nil {
			t.Fatal(err)
		}
		seen[u] = l.Label
	}
	if len(seen) != 20 {
		t.Errorf("カテゴリ = %d 件, want 20: %v", len(seen), seen)
	}
	for u, label := range map[string]string{
		"https://www.chobirich.com/shopping/shop/101?page=1": "総合通販 ・オークション", // リンク内の <br> は空白になる
		"https://www.chobirich.com/earn/apply/104?page=1":    "クレジットカード",
	} {
		if seen[u] != label {
			t.Errorf("%s のラベル = %q, want %q", u, seen[u], label)
		}
	}
}

func TestChobirichCanonical(t *testing.T) {
	d := loadChobirich(t)
	for _, href := range []string{"/ad_details/50254", "/ad_details/50254/", "https://www.chobirich.com/ad_details/50254/#shopping_rate"} {
		got, err := Canonical(href, d.Base(), d.URL)
		if err != nil {
			t.Fatal(err)
		}
		if got != "https://www.chobirich.com/ad_details/50254/" {
			t.Errorf("Canonical(%q) = %q", href, got)
		}
		if id := ExternalID(got, d.ExternalIDPattern()); id != "50254" {
			t.Errorf("ExternalID = %q", id)
		}
	}
}

func TestParseReward(t *testing.T) {
	i := func(v int64) *int64 { return &v }
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		raw     string
		points  *int64
		percent *float64
	}{
		{"14,000P", i(14000), nil},
		{"1,500P", i(1500), nil},
		{"300P", i(300), nil},
		{"最大10,000P", i(10000), nil},
		{"１４，０００Ｐ", i(14000), nil},
		{"1.0%", nil, f(1.0)},
		{"20.0%", nil, f(20.0)},
		{"0.5％", nil, f(0.5)},
		{"1,000pt", i(1000), nil},
		{"1000ポイント", i(1000), nil},
		{"ポイント対象外", i(0), nil},
		{"対象外", i(0), nil},
		{"0", i(0), nil},
		{" ０ ", i(0), nil},
		{"", nil, nil},
		{"10", nil, nil}, // 単位の無い 0 以外の数は額か率か分からない
		{"要確認", nil, nil},
		{"P", nil, nil},
	}
	for _, c := range cases {
		got := ParseReward(c.raw)
		switch {
		case c.points != nil:
			if got.Points == nil || *got.Points != *c.points || got.Percent != nil {
				t.Errorf("ParseReward(%q) = %s, want points %d", c.raw, fmtReward(got), *c.points)
			}
		case c.percent != nil:
			if got.Percent == nil || *got.Percent != *c.percent || got.Points != nil {
				t.Errorf("ParseReward(%q) = %s, want percent %v", c.raw, fmtReward(got), *c.percent)
			}
		default:
			if got.Points != nil || got.Percent != nil {
				t.Errorf("ParseReward(%q) = %s, want nothing", c.raw, fmtReward(got))
			}
		}
	}
}

func fmtReward(r Reward) string {
	switch {
	case r.Points != nil && r.Percent != nil:
		return fmt.Sprintf("points=%d,percent=%v", *r.Points, *r.Percent)
	case r.Points != nil:
		return fmt.Sprintf("points=%d", *r.Points)
	case r.Percent != nil:
		return fmt.Sprintf("percent=%v", *r.Percent)
	default:
		return "nothing"
	}
}

func loadPointtown(t *testing.T) *site.Definition {
	t.Helper()
	d, err := site.LoadByID(filepath.Join("..", "..", "sites"), "pointtown")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func pointtownFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "pointtown", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPointtownCreditListing(t *testing.T) {
	d := loadPointtown(t)
	doc, err := Parse(pointtownFixture(t, "category_service_creditcard_p1.html"))
	if err != nil {
		t.Fatal(err)
	}
	items, skipped, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 20 || skipped != 0 {
		t.Fatalf("items = %d, skipped = %d, want 20 / 0", len(items), skipped)
	}
	base, re := d.Base(), d.ExternalIDPattern()
	for i, it := range items {
		if it.Name == "" || it.Href == "" || it.RewardRaw == "" {
			t.Errorf("items[%d] に空欄: %+v", i, it)
		}
		// 一覧のテキストは「…」で省略されるが、画像の alt には全文がある
		if strings.HasSuffix(it.Name, "…") {
			t.Errorf("items[%d] の案件名が省略されている: %q", i, it.Name)
		}
		// 単位の無い "12,000" に reward_unit の pt が付き、ポイントとして数値化できる
		if r := ParseReward(it.RewardRaw); r.Points == nil || !strings.HasSuffix(it.RewardRaw, "pt") {
			t.Errorf("items[%d] の還元額 %q がポイントとして数値化できない", i, it.RewardRaw)
		}
		u, err := Canonical(it.Href, base, d.URL)
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^https://www\.pointtown\.com/item/[0-9]+$`).MatchString(u) {
			t.Errorf("items[%d] の URL が詳細 URL でない: %s", i, u)
		}
		if ExternalID(u, re) == "" {
			t.Errorf("items[%d] の external_id が取れない: %s", i, u)
		}
	}
	// ポイントアップ中の案件は元の額（9,250）ではなく今の額（15,000）を取る
	found := false
	for _, it := range items {
		if strings.HasPrefix(it.Name, "三菱ＵＦＪカード・プラチナ・アメリカン・エキスプレス") {
			found = true
			if it.RewardRaw != "15,000pt" {
				t.Errorf("UP 中の案件の還元額 = %q, want 15,000pt", it.RewardRaw)
			}
			if it.Name != "三菱ＵＦＪカード・プラチナ・アメリカン・エキスプレス®・カード" {
				t.Errorf("案件名 = %q（alt の全文が取れていない）", it.Name)
			}
		}
	}
	if !found {
		t.Error("UP 中の案件がフィクスチャに見つからない")
	}
	if total, ok := Total(doc, d.Listing); !ok || total != 107 {
		t.Errorf("Total = %d, %v, want 107", total, ok)
	}
	if last, _ := LastPage(doc, d.Listing); last != 6 {
		t.Errorf("LastPage = %d, want 6（20 件/ページで 107 件）", last)
	}
}

func TestPointtownShoppingListingHasPercentRewards(t *testing.T) {
	d := loadPointtown(t)
	doc, err := Parse(pointtownFixture(t, "category_shopping_mailorder.html"))
	if err != nil {
		t.Fatal(err)
	}
	items, skipped, err := Items(doc, d.Listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 20 || skipped != 0 {
		t.Fatalf("items = %d, skipped = %d, want 20 / 0", len(items), skipped)
	}
	percent := 0
	for _, it := range items {
		r := ParseReward(it.RewardRaw)
		switch {
		case r.Percent != nil:
			percent++
			if strings.HasSuffix(it.RewardRaw, "pt") {
				t.Errorf("率表記に単位が付いた: %q", it.RewardRaw)
			}
		case r.Points != nil:
		default:
			t.Errorf("数値化できない: %q (%s)", it.RewardRaw, it.Name)
		}
	}
	if percent < 10 {
		t.Errorf("率表記 %d 件（想定：大半が率）", percent)
	}
	if total, ok := Total(doc, d.Listing); !ok || total != 30 {
		t.Errorf("Total = %d, %v, want 30", total, ok)
	}
	if last, _ := LastPage(doc, d.Listing); last != 2 {
		t.Errorf("LastPage = %d, want 2", last)
	}
}

func TestPointtownCategoryDiscovery(t *testing.T) {
	d := loadPointtown(t)
	doc, err := Parse(pointtownFixture(t, "category_index.html"))
	if err != nil {
		t.Fatal(err)
	}
	links, err := Links(doc, d.Discovery)
	if err != nil {
		t.Fatal(err)
	}
	cats := map[string]string{}
	for _, l := range links {
		params, err := Params(l.Href, d.ParamPattern())
		if err != nil {
			continue // /category 自体など、一覧でないリンク
		}
		if l.Label == "" {
			t.Errorf("ラベルが空: %+v", l)
		}
		cats[params["group"]+"/"+params["slug"]] = l.Label
	}
	if len(cats) != 26 {
		t.Errorf("カテゴリ数 = %d, want 26: %v", len(cats), cats)
	}
	// 同じ slug がショッピングとサービスの両方にある（beauty / other）ので group で区別する
	for key, want := range map[string]string{
		"service/creditcard": "クレジットカード",
		"shopping/beauty":    "ビューティー/コスメ",
		"service/beauty":     "美容/エステ",
		"shopping/other":     "その他(ショッピング)",
	} {
		if cats[key] != want {
			t.Errorf("%s のラベル = %q, want %q", key, cats[key], want)
		}
	}
}

func TestItemsRewardUnitOnlyForBareNumbers(t *testing.T) {
	doc, err := Parse([]byte(`<ul>
<li class="i"><a href="/a" class="n">A</a><p class="r">12,000</p></li>
<li class="i"><a href="/b" class="n">B</a><p class="r">3.5%</p></li>
<li class="i"><a href="/c" class="n">C</a><p class="r">最大500</p></li>
<li class="i"><a href="/d" class="n">D</a></li>
</ul>`))
	if err != nil {
		t.Fatal(err)
	}
	l := site.Listing{ItemSelector: ".i", NameSelector: ".n", RewardSelector: ".r", LinkSelector: "a", RewardUnit: "pt"}
	items, _, err := Items(doc, l)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"12,000pt", "3.5%", "最大500", ""}
	for i, it := range items {
		if it.RewardRaw != want[i] {
			t.Errorf("items[%d].RewardRaw = %q, want %q", i, it.RewardRaw, want[i])
		}
	}
}
