package extract

import (
	"fmt"
	"os"
	"path/filepath"
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
