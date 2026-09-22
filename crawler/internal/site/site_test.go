package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// リポジトリの sites/*.yaml がすべて読み込めて検証を通ることを担保する。
// セレクタ定義を壊す変更（Routine の自動マージ対象）はここで落ちる。
func TestAllSiteDefinitionsAreValid(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("..", "..", "sites", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("sites/*.yaml が見つからない")
	}
	for _, path := range matches {
		d, err := Load(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		want := strings.TrimSuffix(filepath.Base(path), ".yaml")
		if d.ID != want {
			t.Errorf("%s: id %q がファイル名 %q と一致しない", path, d.ID, want)
		}
	}
}

func TestLoadByIDRejectsBadID(t *testing.T) {
	for _, id := range []string{"", "../moppy", "Moppy", "a b"} {
		if _, err := LoadByID("../../sites", id); err == nil {
			t.Errorf("LoadByID(%q) がエラーにならない", id)
		}
	}
}

func TestMoppyRenderURL(t *testing.T) {
	d, err := LoadByID(filepath.Join("..", "..", "sites"), "moppy")
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.RenderURL(map[string]string{"parent_category": "3", "child_category": "43"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://pc.moppy.jp/ajax/category/get_list.php?parent_category=3&child_category=43&objective_category=0&current_page=2&af_sorter=1&exclude_purchased="
	if got != want {
		t.Errorf("RenderURL = %q, want %q", got, want)
	}

	// child_category が無いリンク（親のみ）は param_defaults で 0 になる
	got, err = d.RenderURL(map[string]string{"parent_category": "2"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "parent_category=2&child_category=0&") {
		t.Errorf("param_defaults が効いていない: %q", got)
	}

	// 解決できないプレースホルダはエラー
	if _, err := d.RenderURL(map[string]string{}, 1); err == nil {
		t.Error("parent_category 未指定でエラーにならない")
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	bad := `
id: Bad ID
name: ""
base_url: not-a-url
request_headers:
  user-agent: x
discovery:
  url: ""
  link_selector: "a[["
listing:
  url_template: "/list?x=1"
  max_pages: 0
  item_selector: ""
  name_selector: ".n"
  reward_selector: ".r"
  link_selector: "a"
  pagination_selector: ".p a"
  pagination_attr: ""
url:
  external_id_regex: "id=(\\d+)-(\\d+)"
`
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("不正な定義がエラーにならない")
	}
	for _, want := range []string{
		"id は", "name が空", "base_url が不正", "User-Agent", "discovery.url が空",
		"discovery.link_selector が不正", "{page}", "max_pages", "item_selector が空",
		"pagination_attr が空", "キャプチャグループ",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("エラーに %q が含まれない: %v", want, err)
		}
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.yaml")
	src, err := os.ReadFile(filepath.Join("..", "..", "sites", "moppy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(src, []byte("\ninterval_seconds: 1\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "interval_seconds") {
		t.Errorf("未知のキーがエラーにならない: %v", err)
	}
}
