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

## 自動マージしてよい範囲

- `crawler/sites/*.yaml`（セレクタ定義。テスト通過が条件）
- `docs/**`、`reports/**`

それ以外はオーナー承認を待つ。`destructive-migration` ラベルが付いた PR は、上記に該当しても自動マージしない。

## 存在する GitHub Secrets（値は読めない。名前だけ知っておく）

- `SUPABASE_URL` — `https://<project-id>.supabase.co`
- `SUPABASE_SECRET_KEY` — `sb_secret_...`（RLSを無視できる。クローラーの書き込み専用。フロントに出さない）
- `FIREBASE_SERVICE_ACCOUNT` — デプロイ用サービスアカウントJSON
- `SUPABASE_ACCESS_TOKEN` — Supabase CLI の認証トークン（`sbp_...`）。`deploy.yml` の `db push` と `ci.yml` の dry-run が使う
- `SUPABASE_DB_PASSWORD` — 本番 DB の postgres パスワード。同上。コードやログに出さない

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
