// Package extract は取得した HTML から、サイト定義（site.Definition）の CSS セレクタで
// 案件・カテゴリリンク・ページ番号を取り出す。ネットワークには触れない。
package extract

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/site"
)

// Parse は HTML（断片でも可）を DOM にする。
func Parse(b []byte) (*html.Node, error) {
	doc, err := html.Parse(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("extract: HTML の解析に失敗: %w", err)
	}
	return doc, nil
}

// Item は一覧ページから取り出した 1 案件。URL は未正規化の href。
type Item struct {
	Name      string
	RewardRaw string
	Href      string
}

// Items は一覧ページから案件を取り出す。案件名か href が取れない要素は飛ばし、その数を skipped で返す。
func Items(doc *html.Node, l site.Listing) (items []Item, skipped int, err error) {
	itemSel, err := cascadia.Parse(l.ItemSelector)
	if err != nil {
		return nil, 0, fmt.Errorf("extract: item_selector: %w", err)
	}
	nameSel, err := cascadia.Parse(l.NameSelector)
	if err != nil {
		return nil, 0, fmt.Errorf("extract: name_selector: %w", err)
	}
	rewardSel, err := cascadia.Parse(l.RewardSelector)
	if err != nil {
		return nil, 0, fmt.Errorf("extract: reward_selector: %w", err)
	}
	linkSel, err := cascadia.Parse(l.LinkSelector)
	if err != nil {
		return nil, 0, fmt.Errorf("extract: link_selector: %w", err)
	}
	var fallbackSel cascadia.Sel
	if l.RewardFallbackSelector != "" {
		if fallbackSel, err = cascadia.Parse(l.RewardFallbackSelector); err != nil {
			return nil, 0, fmt.Errorf("extract: reward_fallback_selector: %w", err)
		}
	}

	for _, n := range cascadia.QueryAll(doc, itemSel) {
		name := Text(cascadia.Query(n, nameSel))
		href := Attr(linkOrSelf(n, linkSel), "href")
		if name == "" || href == "" {
			skipped++
			continue
		}
		reward := Text(cascadia.Query(n, rewardSel))
		if reward == "" && fallbackSel != nil {
			// 還元額の要素が無い案件（ポイント対象外など）は代替要素の文言を還元額として記録する
			reward = Text(cascadia.Query(n, fallbackSel))
		}
		if reward == "" {
			// サイトが還元 0 の案件で要素自体を出さない場合の既定文言（未設定なら空のまま）
			reward = l.RewardWhenEmpty
		}
		items = append(items, Item{
			Name:      name,
			RewardRaw: reward,
			Href:      href,
		})
	}
	return items, skipped, nil
}

// linkOrSelf は案件要素の中のリンクを返す。案件要素自身がリンクならそれを返す。
func linkOrSelf(n *html.Node, sel cascadia.Sel) *html.Node {
	if sel.Match(n) {
		return n
	}
	return cascadia.Query(n, sel)
}

// LastPage はページネーションから最大ページ番号を返す。要素が無ければ 1。
func LastPage(doc *html.Node, l site.Listing) (int, error) {
	if l.PaginationSelector == "" {
		return 1, nil
	}
	sel, err := cascadia.Parse(l.PaginationSelector)
	if err != nil {
		return 0, fmt.Errorf("extract: pagination_selector: %w", err)
	}
	last := 1
	for _, n := range cascadia.QueryAll(doc, sel) {
		v := Attr(n, l.PaginationAttr)
		if v == "" {
			v = Text(n)
		}
		if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && p > last {
			last = p
		}
	}
	return last, nil
}

// Link はメニューから見つけた一覧ページへのリンク。
type Link struct {
	Href  string
	Label string
}

// Links はメニューページから一覧ページへのリンクを集める。
func Links(doc *html.Node, d site.Discovery) ([]Link, error) {
	sel, err := cascadia.Parse(d.LinkSelector)
	if err != nil {
		return nil, fmt.Errorf("extract: discovery.link_selector: %w", err)
	}
	var links []Link
	for _, n := range cascadia.QueryAll(doc, sel) {
		href := Attr(n, "href")
		if href == "" {
			continue
		}
		label := ""
		if d.LabelAttr != "" {
			label = Attr(n, d.LabelAttr)
		}
		if label == "" {
			label = Text(n)
		}
		label = strings.TrimSpace(strings.TrimPrefix(label, d.LabelTrimPrefix))
		links = append(links, Link{Href: href, Label: label})
	}
	return links, nil
}

