// Package site はサイトごとのクロール定義（crawler/sites/<id>.yaml）を読み込み、検証する。
//
// 抽出は CSS セレクタで行い、その定義は YAML に分離する（ADR-0003）。夜間 Routine が
// セレクタを修復する時に触るのはこの YAML だけで、Go コードは変えない前提。
// リクエスト間隔・停止条件・User-Agent は定義に含めない（crawler/internal/policy が持つ）。
package site

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andybalholm/cascadia"
	"gopkg.in/yaml.v3"
)

// Definition は 1 サイト分の定義。
type Definition struct {
	// ID は sites テーブルの id と一致させる（moppy, hapitas ...）。
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	// BaseURL は相対 URL の解決とロボット排除（robots.txt）の取得に使う。
	BaseURL string `yaml:"base_url"`
	// RequestHeaders は全リクエストに付ける追加ヘッダ。User-Agent は policy が付けるので指定しない。
	RequestHeaders map[string]string `yaml:"request_headers"`
	Discovery      Discovery         `yaml:"discovery"`
	Listing        Listing           `yaml:"listing"`
	URL            URLRules          `yaml:"url"`
}

// Discovery は一覧ページ（カテゴリ）の発見方法。メニューページからリンクを集める。
type Discovery struct {
	URL          string `yaml:"url"`
	LinkSelector string `yaml:"link_selector"`
	// LabelAttr はカテゴリ名を持つ属性。空ならリンクのテキストを使う。
	LabelAttr       string `yaml:"label_attr"`
	LabelTrimPrefix string `yaml:"label_trim_prefix"`
	// ParamPattern は href に対する正規表現。名前付きグループ（(?P<slug>...)）をテンプレートの
	// パラメータにする。パスにカテゴリが入るサイト用。空ならクエリパラメータをそのまま使う。
	ParamPattern string `yaml:"param_pattern"`
}

// Listing は一覧ページの取得と抽出方法。
type Listing struct {
	// URLTemplate は発見したリンクのパラメータ名（{parent_category}、{slug} 等）と {page} を置換する。
	// {page} を含まなければ 1 ページだけ取る。
	URLTemplate string `yaml:"url_template"`
	// URLTemplates は同じカテゴリを複数の一覧（並び順違い等）で取る時に使う。url_template と併用不可。
	URLTemplates  []string          `yaml:"url_templates"`
	ParamDefaults map[string]string `yaml:"param_defaults"`
	// MaxPages は 1 カテゴリ・1 テンプレートあたりのページ数の上限（暴走防止）。
	MaxPages       int    `yaml:"max_pages"`
	ItemSelector   string `yaml:"item_selector"`
	NameSelector   string `yaml:"name_selector"`
	RewardSelector string `yaml:"reward_selector"`
	// RewardFallbackSelector は reward_selector で文字列が取れない時に見る要素（「ポイント対象外」等）。任意。
	RewardFallbackSelector string `yaml:"reward_fallback_selector"`
	// RewardWhenEmpty は還元額の要素が無い案件に入れる文言（サイトが還元 0 の案件で要素を出さない場合）。
	// 空なら還元額は空のまま（抽出失敗として数える）。任意。
	RewardWhenEmpty    string `yaml:"reward_when_empty"`
	LinkSelector       string `yaml:"link_selector"`
	PaginationSelector string `yaml:"pagination_selector"`
	PaginationAttr     string `yaml:"pagination_attr"`
	// TotalSelector はカテゴリの総件数を持つ要素（任意）。取得件数と比べて表示上限による取りこぼしを記録する。
	TotalSelector string `yaml:"total_selector"`
	// TotalAttr は総件数を持つ属性。空なら要素のテキスト。
	TotalAttr string `yaml:"total_attr"`
}

// Templates は一覧 URL のテンプレートを返す（url_template か url_templates のどちらか）。
func (l Listing) Templates() []string {
	if l.URLTemplate != "" {
		return []string{l.URLTemplate}
	}
	return l.URLTemplates
}

