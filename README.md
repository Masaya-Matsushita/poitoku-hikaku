# ポイ得比較（Poitoku-Hikaku）

ポイントサイトの案件を横断検索・比較できるWebサービス

**URL**: https://poitoku-hikaku.com

## 概要

「ポイ得比較」は、複数のポイントサイト（ポイ活サイト）の案件を一括検索し、どのサイト経由が最もお得かを比較できるサービスです。

ユーザーが商品名やサービス名で検索すると、対応するポイントサイトの還元額を一覧で表示し、最もお得な選択肢を簡単に見つけることができます。

## 特徴

- **メンテナンス最小化**: AIクローリング（Crawl4AI）採用でHTML変更に強い
- **シンプルなUI**: dokotoku.jp を参考にした検索 + テーブル形式
- **低コスト運用**: 無料枠を活用した個人開発向け設計

## 機能

### MVP（現在）

- キーワード検索による案件横断検索
- 還元額の降順でのランキング表示
- 各ポイントサイトへの遷移リンク
- 対応サイト：
  - [ハピタス](https://hapitas.jp/)
  - [モッピー](https://pc.moppy.jp/)

### 将来構想

- 対応ポイントサイトの拡充
- 価格推移グラフの表示
- 価格変動通知機能
- カテゴリ別ブラウジング

## 技術スタック

### フロントエンド

- **Next.js** (App Router) - SSG/ISRによるSEO最適化
- **TypeScript** - 型安全な開発
- **Tailwind CSS** - UIスタイリング
- **shadcn/ui** - UIコンポーネント（Radix UIベース、アクセシビリティ対応）

### バックエンド / データ収集

- **[Crawl4AI](https://github.com/unclecode/crawl4ai)** - AIクローリング（OSS）
- **Python** - クローラー言語
- **Supabase** - 案件データの保存（PostgreSQL）
- **GitHub Actions** - 日次クローリングの自動実行

### インフラ

- **Vercel** - ホスティング・デプロイ

## なぜ Crawl4AI を採用するか

**最重要要件「メンテナンスを極力減らす」** を実現するため。

| 観点 | 従来型（cheerio等） | Crawl4AI |
|------|-------------------|----------|
| HTML変更への耐性 | ❌ 弱い（セレクタ修正必要） | ✅ 強い（LLM抽出） |
| メンテナンス | ❌ 月数回の修正 | ✅ ほぼ不要（期待） |
| 速度 | ✅ 高速 | ⚠️ やや遅い |
| コスト | ✅ 無料 | ⚠️ LLM API費用（月$1-3程度） |

## アーキテクチャ

```
┌─────────────────────────────────────────────────────────┐
│                  GitHub Actions (cron)                   │
│                    毎日 00:00 JST に実行                  │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                   クローラー (Python)                     │
│                                                         │
│   ┌──────────────────────────────────────────────────┐  │
│   │                    Crawl4AI                      │  │
│   │                                                  │  │
│   │  LLM抽出: HTML構造に依存せず意味ベースで抽出           │  │
│   │  フォールバック: CSS抽出も可能                       │  │
│   └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                   Supabase (PostgreSQL)                 │
│                                                         │
│   offers テーブル                                        │
│   ├── site_name, offer_name, reward                     │
│   ├── url, category, fetched_at                         │
│   └── (日付付きで履歴保存)                                 │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                    Next.js (Vercel)                     │
│                                                         │
│   ISR (Incremental Static Regeneration)                 │
│   └── 1日1回 revalidate で最新データ反映                   │
└─────────────────────────────────────────────────────────┘
```

## プロジェクト構成

```
poitoku-hikaku/
├── src/                          # Next.js フロントエンド（予定）
│   ├── app/
│   │   ├── page.tsx              # トップページ
│   │   ├── search/
│   │   │   └── page.tsx          # 検索結果ページ
│   │   └── layout.tsx            # 共通レイアウト
│   ├── components/
│   │   ├── SearchForm.tsx        # 検索フォーム
│   │   └── OfferTable.tsx        # 結果テーブル
│   ├── lib/
│   │   └── supabase.ts           # Supabaseクライアント
│   └── types/
│       └── offer.ts              # 型定義
│
├── crawler/                       # Python クローラー
│   ├── main.py                   # エントリポイント
│   ├── sites/
│   │   ├── moppy.py              # モッピー用
│   │   └── hapitas.py            # ハピタス用
│   ├── schemas/
│   │   └── offer.py              # LLM抽出スキーマ（Pydantic）
│   └── requirements.txt          # Python依存関係
│
├── .github/
│   └── workflows/
│       └── crawl.yml             # 日次クローリング
│
├── docs/                          # ドキュメント
│   ├── setup.md                  # 環境構築手順
│   ├── requirements.md           # 要件定義
│   ├── structure.md              # 技術構成・DB設計
│   ├── mvp.md                    # MVP計画
│   ├── competitor.md             # 競合分析
│   ├── point-sites.md            # 対象ポイ活サイト一覧
│   └── backlog.md                # バックログ・今後の検討事項
│
├── ai/                            # AI関連
│   ├── plans/                     # 計画・設計ドキュメント
│   └── prompts/                   # プロンプト集
│
├── AGENTS.md                      # AI Agent向けルール
└── README.md
```

## 紹介リンク

| サイト | 紹介コード | 紹介URL |
|--------|-----------|---------|
| ハピタス | `IGVWXW` | https://hapitas.jp/appinvite?i=24798796&route=pcText |
| モッピー | `6v5NA1ab` | https://pc.moppy.jp/entry/invite.php?invite=6v5NA1ab |

## 開発

### 事前準備

開発を開始する前に、以下の準備が必要です：

👉 **[事前準備ガイド](docs/setup.md)** を参照してください。

- Python 3.11+ のインストール
- Node.js 20+ のインストール
- OpenAI APIキーの発行と課金設定
- Supabaseプロジェクトの作成とテーブル作成
- 環境変数ファイル（`.env.local`）の作成

### セットアップ

```bash
# フロントエンド
pnpm install
pnpm run dev

# クローラー
cd crawler
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python main.py
```

### 環境変数

```env
# Supabase
SUPABASE_URL=your_supabase_url
SUPABASE_ANON_KEY=your_supabase_anon_key

# OpenAI (Crawl4AI LLM抽出用)
OPENAI_API_KEY=your_openai_api_key
```

## ドキュメント

詳細は `docs/` ディレクトリを参照：

- [事前準備ガイド](docs/setup.md)
- [要件定義](docs/requirements.md)
- [技術構成](docs/structure.md)
- [MVP計画](docs/mvp.md)
- [競合分析](docs/competitor.md)
- [対象ポイ活サイト一覧](docs/point-sites.md)
- [バックログ・今後の検討事項](docs/backlog.md)

## 開発ステップ

1. **PoC**: Crawl4AIで「楽天カード」データ取得検証 ← 現在地
2. **クローラー実装**: モッピー → ハピタスの順
3. **DB設計**: Supabaseでスキーマ作成
4. **UI実装**: 検索フォーム + 結果テーブル
5. **日次実行**: GitHub Actionsでcron設定
6. **リリース**: 本番公開

## ライセンス

MIT License

## 法的注意事項

- 本サービスは公開情報の収集・比較を目的としています
- 各ポイントサイトの利用規約を遵守してください
- クローリングは適切な間隔で実施し、サーバーに過度な負荷をかけません
- robots.txt を遵守しています
- 本ページにはプロモーションを含みます
