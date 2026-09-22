// Package robots は robots.txt を解釈し、パスの取得可否を判定する（docs/03-guardrails.md「robots.txt 遵守」）。
//
// 対応する範囲は Google の仕様の主要部分：User-agent によるグループ、Allow / Disallow、
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
	g := r.groupFor(userAgent)
	if g == nil {
		return true
	}
	if path == "" {
		path = "/"
	}
	var best *rule
	for i := range g.rules {
		rl := &g.rules[i]
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

// groupFor は UA の製品トークン（"/" より前）に一致するグループ、無ければ * のグループを返す。
func (r *Rules) groupFor(userAgent string) *group {
	token := strings.ToLower(strings.TrimSpace(userAgent))
	if i := strings.IndexAny(token, "/ "); i >= 0 {
		token = token[:i]
	}
	var wildcard *group
	for i := range r.groups {
		g := &r.groups[i]
		for _, a := range g.agents {
			if a == "*" {
				if wildcard == nil {
					wildcard = g
				}
				continue
			}
			if token != "" && (a == token || strings.HasPrefix(a, token)) {
				return g
			}
		}
	}
	return wildcard
}
