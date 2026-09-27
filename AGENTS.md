# AGENTS.md — ポイ得比較

このファイルは AI エージェント（Claude Code、Cursor、Routine とも）が最初に読む唯一のルール集。CLAUDE.md と .cursorrules はここへの参照のみ。

## このプロジェクトは何か

ポイントサイト案件の横断比較サービス。**AI主導で開発・運用**し、オーナーはレビューのみ行う。
目的の第一は「AI主導開発手法の実践」。収益は第二（`docs/00-vision.md`）。

## 最初に読む順序

1. `docs/00-vision.md` — 何のためにやっているか
2. `docs/03-guardrails.md` — やってはいけないこと
3. `docs/02-kpi.md` — 何を改善と呼ぶか
4. `docs/04-roadmap.md` — 今どこにいるか
5. 該当する `docs/adr/*.md`
6. `docs/lessons.md` — 過去に何が想定と違ったか（同じ轍を踏まないため）

`docs/reference/` と `logs/` は 2026-02 の記録を含む。**参考情報であり、現在の決定ではない**。現在の決定は `docs/` 直下と `docs/adr/` のみ。

## 絶対に守ること

- `.env*`、`secrets/` を読まない・書かない
- クロールのリクエスト間隔（3秒）と頻度（1日1回）を短くしない。テストが落ちる
- 有料サービス・有料APIを有効化しない。支払い手段は存在しない（ADR-0002）
- main に直接 push しない。すべてPR経由
- 破壊的DBマイグレーション（drop / alter ... type / truncate。CI が `destructive-migration` ラベルを付ける）は自動マージしない。適用済みのマイグレーションファイルは編集せず、新しいファイルを積む
- 対象サイトのコンテンツを転載しない。保存するのは案件名・還元額・URL・カテゴリのみ

## 自動マージしてよい範囲（ADR-0006）

マージの判定は CI（`.github/workflows/automerge.yml`）が行う。**AI は自分で `gh pr merge` しない**。PR を作ったら終わり。

次をすべて満たす PR は、CI が auto-merge を有効化して自動でマージされる：

- CI（`ci.yml` の全ジョブ）が通っている
- `destructive-migration` / `needs-owner-review` ラベルが付いていない
- 変更ファイルが下の「オーナー承認が必要なパス」に触れていない

オーナー承認が必要なパス：

- `.github/**`（ワークフローと CI のスクリプト。自動マージの判定そのものを含む）
- `crawler/internal/policy/**`（クロールの間隔・停止条件・User-Agent）
- `docs/03-guardrails.md`
- `crawler/sites/*.yaml` の新規追加（対象サイトを増やす。既存ファイルの変更は自動マージ可）

満たさない PR には CI が `needs-owner-review` ラベルを付け、理由をコメントする。ラベルを外すのはオーナー。
オーナー承認のパスに触れる変更は、無関係な変更と同じ PR に混ぜない（混ぜると全体が承認待ちになる）。
`reports/` は `report.yml` が毎日生成して自動マージする。手で編集しない。

## 存在する GitHub Secrets（値は読めない。名前だけ知っておく）

- `SUPABASE_URL` — `https://<project-id>.supabase.co`
- `SUPABASE_SECRET_KEY` — `sb_secret_...`（RLSを無視できる。クローラーの書き込みと日次レポートの読み取り（`crawl_logs` は非公開）に使う。フロントに出さない）
- `FIREBASE_SERVICE_ACCOUNT` — デプロイ用サービスアカウントJSON
- `SUPABASE_DB_PASSWORD` — 本番 DB の postgres パスワード。`deploy.yml` の `db push` と `ci.yml` の dry-run がセッションプーラー経由の `--db-url` で使う。コードやログに出さない
- `AUTOMERGE_APP_CLIENT_ID` / `AUTOMERGE_APP_PRIVATE_KEY` — 自動マージ用 GitHub App。`automerge.yml` が auto-merge の有効化・解除だけに使う（ADR-0006）

Supabase のアクセストークンは使わない（CI は Management API を呼ばない。`supabase/README.md`）。

publishable key（`sb_publishable_...`）は公開してよい鍵なので Secrets ではなくコードに置く。

## 技術規約

- Go：標準ライブラリ優先、`context` を必ず通す、型付きエラー。テストは `go test ./...`
- TypeScript：`strict`、型定義を省略しない、React は関数コンポーネントのみ
- ツールのバージョンは `.tool-versions`（asdf）で固定し、CI も同じファイルを読む。上げる時はここを変える
- PR を出す前にローカルで CI と同じ検証を通す：`crawler/` で `gofmt -l .`（出力なし）・`go vet ./...`・`go test ./...`、`web/` で `npm run lint`・`npm run typecheck`・`npm test`・`npm run build`
- クローラーの間隔・停止条件・User-Agent は `crawler/internal/policy` の定数だけを参照する。独自のリテラルを持たない
- セレクタは `crawler/sites/<site>.yaml` に置き、Go コードにサイト固有の値を書かない。変更は `crawler/testdata/` のフィクスチャでテストが通ること。実サイトへの確認は `go run ./cmd/crawler -site <site> -dry-run`（policy の間隔で数百リクエスト飛ぶ。1 回で済ませる）
- 過度な抽象化をしない。3回同じことを書いてから共通化する
- コミットメッセージ・コメントは日本語可。1コミット＝1論理変更
- ドキュメントの変更は同じPRに含める

## 迷ったら

- 「何もしない」は正解のひとつ。無意味な変更を積まない
- 判断に迷う変更は、実装せずPRの代わりに `docs/proposals/` に提案を書く
