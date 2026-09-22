package extract

import (
	"regexp"
	"strconv"
	"strings"
)

// Reward は還元額の表示文字列から取り出した数値。固定額なら Points、率なら Percent。
// どちらも取れなければ両方 nil（reward_raw だけ保存し、抽出失敗として KPI に数える）。
type Reward struct {
	Points  *int64
	Percent *float64
}

var (
	percentPattern = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*%`)
	pointsPattern  = regexp.MustCompile(`(?i)([0-9][0-9,]*)\s*(?:P|pt|ポイント)`)
)

// noRewardMarkers は「還元なし」を表す文言。含まれていれば固定額 0 として扱う
// （抽出失敗ではなく、還元が無いことが分かっている状態）。
var noRewardMarkers = []string{"対象外"}

// ParseReward は "14,000P" / "1.0%" / "最大10,000P" のような表示文字列を解釈する。
// 全角の数字・記号は半角に寄せてから見る。率と額の両方があれば率を優先しない（どちらか一方だけ返す）。
// 「ポイント対象外」のような還元なしの文言は Points = 0。
func ParseReward(raw string) Reward {
	s := normalize(raw)
	for _, marker := range noRewardMarkers {
		if strings.Contains(s, marker) {
			zero := int64(0)
			return Reward{Points: &zero}
		}
	}
	if m := percentPattern.FindStringSubmatch(s); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return Reward{Percent: &v}
		}
	}
	if m := pointsPattern.FindStringSubmatch(s); m != nil {
		if v, err := strconv.ParseInt(strings.ReplaceAll(m[1], ",", ""), 10, 64); err == nil {
			return Reward{Points: &v}
		}
	}
	return Reward{}
}

// normalize は全角英数字・カンマ・ピリオド・パーセントを半角にする。
func normalize(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= '０' && r <= '９':
			return '0' + (r - '０')
		case r >= 'Ａ' && r <= 'Ｚ':
			return 'A' + (r - 'Ａ')
		case r >= 'ａ' && r <= 'ｚ':
			return 'a' + (r - 'ａ')
		case r == '，':
			return ','
		case r == '．':
			return '.'
		case r == '％':
			return '%'
		case r == '　':
			return ' '
		}
		return r
	}, s)
}
