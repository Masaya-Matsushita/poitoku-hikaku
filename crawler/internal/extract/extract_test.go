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
		{"", nil, nil},
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
