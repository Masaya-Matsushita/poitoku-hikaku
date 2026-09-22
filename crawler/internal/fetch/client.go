// Package fetch は対象サイトへの HTTP 取得を、policy の制約（間隔・停止条件・User-Agent）を
// 必ず通す形で提供する。クローラー本体はこのパッケージ以外で HTTP を発行しない。
package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/policy"
)

const (
	requestTimeout = 30 * time.Second
	maxBodyBytes   = 8 << 20
)

// StopError はサーキットブレーカーが作動し、以後のリクエストを一切行わないことを表す。
// Reason は crawl_logs.abort_reason にそのまま入る（http_429 / http_403 / server_error_streak）。
type StopError struct {
	Reason     string
	StatusCode int
	URL        string
}

func (e *StopError) Error() string {
	return fmt.Sprintf("fetch: クロール停止（%s, HTTP %d, %s）", e.Reason, e.StatusCode, e.URL)
}

// HTTPError は停止条件ではない非 2xx 応答。呼び出し側はスキップして続行できる。
type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("fetch: HTTP %d（%s）", e.StatusCode, e.URL)
}

// RequestError は接続失敗・タイムアウトなど応答が得られなかった失敗。
type RequestError struct {
	URL string
	Err error
}

func (e *RequestError) Error() string { return fmt.Sprintf("fetch: %s: %v", e.URL, e.Err) }
func (e *RequestError) Unwrap() error { return e.Err }

// Client は 1 サイト向けの取得クライアント。同時に 1 リクエストしか行わず、
// リクエスト開始の間隔を policy.MinRequestInterval 以上に保つ。
type Client struct {
	httpClient *http.Client
	headers    map[string]string
	interval   time.Duration
	now        func() time.Time
	sleep      func(context.Context, time.Duration) error

	mu              sync.Mutex
	last            time.Time
	requests        int
	serverErrStreak int
	stopped         *StopError
}

// New はクライアントを作る。headers は全リクエストに付ける追加ヘッダ（User-Agent 以外）。
func New(headers map[string]string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: requestTimeout,
			// リダイレクトを自動で追うと間隔と回数の管理から漏れるため追わない
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		headers:  headers,
		interval: policy.MinRequestInterval,
		now:      time.Now,
		sleep:    sleepContext,
	}
}

// Get は URL を取得して本文を返す。2xx 以外は HTTPError、停止条件に達したら StopError。
// 一度 StopError を返した後は、リクエストを発行せずに同じ StopError を返し続ける。
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped != nil {
		return nil, c.stopped
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !c.last.IsZero() {
		if wait := c.interval - c.now().Sub(c.last); wait > 0 {
			if err := c.sleep(ctx, wait); err != nil {
				return nil, err
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &RequestError{URL: rawURL, Err: err}
	}
	req.Header.Set("User-Agent", policy.UserAgent)
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	c.last = c.now()
	c.requests++
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &RequestError{URL: rawURL, Err: err}
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

	code := resp.StatusCode
	if policy.ShouldStopImmediately(code) {
		c.stopped = &StopError{Reason: fmt.Sprintf("http_%d", code), StatusCode: code, URL: rawURL}
		return nil, c.stopped
	}
	if policy.IsServerError(code) {
		c.serverErrStreak++
		if c.serverErrStreak >= policy.MaxServerErrorStreak {
			c.stopped = &StopError{Reason: "server_error_streak", StatusCode: code, URL: rawURL}
			return nil, c.stopped
		}
		return nil, &HTTPError{StatusCode: code, URL: rawURL}
	}
	c.serverErrStreak = 0
	if code < 200 || code >= 300 {
		return nil, &HTTPError{StatusCode: code, URL: rawURL}
	}
	if readErr != nil {
		return nil, &RequestError{URL: rawURL, Err: readErr}
	}
	return body, nil
}

// Requests は発行したリクエスト数を返す（crawl_logs.request_count）。
func (c *Client) Requests() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests
}

// Stopped は停止していればその理由を返す。
func (c *Client) Stopped() *StopError {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopped
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