// QueryParams は href のクエリパラメータを map にする（url_template の置換用）。
func QueryParams(href string) (map[string]string, error) {
	u, err := url.Parse(href)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for k, vs := range u.Query() {
		if len(vs) > 0 {
			out[k] = vs[0]
		}
	}
	return out, nil
}

// ErrNoParamMatch は discovery.param_pattern が href に一致しなかったことを表す。
var ErrNoParamMatch = errors.New("extract: param_pattern に一致しない")

// Params は href からテンプレート用のパラメータを取る。クエリパラメータに加え、pattern（名前付き
// グループ）が与えられればその一致も入れる（同名ならこちらが優先）。pattern が一致しなければ ErrNoParamMatch。
func Params(href string, pattern *regexp.Regexp) (map[string]string, error) {
	params, err := QueryParams(href)
	if err != nil {
		return nil, err
	}
	if pattern == nil {
		return params, nil
	}
	m := pattern.FindStringSubmatch(href)
	if m == nil {
		return nil, ErrNoParamMatch
	}
	for i, name := range pattern.SubexpNames() {
		if name != "" && i < len(m) {
			params[name] = m[i]
		}
	}
	return params, nil
}

// Total は一覧ページのカテゴリ総件数を返す。total_selector が無い・読めない時は ok=false。
func Total(doc *html.Node, l site.Listing) (total int, ok bool) {
	if l.TotalSelector == "" {
		return 0, false
	}
	sel, err := cascadia.Parse(l.TotalSelector)
	if err != nil {
		return 0, false
	}
	n := cascadia.Query(doc, sel)
	if n == nil {
		return 0, false
	}
	v := Text(n)
	if l.TotalAttr != "" {
		v = Attr(n, l.TotalAttr)
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, v)
	if digits == "" {
		return 0, false
	}
	total, err = strconv.Atoi(digits)
	return total, err == nil
}

// Canonical は href を base で絶対 URL にし、url ルール（パスの末尾・別名の統一・残すパラメータ）で
// 正規化する。フラグメントは落とし、クエリはキー順に並べる。
func Canonical(href string, base *url.URL, r site.URLRules) (string, error) {
	u, err := base.Parse(strings.TrimSpace(href))
	if err != nil {
		return "", fmt.Errorf("extract: URL %q を解決できない: %w", href, err)
	}
	u.Fragment = ""
	if r.PathPattern != "" {
		if re, err := regexp.Compile(r.PathPattern); err == nil {
			if m := re.FindStringSubmatchIndex(u.Path); m != nil {
				if r.PathTemplate != "" {
					u.Path = string(re.ExpandString(nil, r.PathTemplate, u.Path, m))
				} else if len(m) >= 4 && m[2] >= 0 {
					u.Path = u.Path[m[2]:m[3]]
				}
				u.RawPath = ""
			}
		}
	}
	keep := map[string]bool{}
	for _, k := range r.KeepParams {
		keep[k] = true
	}
	out := url.Values{}
	for k, vs := range u.Query() {
		name := k
		if to, ok := r.RenameParams[k]; ok {
			name = to
		}
		if len(keep) > 0 && !keep[name] {
			continue
		}
		for _, v := range vs {
			out.Add(name, v)
		}
	}
	u.RawQuery = out.Encode()
	return u.String(), nil
}

// ExternalID は正規化済み URL からサイト側の案件 ID を取り出す。取れなければ空。
func ExternalID(canonical string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(canonical)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// Text は要素配下のテキストを連結し、空白を 1 つに潰して返す。nil なら空。
func Text(n *html.Node) string {
	if n == nil {
		return ""
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
			sb.WriteByte(' ')
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(sb.String()), " ")
}

// Attr は属性値を返す。無ければ空。
func Attr(n *html.Node, name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == name {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}
