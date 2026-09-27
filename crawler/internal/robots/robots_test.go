package robots

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/policy"
)

func TestMoppyRobots(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "moppy", "robots.txt"))
	if err != nil {
		t.Fatal(err)
	}
	r := Parse(body)
	cases := []struct {
		path string
		want bool
	}{
		{"/", true},
		{"/ajax/category/get_menu.php", true},
		{"/ajax/category/get_list.php?parent_category=2&child_category=0&current_page=1", true},
		{"/ad/detail.php?site_id=1", true},
		{"/ad/j.php", false},
		{"/ad/r.php?x=1", false},
		{"/receive/", false},
		{"/receive/abc", false},
		{"/img/logo.png", false},
		{"/notfound.php", false},
	}
	for _, c := range cases {
		if got := r.Allowed(policy.UserAgent, c.path); got != c.want {
			t.Errorf("Allowed(%q) = %v, want %v", c.path, got, c.want)
		}
	}
	// ia_archiver は全面禁止のグループ
	if r.Allowed("ia_archiver", "/") {
		t.Error("ia_archiver が許可されている")
	}
	// Mediapartners-Google は Disallow: (空) = 全部許可
	if !r.Allowed("Mediapartners-Google", "/ad/j.php") {
		t.Error("Mediapartners-Google の空 Disallow が全許可になっていない")
	}
}

func TestSpecificGroupBeatsWildcard(t *testing.T) {
	r := Parse([]byte(`
User-agent: *
Disallow: /

User-agent: poitoku-hikaku
Disallow: /private/
`))
	if !r.Allowed(policy.UserAgent, "/ajax/x") {
		t.Error("自分向けグループより * が優先されている")
	}
	if r.Allowed(policy.UserAgent, "/private/a") {
		t.Error("自分向けグループの Disallow が効いていない")
	}
	if r.Allowed("other-bot/1.0", "/ajax/x") {
		t.Error("* グループの Disallow: / が効いていない")
	}
}

func TestLongestMatchWinsAndAllowTiesWin(t *testing.T) {
	r := Parse([]byte(`
User-agent: *
Disallow: /a/
Allow: /a/b/
Disallow: /c
Allow: /c
Disallow: /*.php$
`))
	cases := []struct {
		path string
		want bool
	}{
		{"/a/x", false},
		{"/a/b/x", true},
		{"/c", true},      // 同長なら Allow
		{"/x.php", false}, // $ で末尾一致
		{"/x.php?q=1", true},
		{"/other", true},
	}
	for _, c := range cases {
		if got := r.Allowed("bot", c.path); got != c.want {
			t.Errorf("Allowed(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestNoRulesAllowsEverything(t *testing.T) {
	if !Parse(nil).Allowed(policy.UserAgent, "/anything") {
		t.Error("空の robots.txt で拒否された")
	}
	var nilRules *Rules
	if !nilRules.Allowed(policy.UserAgent, "/") {
		t.Error("nil Rules で拒否された")
	}
}

// ちょびリッチの robots.txt は User-agent: * のグループが 2 つあり、1 つ目は Allow: /ads.txt だけ、
// Disallow は 2 つ目にある。同じ UA のグループは結合する（RFC 9309 2.2.1）。
func TestChobirichRobotsMergesWildcardGroups(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "chobirich", "robots.txt"))
	if err != nil {
		t.Fatal(err)
	}
	r := Parse(body)
	cases := []struct {
		path string
		want bool
	}{
		{"/", true},
		{"/ads.txt", true},
		{"/shopping/shop/101?page=2", true},
		{"/earn/apply/104?page=1", true},
		{"/ad_details/1895190/", true},
		{"/ad_details/redirect/1666103", false},
		{"/ad_details/56546/", false},
		{"/member/", false},
		{"/regist/", false},
		{"/shopping/shop/101?utm_source=x", false},
	}
	for _, c := range cases {
		if got := r.Allowed(policy.UserAgent, c.path); got != c.want {
			t.Errorf("Allowed(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestSpecificGroupsAreMerged(t *testing.T) {
	r := Parse([]byte(`
User-agent: poitoku-hikaku
Disallow: /a/

User-agent: *
Disallow: /

User-agent: poitoku-hikaku
Disallow: /b/
`))
	if r.Allowed(policy.UserAgent, "/a/x") || r.Allowed(policy.UserAgent, "/b/x") {
		t.Error("自分向けの 2 つのグループが結合されていない")
	}
	if !r.Allowed(policy.UserAgent, "/c") {
		t.Error("自分向けグループがあるのに * グループの規則が混ざっている")
	}
}
