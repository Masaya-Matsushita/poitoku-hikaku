# supabase/

Supabase（PostgreSQL）のスキーマを `migrations/` に置く。プロジェクト ID は `tbvzseiehzuobglwbedo`。

## テーブル

| テーブル | 役割 | 書き込み |
|---|---|---|
| `sites` | 対象ポイントサイトのマスタ。ポイント単価を持つ | マイグレーション（初期データ） |
| `offers` | サイトごとの案件。1 案件 1 行。`unique (site_id, url)` | クローラー（upsert） |
| `offer_snapshots` | 日次の還元額履歴。1 案件 × 1 日 = 1 行 | クローラー（insert） |
| `crawl_logs` | クロール実行ログ。KPI（成功率・抽出精度）の一次データ | クローラー |
| `current_offers`（view） | 案件ごとの最新還元額と円換算 | — |

読み取りは `sites` / `offers` / `offer_snapshots` を anon（publishable key）に公開する。
`crawl_logs` は公開しない。書き込みポリシーは作らず、secret key（RLS を通らない）を持つクローラーだけが書く。

設計の意図はマイグレーションファイル冒頭のコメントに書いてある。

## 適用方法（オーナーが行う）

マイグレーションの本番適用は CI では行わない（`docs/03-guardrails.md`「破壊的マイグレーション」、
および DB 接続情報を GitHub Secrets に置いていないため）。以下のいずれかで手動適用する。

### A. Dashboard の SQL Editor（最も簡単）

1. https://supabase.com/dashboard/project/tbvzseiehzuobglwbedo/sql/new を開く
2. `migrations/*.sql` の内容をファイル名順に貼り付けて実行する

### B. Supabase CLI

```sh
brew install supabase/tap/supabase
supabase login
supabase link --project-ref tbvzseiehzuobglwbedo   # DB パスワードを聞かれる
supabase db push                                    # 未適用の migrations を順に適用
```

CLI で適用すると `supabase_migrations.schema_migrations` に適用履歴が残り、
以後 `supabase db push` で差分だけ適用できる。A で適用した場合は履歴が残らないため、
CLI に切り替える時は `supabase migration repair --status applied 20260922000000` で整合させる。

## 新しいマイグレーションの追加

- ファイル名は `YYYYMMDDHHmmss_<内容>.sql`（Supabase CLI の規約）
- 既存ファイルは編集しない。変更は新しいファイルで積む
- 列や表を落とす・型を変える破壊的変更はオーナー承認必須。Routine は `docs/proposals/` に提案を書くに留める
