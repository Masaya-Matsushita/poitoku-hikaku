// Package robots は robots.txt を解釈し、パスの取得可否を判定する（docs/03-guardrails.md「robots.txt 遵守」）。
//
// 対応する範囲は Google の仕様（RFC 9309）の主要部分：User-agent によるグループ（同じ UA に
// 当たるグループが複数あれば規則を結合する）、Allow / Disallow、
// ワイルドカード *、末尾一致 $、最長一致優先（同長なら Allow）。Crawl-delay は policy の
// 固定間隔（3 秒）より短くしない前提で無視する。
package robots

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

type rule struct {
	allow   bool
	pattern string
	re      *regexp.Regexp
}

type group struct {
	agents []string
	rules  []rule
}

// Rules は解釈済みの robots.txt。
type Rules struct {
	groups []group
}

// Parse は robots.txt を解釈する。壊れた行は無視する。
func Parse(body []byte) *Rules {
	r := &Rules{}
	var cur *group
	lastWasAgent := false
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		field = strings.ToLower(strings.TrimSpace(field))
		value = strings.TrimSpace(value)
		switch field {
		case "user-agent":
			if cur == nil || !lastWasAgent {
				r.groups = append(r.groups, group{})
				cur = &r.groups[len(r.groups)-1]
			}
			cur.agents = append(cur.agents, strings.ToLower(value))
			lastWasAgent = true
		case "allow", "disallow":
			lastWasAgent = false
			if cur == nil || value == "" {
				// グループ外の規則と空の Disallow（= 全部許可）は無視
				continue
			}
			cur.rules = append(cur.rules, rule{allow: field == "allow", pattern: value, re: compile(value)})
		default:
			lastWasAgent = false
		}
	}
	return r
}

// compile は robots のパターン（* と $ のみ特別）を正規表現にする。
func compile(pattern string) *regexp.Regexp {
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = strings.TrimSuffix(pattern, "$")
	}
	parts := strings.Split(pattern, "*")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	expr := "^" + strings.Join(parts, ".*")
	if anchored {
		expr += "$"
	}
	return regexp.MustCompile(expr)
}

// Allowed は userAgent（"poitoku-hikaku/1.0 (...)" のような完全な UA 文字列）が
// path（クエリを含んでよい）を取得してよいかを返す。該当グループが無ければ許可。
func (r *Rules) Allowed(userAgent, path string) bool {
	if r == nil {
		return true
	}
	rules := r.rulesFor(userAgent)
	if path == "" {
		path = "/"
	}
	var best *rule
	for i := range rules {
		rl := &rules[i]
		if !rl.re.MatchString(path) {
			continue
		}
		if best == nil || len(rl.pattern) > len(best.pattern) || (len(rl.pattern) == len(best.pattern) && rl.allow) {
			best = rl
		}
	}
	if best == nil {
		return true
	}
	return best.allow
}

// rulesFor は UA の製品トークン（"/" より前）に一致するグループ、無ければ * のグループの規則を返す。
// 同じ UA に当たるグループが複数あれば（User-agent: * が 2 回書かれている等）規則を結合する
// （RFC 9309 2.2.1）。最初のグループだけを見ると後ろのグループの Disallow を取りこぼす
// （ちょびリッチの robots.txt は 1 つ目の * グループが Allow: /ads.txt だけで、Disallow は 2 つ目にある）。
func (r *Rules) rulesFor(userAgent string) []rule {
	token := strings.ToLower(strings.TrimSpace(userAgent))
	if i := strings.IndexAny(token, "/ "); i >= 0 {
		token = token[:i]
	}
	var specific, wildcard []rule
	matched := false
	for _, g := range r.groups {
		isSpecific, isWildcard := false, false
		for _, a := range g.agents {
			switch {
			case a == "*":
				isWildcard = true
			case token != "" && (a == token || strings.HasPrefix(a, token)):
				isSpecific = true
			}
		}
		if isSpecific {
			matched = true
			specific = append(specific, g.rules...)
		}
		if isWildcard {
			wildcard = append(wildcard, g.rules...)
		}
	}
	if matched {
		return specific
	}
	return wildcard
}
