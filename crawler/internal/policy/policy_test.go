package policy

import (
	"strings"
	"testing"
	"time"
)

// docs/03-guardrails.md：リクエスト間隔 3 秒以上。短くする変更はここで落ちる。
func TestMinRequestIntervalIsAtLeastThreeSeconds(t *testing.T) {
	const floor = 3 * time.Second
	if MinRequestInterval < floor {
		t.Fatalf("MinRequestInterval = %s, ガードレールの下限 %s を下回っている", MinRequestInterval, floor)
	}
}

// docs/03-guardrails.md：5xx 連続 3 回で停止。緩める（回数を増やす）変更はここで落ちる。
func TestMaxServerErrorStreakIsAtMostThree(t *testing.T) {
	if MaxServerErrorStreak < 1 || MaxServerErrorStreak > 3 {
		t.Fatalf("MaxServerErrorStreak = %d, 1〜3 の範囲外", MaxServerErrorStreak)
	}
}

// docs/03-guardrails.md：User-Agent に連絡先を明記する。
func TestUserAgentHasContact(t *testing.T) {
	if !strings.HasPrefix(UserAgent, "poitoku-hikaku/") {
		t.Errorf("UserAgent = %q, サービス名で始まっていない", UserAgent)
	}
	if !strings.Contains(UserAgent, "(+https://") {
		t.Errorf("UserAgent = %q, 連絡先 URL（+https://...）が含まれていない", UserAgent)
	}
}

func TestShouldStopImmediately(t *testing.T) {
	cases := []struct {
		status int
		want   bool
	}{
		{429, true},
		{403, true},
		{200, false},
		{301, false},
		{404, false},
		{500, false},
		{503, false},
	}
	for _, c := range cases {
		if got := ShouldStopImmediately(c.status); got != c.want {
			t.Errorf("ShouldStopImmediately(%d) = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestIsServerError(t *testing.T) {
	cases := []struct {
		status int
		want   bool
	}{
		{499, false},
		{500, true},
		{502, true},
		{599, true},
		{600, false},
		{200, false},
	}
	for _, c := range cases {
		if got := IsServerError(c.status); got != c.want {
			t.Errorf("IsServerError(%d) = %v, want %v", c.status, got, c.want)
		}
	}
}
