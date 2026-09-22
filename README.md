# ポイ得比較（Poitoku-Hikaku）

ポイントサイトの案件を横断比較し、価格履歴と買い時判定を提供するWebサービス。
AI主導で開発・運用する実験プロジェクト。

## ドキュメント

| ファイル | 内容 |
|---|---|
| `AGENTS.md` | AI向けの規約と参照順序（CLAUDE.md・.cursorrules はここへの参照）|
| `docs/00-vision.md` | 目的・成功の定義・非目標 |
| `docs/01-strategy.md` | データ・集客・収益戦略 |
| `docs/02-kpi.md` | KPIと計測 |
| `docs/03-guardrails.md` | ガードレール |
| `docs/04-roadmap.md` | ロードマップ |
| `docs/05-routines.md` | 夜間Routine設計 |
| `docs/06-legal.md` | 法務チェックリスト |
| `docs/lessons.md` | 学び（想定と違ったこと）。第二弾以降の選定に使う |
| `docs/adr/` | 意思決定記録 |
| `docs/reference/` | 参照資料（2026-02、要再検証） |
| `logs/` | 会議記録・事故記録 |
| `reports/` | 自動生成レポート |

## ディレクトリ構成

```
crawler/              Go クローラー（go.mod はここ）
  cmd/crawler/        エントリポイント
  internal/policy/    加害防止の固定値（3秒間隔・429/403 停止・UA）とそのテスト
  sites/*.yaml        サイトごとのセレクタ定義（次の PR で追加）
web/                  React + Vite + TypeScript（静的生成。現在は仮ページ）
supabase/
  migrations/         スキーマ。適用手順は supabase/README.md
.github/workflows/
  ci.yml              PR と main：go test / vitest / lint。migrations 変更時は Supabase の dry-run を PR コメントに
  deploy.yml          main への push：supabase db push → Firebase Hosting へデプロイ
  （daily-crawl / weekly-backup は今後追加）
.github/scripts/      ワークフローから呼ぶスクリプト（dry-run のコメント生成、破壊的 SQL の検出）
firebase.json         Hosting 設定（公開ディレクトリは web/dist）
.tool-versions        Go / Node のバージョン固定（asdf）。CI もこれを読む
```

## 開発環境

```sh
asdf plugin add golang && asdf plugin add nodejs
asdf install                       # .tool-versions の Go / Node を入れる

cd crawler && go test ./...        # クローラー
cd web && npm ci && npm run lint && npm run typecheck && npm test && npm run build
```

## 本番環境

| サービス | 識別子 | 備考 |
|---|---|---|
| Firebase Hosting | プロジェクト `poitoku-hikaku` | https://poitoku-hikaku.web.app |
| Supabase | プロジェクト `tbvzseiehzuobglwbedo` | マイグレーションは main マージ時に `deploy.yml` が適用（`supabase/README.md`） |
| GitHub Actions | このリポジトリ | Secrets 名は `AGENTS.md` |
