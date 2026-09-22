// Package policy は、対象サイトへの加害を防ぐための固定値を定義する。
//
// 値は docs/03-guardrails.md「クローラー（対象サイトへの加害防止）」に対応し、
// policy_test.go がその下限・上限を担保する。これらを緩める変更は CI で落ちる
// （AGENTS.md「絶対に守ること」）。クローラー本体はこのパッケージの値だけを参照し、
// 独自のリテラルで間隔や閾値を持たないこと。
package policy

import (
	"net/http"
	"time"
)

const (
	// MinRequestInterval は同一サイトへの連続リクエストの最小間隔。
	MinRequestInterval = 3 * time.Second

	// MaxServerErrorStreak は 5xx 応答がこの回数連続したらクロールを停止する閾値。
	MaxServerErrorStreak = 3

	// UserAgent は全リクエストに付ける User-Agent。連絡先を明記する。
	UserAgent = "poitoku-hikaku/1.0 (+https://poitoku-hikaku.com/about)"
)

// ShouldStopImmediately は、受信した時点でクロールを即時停止すべき
// HTTP ステータスコードかどうかを返す（429 / 403）。
func ShouldStopImmediately(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, http.StatusForbidden:
		return true
	default:
		return false
	}
}

// IsServerError は 5xx 応答かどうかを返す。MaxServerErrorStreak の連続カウントに使う。
func IsServerError(statusCode int) bool {
	return statusCode >= 500 && statusCode <= 599
}
