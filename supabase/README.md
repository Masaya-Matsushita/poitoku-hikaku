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

## 適用の流れ（自動）

本番への適用は **main へのマージ時に `deploy.yml` が行う**。手元や Dashboard から本番に直接 DDL を流さない。

```
PR で supabase/migrations/ を変更
  → ci.yml「supabase dry-run」：supabase link → 本番に対して supabase db push --dry-run --include-all
      → 適用予定の SQL を PR コメントに出す（push ごとに同じコメントを更新）
      → drop / alter ... type / truncate を含めば destructive-migration ラベルを付ける
main にマージ
  → deploy.yml「supabase db push」：supabase link → supabase db push --include-all で適用
  → 成功したら「firebase hosting」：web/ をビルドして配信（DB が失敗したら配信しない）
```

- CI では `db push` の前に必ず `supabase link --project-ref tbvzseiehzuobglwbedo` を実行する。GitHub Actions のランナーは IPv6 を持たず、DB への直接接続（IPv6 のみ）ができないため、link がプーラー（IPv4）経由の接続設定を作る。link 後の `db push` に `--project-ref` は不要
- 適用済みなら `db push` は「up to date」で何もしない（冪等）
- `--include-all`：リモート履歴に無いファイルをタイムスタンプの新旧に関わらず適用する。並行する PR の順序が入れ替わっても取り残さないため
- CLI のバージョンは `ci.yml` と `deploy.yml` で `2.117.0` に固定している。上げる時は両方を変える
- 使う Secrets：`SUPABASE_ACCESS_TOKEN`（CLI 認証）、`SUPABASE_DB_PASSWORD`（DB パスワード）。名前は `AGENTS.md`
- 履歴は `supabase_migrations.schema_migrations` に残る。初回スキーマ（`20260922000000_initial_schema.sql`）は 2026-09-22 にオーナーがローカル CLI（`supabase link` → `supabase db push`）で適用し、履歴も記録済み

### 破壊的マイグレーション（`destructive-migration` ラベル）

`.github/scripts/detect-destructive-sql.sh` が、コメントを除いた各文に `drop` / `truncate` / `alter ... type` が含まれるかを見る。
文字列リテラル内の語も拾うので誤検知はありうる（疑わしい側に倒している）。

- ラベル付き PR は自動マージの対象外。オーナーがレビューして承認する（`docs/03-guardrails.md`）
- CI はラベルを付けるだけで外さない。誤検知や修正後に外すのはオーナー
- Routine は破壊的変更を実装せず、`docs/proposals/` に提案を書く

## 新しいマイグレーションの追加

- ファイル名は `YYYYMMDDHHmmss_<内容>.sql`（`supabase migration new <内容>` で生成できる）
- **既存ファイルは編集しない**。適用済みのファイルを変えても本番には反映されず、履歴と食い違うだけ。変更は新しいファイルで積む
- `create index concurrently` のようにトランザクション外で走る文は、単独のファイルに分けて `if not exists` を付ける（途中で失敗すると履歴なしの半適用になる）
- ローカルで構文だけ確かめたい時は PR を出せば dry-run コメントで確認できる

## 手動で触る必要がある時（オーナーのみ）

```sh
brew install supabase/tap/supabase
supabase login
supabase link --project-ref tbvzseiehzuobglwbedo   # DB パスワードを聞かれる
supabase db push --dry-run --include-all             # 適用予定の確認
supabase migration list                              # ローカルと本番の履歴の突き合わせ
```

Dashboard から直接 DDL を実行した場合や、履歴だけがずれた場合は `supabase migration repair --status applied <version>` で
`schema_migrations` を整合させる。CI の dry-run が「remote versions absent locally」で落ちる時もこれが原因。