// URLRules は詳細 URL の正規化と、サイト側 ID の抽出方法。
type URLRules struct {
	// KeepParams に列挙したクエリパラメータだけ残す（追跡用パラメータを落とす）。空なら全部残す。
	KeepParams []string `yaml:"keep_params"`
	// RenameParams は同じ意味の別名を寄せる（s_id → site_id 等）。KeepParams より先に適用する。
	RenameParams map[string]string `yaml:"rename_params"`
	// PathPattern はパスに対する正規表現。一致したら PathTemplate（無ければ最初のグループ）をパスにする
	// （/item/detail/itemid/1/apn/xxx → /item/detail/itemid/1/ のような追跡用の末尾を落とす）。任意。
	PathPattern string `yaml:"path_pattern"`
	// PathTemplate は PathPattern の一致から作る正規のパス。$1 等でグループを参照する。
	// 同じ案件が別のパス（リダイレクト用 URL 等）で現れる場合に 1 つの URL に寄せる。任意。
	PathTemplate    string `yaml:"path_template"`
	ExternalIDRegex string `yaml:"external_id_regex"`
}

var (
	idPattern          = regexp.MustCompile(`^[a-z0-9_-]+$`)
	placeholderPattern = regexp.MustCompile(`\{([a-z_]+)\}`)
)

// Load は YAML ファイルを読み込んで検証する。
func Load(path string) (*Definition, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("site: %w", err)
	}
	var d Definition
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("site: %s の解析に失敗: %w", path, err)
	}
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("site: %s: %w", path, err)
	}
	return &d, nil
}

// LoadByID は <dir>/<id>.yaml を読み込む。
func LoadByID(dir, id string) (*Definition, error) {
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("site: 不正なサイト ID %q", id)
	}
	return Load(filepath.Join(dir, id+".yaml"))
}

// Validate は必須項目とセレクタ・正規表現の妥当性を確認する。
func (d *Definition) Validate() error {
	var errs []error
	if !idPattern.MatchString(d.ID) {
		errs = append(errs, fmt.Errorf("id は英小文字・数字・-_ のみ: %q", d.ID))
	}
	if d.Name == "" {
		errs = append(errs, errors.New("name が空"))
	}
	if u, err := url.Parse(d.BaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		errs = append(errs, fmt.Errorf("base_url が不正: %q", d.BaseURL))
	}
	if strings.EqualFold(headerKey(d.RequestHeaders, "User-Agent"), "User-Agent") {
		errs = append(errs, errors.New("request_headers に User-Agent は書けない（policy が付ける）"))
	}

	if d.Discovery.URL == "" {
		errs = append(errs, errors.New("discovery.url が空"))
	}
	errs = append(errs, checkSelector("discovery.link_selector", d.Discovery.LinkSelector, true)...)
	if d.Discovery.ParamPattern != "" {
		if re, err := regexp.Compile(d.Discovery.ParamPattern); err != nil {
			errs = append(errs, fmt.Errorf("discovery.param_pattern が不正: %w", err))
		} else if !hasNamedGroup(re) {
			errs = append(errs, errors.New("discovery.param_pattern は名前付きグループ（(?P<name>...)）を 1 つ以上持つ"))
		}
	}

	l := d.Listing
	switch {
	case l.URLTemplate != "" && len(l.URLTemplates) > 0:
		errs = append(errs, errors.New("listing.url_template と listing.url_templates は併用できない"))
	case l.URLTemplate == "" && len(l.URLTemplates) == 0:
		errs = append(errs, errors.New("listing.url_template か listing.url_templates が必要"))
	}
	for i, t := range l.URLTemplates {
		if t == "" {
			errs = append(errs, fmt.Errorf("listing.url_templates[%d] が空", i))
		}
	}
	errs = append(errs, checkSelector("listing.total_selector", l.TotalSelector, false)...)
	if l.MaxPages <= 0 {
		errs = append(errs, fmt.Errorf("listing.max_pages は 1 以上: %d", l.MaxPages))
	}
	errs = append(errs, checkSelector("listing.item_selector", l.ItemSelector, true)...)
	errs = append(errs, checkSelector("listing.name_selector", l.NameSelector, true)...)
	errs = append(errs, checkSelector("listing.reward_selector", l.RewardSelector, true)...)
	errs = append(errs, checkSelector("listing.reward_fallback_selector", l.RewardFallbackSelector, false)...)
	errs = append(errs, checkSelector("listing.link_selector", l.LinkSelector, true)...)
	errs = append(errs, checkSelector("listing.pagination_selector", l.PaginationSelector, false)...)
	if l.PaginationSelector != "" && l.PaginationAttr == "" {
		errs = append(errs, errors.New("listing.pagination_attr が空"))
	}

	if d.URL.PathPattern != "" {
		if re, err := regexp.Compile(d.URL.PathPattern); err != nil {
			errs = append(errs, fmt.Errorf("url.path_pattern が不正: %w", err))
		} else if re.NumSubexp() < 1 {
			errs = append(errs, errors.New("url.path_pattern はキャプチャグループを 1 つ以上持つ"))
		}
	}
	if d.URL.PathTemplate != "" && d.URL.PathPattern == "" {
		errs = append(errs, errors.New("url.path_template には url.path_pattern が必要"))
	}
	if d.URL.ExternalIDRegex == "" {
		errs = append(errs, errors.New("url.external_id_regex が空"))
	} else if re, err := regexp.Compile(d.URL.ExternalIDRegex); err != nil {
		errs = append(errs, fmt.Errorf("url.external_id_regex が不正: %w", err))
	} else if re.NumSubexp() != 1 {
		errs = append(errs, errors.New("url.external_id_regex はキャプチャグループを 1 つ持つ"))
	}
	return errors.Join(errs...)
}

