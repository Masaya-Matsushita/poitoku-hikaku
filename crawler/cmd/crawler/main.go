// Command crawler はポイントサイトの案件を日次で収集する。
// 本体（サイト定義の読み込み、HTTP 取得、抽出、Supabase への保存）は次の PR で実装する。
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/policy"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "crawler:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Fprintf(out, "poitoku-hikaku crawler (雛形)\n")
	fmt.Fprintf(out, "  user-agent:       %s\n", policy.UserAgent)
	fmt.Fprintf(out, "  request interval: %s\n", policy.MinRequestInterval)
	fmt.Fprintf(out, "クロール本体は未実装です。\n")
	return nil
}
