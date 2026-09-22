// Command crawler はポイントサイトの案件を日次で収集し、Supabase に保存する。
//
//	go run ./cmd/crawler -site moppy            # 本番（SUPABASE_URL / SUPABASE_SECRET_KEY が必要）
//	go run ./cmd/crawler -site moppy -dry-run   # 取得と抽出だけ。DB には書かない
//
// 終了コード：0 = success / partial、1 = failed / aborted / 起動失敗。
// 間隔・停止条件・User-Agent は internal/policy の定数のみを参照する（AGENTS.md）。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/crawl"
	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/fetch"
	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/site"
	"github.com/Masaya-Matsushita/poitoku-hikaku/crawler/internal/supabase"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("crawler", flag.ContinueOnError)
	fs.SetOutput(stderr)
	siteID := fs.String("site", "", "サイト ID（sites/<id>.yaml）")
	sitesDir := fs.String("sites-dir", "sites", "サイト定義のディレクトリ")
	dryRun := fs.Bool("dry-run", false, "DB に書き込まず、取得と抽出だけ行う")
	version := fs.String("version", defaultVersion(getenv), "crawl_logs.crawler_version に記録する識別子")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *siteID == "" {
		fmt.Fprintln(stderr, "crawler: -site を指定してください")
		return 1
	}

	logger := log.New(stderr, "", log.LstdFlags)
	def, err := site.LoadByID(*sitesDir, *siteID)
	if err != nil {
		logger.Print(err)
		return 1
	}

	var store crawl.Store
	if *dryRun {
		store = crawl.DryRunStore{Logf: logger.Printf}
	} else {
		store, err = supabase.New(getenv("SUPABASE_URL"), getenv("SUPABASE_SECRET_KEY"))
		if err != nil {
			logger.Print(err)
			return 1
		}
	}

	logger.Printf("start site=%s dry_run=%v version=%s", def.ID, *dryRun, *version)
	res, err := crawl.Run(ctx, def, fetch.New(def.RequestHeaders), store, crawl.Options{
		Version: *version,
		Logf:    logger.Printf,
	})
	if err != nil {
		logger.Print(err)
		if res == nil {
			return 1
		}
	}

	out, _ := json.MarshalIndent(res.Summary, "", "  ")
	fmt.Fprintln(stdout, string(out))
	if *dryRun {
		printSample(stdout, res.Offers)
	}

	switch res.Summary.Status {
	case "success", "partial":
		if err != nil {
			return 1
		}
		return 0
	default:
		return 1
	}
}

// defaultVersion は GitHub Actions なら git SHA の先頭 7 桁、それ以外は dev。
func defaultVersion(getenv func(string) string) string {
	if sha := getenv("GITHUB_SHA"); len(sha) >= 7 {
		return sha[:7]
	}
	return "dev"
}

// printSample は dry-run 時に先頭数件と、還元額を数値化できなかった案件を表示する
// （保存内容の目視確認と、セレクタ・数値化ルールの修復のため）。
func printSample(w io.Writer, offers []crawl.Offer) {
	crawl.SortOffers(offers)
	n := min(5, len(offers))
	fmt.Fprintf(w, "--- sample %d/%d ---\n", n, len(offers))
	for _, o := range offers[:n] {
		b, _ := json.Marshal(o)
		fmt.Fprintln(w, string(b))
	}

	var unparsed []crawl.Offer
	for _, o := range offers {
		if o.RewardPoints == nil && o.RewardPercent == nil {
			unparsed = append(unparsed, o)
		}
	}
	m := min(20, len(unparsed))
	fmt.Fprintf(w, "--- unparsed reward %d/%d (showing %d) ---\n", len(unparsed), len(offers), m)
	for _, o := range unparsed[:m] {
		fmt.Fprintf(w, "%q\t%s\t%s\n", o.RewardRaw, o.URL, o.Name)
	}
}
