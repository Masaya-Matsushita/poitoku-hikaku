package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/policy"
)

func TestNewUsesPolicyInterval(t *testing.T) {
	c := New(nil)
	if c.interval != policy.MinRequestInterval {
		t.Fatalf("interval = %s, policy は %s", c.interval, policy.MinRequestInterval)
	}
}

// 間隔は本物の時計ではなく差し替えた now/sleep で検証する（テストを 3 秒待たせない）。
func TestGetKeepsMinimumInterval(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := New(map[string]string{"X-Requested-With": "XMLHttpRequest"})
	clock := time.Unix(1_700_000_000, 0)
	var slept []time.Duration
	c.now = func() time.Time { return clock }
	c.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		clock = clock.Add(d)
		return nil
	}

	for i := 0; i < 3; i++ {
		if _, err := c.Get(context.Background(), srv.URL+"/"); err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(500 * time.Millisecond) // 処理時間のつもり
	}
	if len(slept) != 2 {
		t.Fatalf("sleep 回数 = %d, want 2（初回は待たない）", len(slept))
	}
	for _, d := range slept {
		if d != policy.MinRequestInterval-500*time.Millisecond {
			t.Errorf("sleep = %s, want %s", d, policy.MinRequestInterval-500*time.Millisecond)
		}
	}
	if c.Requests() != 3 || atomic.LoadInt32(&hits) != 3 {
		t.Errorf("requests = %d, server hits = %d", c.Requests(), hits)
	}
}

func TestGetSetsUserAgentAndHeaders(t *testing.T) {
	var ua, xrw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		xrw = r.Header.Get("X-Requested-With")
		w.Write([]byte("<html></html>"))
	}))
	defer srv.Close()

	c := newFast(map[string]string{"X-Requested-With": "XMLHttpRequest"})
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "<html></html>" {
		t.Errorf("body = %q", body)
	}
	if ua != policy.UserAgent {
		t.Errorf("User-Agent = %q, want %q", ua, policy.UserAgent)
	}
	if xrw != "XMLHttpRequest" {
		t.Errorf("X-Requested-With = %q", xrw)
	}
}

func TestStopImmediatelyOn429AndNeverRequestAgain(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newFast(nil)
	_, err := c.Get(context.Background(), srv.URL)
	var stop *StopError
	if !errors.As(err, &stop) {
		t.Fatalf("StopError ではない: %v", err)
	}
	if stop.Reason != "http_429" || stop.StatusCode != 429 {
		t.Errorf("stop = %+v", stop)
	}
	for i := 0; i < 3; i++ {
		if _, err := c.Get(context.Background(), srv.URL); !errors.As(err, &stop) {
			t.Fatalf("停止後の Get が StopError を返さない: %v", err)
		}
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("停止後にもリクエストが飛んでいる: hits = %d", hits)
	}
	if c.Stopped() == nil || c.Requests() != 1 {
		t.Errorf("Stopped = %v, Requests = %d", c.Stopped(), c.Requests())
	}
}

func TestStopOn403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := newFast(nil).Get(context.Background(), srv.URL)
	var stop *StopError
	if !errors.As(err, &stop) || stop.Reason != "http_403" {
		t.Fatalf("403 で停止しない: %v", err)
	}
}

func TestServerErrorStreakStopsAtPolicyLimit(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := newFast(nil)
	var stop *StopError
	for i := 1; i <= policy.MaxServerErrorStreak; i++ {
		_, err := c.Get(context.Background(), srv.URL)
		if i < policy.MaxServerErrorStreak {
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != 502 {
				t.Fatalf("%d 回目: HTTPError ではない: %v", i, err)
			}
			continue
		}
		if !errors.As(err, &stop) || stop.Reason != "server_error_streak" {
			t.Fatalf("%d 回目で停止しない: %v", i, err)
		}
	}
	if int(atomic.LoadInt32(&hits)) != policy.MaxServerErrorStreak {
		t.Errorf("hits = %d, want %d", hits, policy.MaxServerErrorStreak)
	}
}

func TestSuccessResetsServerErrorStreak(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 500, 500, 200, 500, 500, 200 ... 連続 3 回にはならない
		if atomic.AddInt32(&n, 1)%3 == 0 {
			w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newFast(nil)
	for i := 0; i < 9; i++ {
		_, err := c.Get(context.Background(), srv.URL)
		var stop *StopError
		if errors.As(err, &stop) {
			t.Fatalf("%d 回目で誤って停止: %v", i+1, err)
		}
	}
}

func TestNon2xxIsHTTPErrorAndRedirectsAreNotFollowed(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/target", http.StatusFound)
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()

	c := newFast(nil)
	_, err := c.Get(context.Background(), srv.URL+"/missing")
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != 404 {
		t.Fatalf("404 が HTTPError にならない: %v", err)
	}
	_, err = c.Get(context.Background(), srv.URL+"/redirect")
	if !errors.As(err, &httpErr) || httpErr.StatusCode != 302 {
		t.Fatalf("リダイレクトが HTTPError(302) にならない: %v", err)
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Errorf("リダイレクト先まで取得している: hits = %d", hits)
	}
}

func TestCanceledContextStopsBeforeRequest(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newFast(nil).Get(ctx, srv.URL); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Error("キャンセル済みなのにリクエストした")
	}
}

// newFast はテスト用に待ち時間を潰したクライアント。本番の間隔は TestNewUsesPolicyInterval で担保する。
func newFast(headers map[string]string) *Client {
	c := New(headers)
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return c
}
