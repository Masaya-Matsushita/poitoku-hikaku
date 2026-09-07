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
| AIの自動マージ範囲 | 初期は **セレクタ定義（`crawler/sites/*.yaml`）とドキュメントのみ自動マージ可**。それ以外はオーナー承認 |
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
| 破壊的マイグレーション | オーナー承認必須（Routineは提案のみ） |

## 事故が起きてから足すもの（想定リスト）

- Routine の1晩あたりの変更量上限
- 特定ディレクトリの変更禁止
- デプロイ後のスモークテストと自動ロールバック
- 対象サイトからの警告メールへの対応手順

事故は `logs/incidents/YYYY-MM-DD-*.md` に記録し、追加したガードレールをこのファイルに追記する。