func hasNamedGroup(re *regexp.Regexp) bool {
	for _, name := range re.SubexpNames() {
		if name != "" {
			return true
		}
	}
	return false
}

// ParamPattern はコンパイル済みの discovery.param_pattern を返す。未設定なら nil。Validate 済みの前提。
func (d *Definition) ParamPattern() *regexp.Regexp {
	if d.Discovery.ParamPattern == "" {
		return nil
	}
	return regexp.MustCompile(d.Discovery.ParamPattern)
}

// PathPattern はコンパイル済みの url.path_pattern を返す。未設定なら nil。Validate 済みの前提。
func (d *Definition) PathPattern() *regexp.Regexp {
	if d.URL.PathPattern == "" {
		return nil
	}
	return regexp.MustCompile(d.URL.PathPattern)
}

func checkSelector(name, sel string, required bool) []error {
	if sel == "" {
		if required {
			return []error{fmt.Errorf("%s が空", name)}
		}
		return nil
	}
	if _, err := cascadia.Parse(sel); err != nil {
		return []error{fmt.Errorf("%s が不正: %w", name, err)}
	}
	return nil
}

func headerKey(h map[string]string, name string) string {
	for k := range h {
		if strings.EqualFold(k, name) {
			return k
		}
	}
	return ""
}

// Base は base_url を *url.URL で返す。Validate 済みの前提。
func (d *Definition) Base() *url.URL {
	u, _ := url.Parse(d.BaseURL)
	return u
}

// Resolve は base_url を基準に相対 URL を絶対 URL にする。
func (d *Definition) Resolve(ref string) (string, error) {
	u, err := d.Base().Parse(ref)
	if err != nil {
		return "", fmt.Errorf("site: URL %q を解決できない: %w", ref, err)
	}
	return u.String(), nil
}

// IsPaged はテンプレートが {page} を含む（ページ送りする）かを返す。
func IsPaged(tmpl string) bool {
	return strings.Contains(tmpl, "{page}")
}

// RenderURL はテンプレート tmpl のプレースホルダを params（無ければ param_defaults）と page で置換し、
// base_url を基準に絶対 URL を返す。未解決のプレースホルダが残ればエラー。
func (d *Definition) RenderURL(tmpl string, params map[string]string, page int) (string, error) {
	var missing []string
	out := placeholderPattern.ReplaceAllStringFunc(tmpl, func(m string) string {
		name := m[1 : len(m)-1]
		if name == "page" {
			return fmt.Sprintf("%d", page)
		}
		if v, ok := params[name]; ok {
			return url.QueryEscape(v)
		}
		if v, ok := d.Listing.ParamDefaults[name]; ok {
			return url.QueryEscape(v)
		}
		missing = append(missing, name)
		return m
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("site: url_template のプレースホルダが未解決: %v", missing)
	}
	return d.Resolve(out)
}

// ExternalIDPattern はコンパイル済みの external_id_regex を返す。Validate 済みの前提。
func (d *Definition) ExternalIDPattern() *regexp.Regexp {
	return regexp.MustCompile(d.URL.ExternalIDRegex)
}
