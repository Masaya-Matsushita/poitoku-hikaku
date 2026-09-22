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
| AIの自動マージ範囲 | 初期は **セレクタ定義（`crawler/sites/*.yaml`）とドキュメントのみ自動マージ可**。それ以外はオーナー承認。`destructive-migration` ラベルが付いた PR は範囲内でも自動マージ不可 |
| シークレット | GitHub Secrets のみ。`.env*` は `.gitignore`。Claude は `.env*` を読まない（AGENTS.md に明記） |

### クローラー（対象サイトへの加害防止）

| ガードレール | 実装 |
|---|---|
| リクエスト間隔 3秒以上 | コードで固定し、**テストで担保**（間隔を短くする変更はCIで落ちる） |
| 1日1回 | GitHub Actions の cron のみから起動。手動起動はオーナーのみ |
| robots.txt 遵守 | クローラー起動時に取得・検証 |
| サーキットブレーカー | 429/403 で即停止、5xx 連続3回で停止 |
| User-Agent に連絡先明記 | `poitoku-hikaku/1.0 (+https://poitoku-hikaku.com/about)` |

### コスト

| ガードレール | 実装 |
|---|---|
| 支払い手段を登録しない | Firebase Spark、Supabase Free、GitHub Free。請求アカウント自体を作らない |
| 無料枠監視 | 週次 Routine が各サービスの使用量を確認し `reports/` に記録 |
| 有料LLM API キーを持たない | リポジトリにも Secrets にも置かない |

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
