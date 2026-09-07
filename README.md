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

## 予定ディレクトリ構成

```
crawler/          Go クローラー
  sites/*.yaml    サイトごとのセレクタ定義
web/              React + Vite（静的生成）
supabase/         マイグレーション
.github/workflows daily-crawl / ci / deploy / weekly-backup
```
