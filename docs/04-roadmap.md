# 04 — ロードマップ

「設計は全部入り、リリースは土台から」。継続デプロイなので、フェーズは順序であって締切ではない。

## Phase 0：調査（オーナー＋チャット、着手前〜並行）

- [ ] モッピー・ハピタスの利用規約・robots.txt を再確認（2026-02の調査の再検証）
- [ ] 紹介コードの有効性確認（姓変更後もアカウントは同一のはず）
- [x] ASP（AppDriver / SKYFLAG / GF Rewards）の個人提携条件 → `docs/reference/asp-research-2026-09.md`（初期は一般ASPで代替）
- [ ] ステマ規制の表示文言確定
- [ ] ドメイン取得 `poitoku-hikaku.com`

## Phase 1：データパイプライン（最優先、目標：着手から1週間で本番稼働）

- [ ] リポジトリ初期化、AGENTS.md、CI、Branch protection
- [ ] Supabase スキーマ（`offers` / `crawl_logs` / `sites`。SPEC.md 旧版のDDLをベースに、履歴・正規化を見据えて拡張）
- [ ] Go クローラー：モッピー（検索結果HTML）
- [ ] Go クローラー：ハピタス（個別ページ `<title>`）
- [ ] セレクタを YAML 定義に分離
- [ ] GitHub Actions 日次 cron で本番稼働開始 ← **ここで時計が回り始める**
- [ ] `crawl_logs` から KPI を日次レポート生成

## Phase 2：公開（データが貯まり始めた直後）

- [ ] React（Vite）＋ 静的生成：トップ検索、`/offers/{slug}`
- [ ] Firebase Hosting デプロイ（GitHub Actions から）
- [ ] sitemap.xml、構造化データ、`llms.txt`
- [ ] Search Console 連携
- [ ] ASP（A8.net / afb / アクセストレード）メディア登録・提携申請
- [ ] ステマ表示、about ページ、紹介コード表示
- [ ] 夜間 Routine 稼働開始（`docs/05-routines.md`）

## Phase 3：履歴の価値化（30日以上のデータが貯まってから）

- [ ] 価格履歴グラフ
- [ ] 過去最高・買い時判定
- [ ] 異常値検知 → オーナーへ通知
- [ ] X速報（承認制）
- [ ] 価格変動アラート（LINE公式 or メール）

## Phase 4：拡大

- [ ] 3サイト目以降（ポイントインカム、ECナビ…）。追加はセレクタYAML＋Routineで
- [ ] 案件名の正規化（同一案件のサイト横断マッチング）
- [ ] ASP提携が可能なら案件リンクの直接化
- [ ] 第二弾に向けた共通部分の抽出（リポジトリの型、インフラ構成、パイプライン）。**第二弾の要件が見えてから**着手
- [ ] 横展開の検討（ふるさと納税など。`logs/meetings/2026-02-12-2-alternative-ideas.md`）
