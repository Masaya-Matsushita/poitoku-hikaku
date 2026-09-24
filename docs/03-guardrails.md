# 03 — ガードレール

基本思想：**最初はノーガードでAIを運転し、事故が起きたらガードレールを足す**。
ただし以下は「事故が起きてからでは取り返しがつかない」ため、初日から入れる。

## 初日から入れるもの

### コード・デプロイ

| ガードレール | 実装 |
|---|---|
| main への直push禁止 | GitHub Branch protection（PR必須、CI必須） |
| 本番デプロイは main からのみ | GitHub Actions の deploy job を `if: github.ref == 'refs/heads/main'` |
| テスト通過必須 | CI で `go test` / `vitest` / lint。失敗したPRはマージ不可 |
| AIの自動マージ範囲 | **CI 全通過・`destructive-migration` / `needs-owner-review` ラベルなし・オーナー承認パスに触れない PR は自動マージ**（ADR-0006）。オーナー承認パス：`.github/**`、`crawler/internal/policy/**`、このファイル、`crawler/sites/*.yaml` の新規追加。判定は `automerge.yml`（`ci.yml` の完了で main の版が動く）。満たさない PR には `needs-owner-review` を付ける。AI は自分でマージしない。`reports/**` の日次レポート PR は `report.yml` が作成し CI 通過後に自動マージする |
| 判定を飛ばしたマージの防止 | 判定済みのコミットに commit status `auto-merge-gate` を付け、Branch protection の必須チェックにする（新しい push の後、古い判定で auto-merge が発火しない） |
| シークレット | GitHub Secrets のみ。`.env*` は `.gitignore`。Claude は `.env*` を読まない（AGENTS.md に明記） |

### クローラー（対象サイトへの加害防止）

| ガードレール | 実装 |
|---|---|
| リクエスト間隔 3秒以上 | `crawler/internal/policy` の定数で固定し、**テストで担保**（間隔を短くする変更はCIで落ちる） |
| 1日1回 | `crawl.yml` の cron（03:00 JST）のみから起動。cron が 1 本で日次であることも policy のテストが検証する。手動起動（workflow_dispatch）はオーナーのみ |
| robots.txt 遵守 | クローラー起動時に取得し、メニューと一覧の URL が許可されているか検証。禁止なら `robots_disallow` で打ち切る |
| サーキットブレーカー | 429/403 で即停止、5xx 連続3回で停止 |
| User-Agent に連絡先明記 | `poitoku-hikaku/1.0 (+https://poitoku-hikaku.com/about)` |

### コスト

| ガードレール | 実装 |
|---|---|
| 支払い手段を登録しない | Firebase Spark、Supabase Free、GitHub Free。請求アカウント自体を作らない |
| 無料枠監視 | 週次 Routine が各サービスの使用量を確認し `reports/` に記録 |
| 有料LLM API キーを持たない | リポジトリにも Secrets にも置かない |

### シークレットの期限

週次 Routine（`docs/05-routines.md` の weekly-report）がこの表を読み、**残り 30 日を切った項目を `reports/` に警告として出す**。
2026-09 時点で期限付きの Secret は無い（Supabase のアクセストークンは CI で使わない設計にして削除した）。
期限付きの Secret を CI で使うことになったら、期限と「切れると止まるもの」をこの表に書く。更新したら期限もこの表に書き直す。

| Secret | 使う場所 | 期限 | 切れると止まるもの |
|---|---|---|---|
| `SUPABASE_DB_PASSWORD` | `deploy.yml`（db push）、`ci.yml`（dry-run）。セッションプーラー直結 | 無期限（DB パスワード変更時に更新） | マイグレーション適用 → その後段の Hosting デプロイ |
| `SUPABASE_URL` | `crawl.yml`、`report.yml` | 無期限 | — |
| `SUPABASE_SECRET_KEY` | `crawl.yml`、`report.yml` | 無期限（ローテーション時に更新） | 日次クロールと日次レポート |
| `FIREBASE_SERVICE_ACCOUNT` | `deploy.yml`（hosting） | 無期限（鍵を失効させない限り） | Hosting デプロイ |
| `AUTOMERGE_APP_CLIENT_ID` / `AUTOMERGE_APP_PRIVATE_KEY` | `automerge.yml`（auto-merge の有効化・解除） | 無期限（App の秘密鍵を失効させない限り） | 自動マージ。条件を満たす PR に判定の status が付かず止まる（オーナーは管理者権限でマージ可） |

### データ

| ガードレール | 実装 |
|---|---|
| 週次バックアップ | GitHub Actions で `pg_dump` → 暗号化 → GitHub Release（private）or 別リポジトリ |
| マイグレーションは main からのみ適用 | `deploy.yml` が main への push 時に `supabase db push` を実行し、その後に Hosting を配信。手元や Dashboard から本番に直接 DDL を流さない |
| 適用前に内容が見える | `supabase/migrations/` を変更する PR では `ci.yml` が本番に対して `db push --dry-run` を実行し、適用予定の SQL を PR コメントに出す |
| 破壊的マイグレーション | dry-run の SQL に drop / alter ... type / truncate があれば CI が `destructive-migration` ラベルを付ける。ラベル付き PR はオーナー承認必須（Routine は提案のみ。ラベルを外すのもオーナー） |

## 事故が起きてから足すもの（想定リスト）

- Routine の1晩あたりの変更量上限
- 特定ディレクトリの変更禁止
- デプロイ後のスモークテストと自動ロールバック
- 対象サイトからの警告メールへの対応手順

事故は `logs/incidents/YYYY-MM-DD-*.md` に記録し、追加したガードレールをこのファイルに追記する。
