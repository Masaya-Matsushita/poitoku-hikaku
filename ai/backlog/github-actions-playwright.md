# GitHub Actions での Playwright セットアップ検証

**起票日**: 2026-02-11
**起票者**: エンジニア 藤原
**対応時期**: Phase 3（インフラ・自動化）

## 概要

Crawl4AI は Playwright に依存している可能性がある。GitHub Actions で日次クローリングを実行する際、Playwright の環境構築が必要になる可能性がある。

## 検討事項

- GitHub Actions の ubuntu ランナーで Playwright が動作するか確認
- 必要に応じて `playwright install-deps` ステップを追加
- ブラウザのキャッシュ設定（実行時間短縮）

## 参考

```yaml
# .github/workflows/crawl.yml の例
- name: Install Playwright
  run: |
    pip install playwright
    playwright install chromium
    playwright install-deps
```

## ステータス

📋 未着手
