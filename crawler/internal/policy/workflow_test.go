package policy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docs/03-guardrails.md：日次クロールは 1 日 1 回、GitHub Actions の cron のみから起動する。
// crawl.yml の schedule を読み、cron が 1 本だけで、分・時が固定され、日・月・曜日が * であることを確認する。
// 頻度を上げる変更（複数 cron、*/6 のような時間指定）はここで落ちる。
func TestCrawlWorkflowRunsOncePerDay(t *testing.T) {
	path := filepath.Join("..", "..", "..", ".github", "workflows", "crawl.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s を読めない: %v", path, err)
	}
	crons := regexp.MustCompile(`(?m)^\s*-\s*cron:\s*['"]?([^'"\n#]+?)['"]?\s*(#.*)?$`).FindAllStringSubmatch(string(b), -1)
	if len(crons) != 1 {
		t.Fatalf("crawl.yml の cron は 1 本だけ: %d 本", len(crons))
	}
	fields := strings.Fields(crons[0][1])
	if len(fields) != 5 {
		t.Fatalf("cron の書式が不正: %q", crons[0][1])
	}
	numeric := regexp.MustCompile(`^[0-9]+$`)
	if !numeric.MatchString(fields[0]) || !numeric.MatchString(fields[1]) {
		t.Errorf("分・時は固定値にする（1 日 1 回）: %q", crons[0][1])
	}
	for _, f := range fields[2:] {
		if f != "*" {
			t.Errorf("日・月・曜日は * にする（毎日）: %q", crons[0][1])
		}
	}
}
