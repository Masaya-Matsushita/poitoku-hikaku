package crawl

import (
	"context"
	"time"
)

// DryRunStore は何も保存しない Store。取得と抽出だけ確かめる時に使う（-dry-run）。
type DryRunStore struct {
	Logf func(format string, args ...any)
}

func (d DryRunStore) logf(format string, args ...any) {
	if d.Logf != nil {
		d.Logf(format, args...)
	}
}

func (d DryRunStore) StartCrawlLog(_ context.Context, siteID, crawledOn string, _ time.Time, version string) (int64, error) {
	d.logf("dry-run: crawl_logs は書かない（site=%s crawled_on=%s version=%s）", siteID, crawledOn, version)
	return 0, nil
}

func (d DryRunStore) SaveOffers(_ context.Context, siteID, crawledOn string, offers []Offer) error {
	d.logf("dry-run: %s の %d 件を保存しない（crawled_on=%s）", siteID, len(offers), crawledOn)
	return nil
}

func (d DryRunStore) FinishCrawlLog(_ context.Context, _ int64, s Summary) error {
	d.logf("dry-run: 結果 status=%s requests=%d offers=%d parsed=%d errors=%d", s.Status, s.RequestCount, s.OfferCount, s.ParsedCount, s.ErrorCount)
	return nil
}
