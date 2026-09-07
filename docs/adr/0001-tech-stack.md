# ADR-0001 技術スタック

日付：2026-09-07　ステータス：採択

## 決定

| 層 | 技術 |
|---|---|
| クローラー | Go（`crawler/`）。セレクタは YAML 定義 |
| DB | Supabase（PostgreSQL、Free） |
| フロント | React（Vite）＋ TypeScript。ビルド時に全ページを静的生成 |
| ホスティング | Firebase Hosting（Spark） |
| ジョブ実行・CI/CD | GitHub Actions |
| AI開発 | Claude Code（Max Plan）＋ Routines |

## 理由

- オーナーが本業で Go / React を使っており、AI主導でもレビューが成立する
- 2026-02 の Next.js / Vercel / Python 構成は「人が保守する」前提で選ばれたもので、前提が変わった
- Vercel Hobby は商用利用不可のため、収益前提では使えない
- GCS 静的ホスティングは独自ドメインHTTPSに Cloud Load Balancer（有料）が必要。Firebase Hosting は無料でHTTPS付き

## 却下した案

- Cloud Run + Cloud SQL：Cloud SQL は最小構成でも有料。Cloud Run Jobs は無料枠内だが、GCP は請求アカウントが必要で「構造的0円」を満たさない（ADR-0002）
- SSR：プログラマティックSEOは静的生成で十分。1,000ページ規模でもビルドは数分

## 結果

- Cloud Run の経験は今回得られない。クロール時間が GitHub Actions の制限（6時間/ジョブ）に近づいたら再検討
