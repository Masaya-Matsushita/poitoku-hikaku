// Command report は crawl_logs から日次の KPI レポート（Markdown）を生成する。
//
//	go run ./cmd/report -date 2026-09-24 -out ../reports/2026-09-24.md
//
// -date を省略すると JST の今日。SUPABASE_URL / SUPABASE_SECRET_KEY が必要（crawl_logs は非公開のため）。
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/report"
	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/supabase"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	date := fs.String("date", time.Now().In(report.JST).Format("2006-01-02"), "対象日（YYYY-MM-DD、JST）")
	out := fs.String("out", "", "出力先ファイル。省略時は標準出力")
	window := fs.Int("window", 30, "クロール成功率の長い方の窓（日）")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	src, err := supabase.New(getenv("SUPABASE_URL"), getenv("SUPABASE_SECRET_KEY"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	r, err := report.Build(ctx, src, *date, time.Now(), *window)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	md := report.Render(r)

	if *out == "" {
		fmt.Fprint(stdout, md)
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.WriteFile(*out, []byte(md), 0o644); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "%s を書きました（要確認 %d 件）\n", *out, len(r.Warnings))
	return 0
}
