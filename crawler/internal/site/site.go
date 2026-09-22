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
}

// Listing は一覧ページの取得と抽出方法。
type Listing struct {
	// URLTemplate は {page} と、発見したリンクのクエリパラメータ名（{parent_category} 等）を置換する。
	URLTemplate   string            `yaml:"url_template"`
	ParamDefaults map[string]string `yaml:"param_defaults"`
	// MaxPages は 1 カテゴリあたりのページ数の上限（暴走防止）。
	MaxPages           int    `yaml:"max_pages"`
	ItemSelector       string `yaml:"item_selector"`
	NameSelector       string `yaml:"name_selector"`
	RewardSelector     string `yaml:"reward_selector"`
	LinkSelector       string `yaml:"link_selector"`
	PaginationSelector string `yaml:"pagination_selector"`
	PaginationAttr     string `yaml:"pagination_attr"`
}

// URLRules は詳細 URL の正規化と、サイト側 ID の抽出方法。
type URLRules struct {
	// KeepParams に列挙したクエリパラメータだけ残す（追跡用パラメータを落とす）。空なら全部残す。
	KeepParams []string `yaml:"keep_params"`
	// RenameParams は同じ意味の別名を寄せる（s_id → site_id 等）。KeepParams より先に適用する。
	RenameParams    map[string]string `yaml:"rename_params"`
	ExternalIDRegex string            `yaml:"external_id_regex"`
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

	l := d.Listing
	if !strings.Contains(l.URLTemplate, "{page}") {
		errs = append(errs, errors.New("listing.url_template に {page} が無い"))
	}
	if l.MaxPages <= 0 {
		errs = append(errs, fmt.Errorf("listing.max_pages は 1 以上: %d", l.MaxPages))
	}
	errs = append(errs, checkSelector("listing.item_selector", l.ItemSelector, true)...)
	errs = append(errs, checkSelector("listing.name_selector", l.NameSelector, true)...)
	errs = append(errs, checkSelector("listing.reward_selector", l.RewardSelector, true)...)
	errs = append(errs, checkSelector("listing.link_selector", l.LinkSelector, true)...)
	errs = append(errs, checkSelector("listing.pagination_selector", l.PaginationSelector, false)...)
	if l.PaginationSelector != "" && l.PaginationAttr == "" {
		errs = append(errs, errors.New("listing.pagination_attr が空"))
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

// RenderURL は url_template のプレースホルダを params（無ければ param_defaults）と page で置換し、
// base_url を基準に絶対 URL を返す。未解決のプレースホルダが残ればエラー。
func (d *Definition) RenderURL(params map[string]string, page int) (string, error) {
	var missing []string
	out := placeholderPattern.ReplaceAllStringFunc(d.Listing.URLTemplate, func(m string) string {
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
