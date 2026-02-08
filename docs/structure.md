# 技術構成・アーキテクチャ

## 技術スタック

### フロントエンド

| 技術 | 用途 | 選定理由 |
|------|------|---------|
| Next.js (App Router) | フレームワーク | ISRによるSEO最適化、Vercelとの親和性 |
| TypeScript | 言語 | 型安全な開発 |
| Tailwind CSS | スタイリング | 高速なUI開発 |
| shadcn/ui | UIコンポーネント | Radix UIベース、アクセシビリティ担保、Tailwindカスタマイズ可 |

### バックエンド / データ収集

| 技術 | 用途 | 選定理由 |
|------|------|---------|
| **Crawl4AI** | クローリング | AIによるHTML変更耐性、メンテナンス削減 |
| Python | クローラー言語 | Crawl4AIがPython製 |
| Supabase | データベース | 無料枠あり、PostgreSQL、マネージド |

### インフラ / CI/CD

| 技術 | 用途 | 選定理由 |
|------|------|---------|
| Vercel | ホスティング | Next.jsとの親和性、無料枠 |
| GitHub Actions | 日次クローリング | 無料枠で十分、cron対応 |

## なぜ Crawl4AI を採用するか

### 従来型（axios + cheerio）との比較

| 観点 | axios + cheerio | Crawl4AI |
|------|-----------------|----------|
| HTML変更への耐性 | ❌ 弱い（セレクタ修正必要） | ✅ 強い（LLM抽出） |
| メンテナンス | ❌ 月数回の修正 | ✅ ほぼ不要 |
| 速度 | ✅ 高速 | ⚠️ やや遅い |
| コスト | ✅ 無料 | ⚠️ LLM API費用 |
| 精度 | ✅ 確実 | ⚠️ 要検証 |

### Crawl4AI の特徴

```python
# LLMなしでも使える（Markdown抽出）
async with AsyncWebCrawler() as crawler:
    result = await crawler.arun(url="...")
    print(result.markdown)

# LLM抽出（構造化データ）
extraction_strategy = LLMExtractionStrategy(
    provider="openai/gpt-4o-mini",
    schema=OfferSchema
)
```

- **完全OSS**: APIキー不要でも基本機能が使える
- **LLM抽出はオプション**: 必要に応じて有効化
- **アクティブな開発**: GitHub #1 トレンド

### ハイブリッドアプローチ

```
Crawl4AI
    ├── 通常時: LLM抽出（メンテナンスフリー）
    └── フォールバック: CSS/XPath抽出（精度保証）
```

## アーキテクチャ図

```
┌─────────────────────────────────────────────────────────────┐
│                  GitHub Actions (cron)                       │
│                    毎日 00:00 JST に実行                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    クローラー (Python)                        │
│                                                              │
│   ┌──────────────────────────────────────────────────────┐  │
│   │                    Crawl4AI                          │  │
│   │  ┌─────────────┐  ┌─────────────┐                   │  │
│   │  │  モッピー    │  │  ハピタス    │                   │  │
│   │  │  crawler    │  │  crawler    │                   │  │
│   │  └─────────────┘  └─────────────┘                   │  │
│   │         │                │                           │  │
│   │         └────────┬───────┘                           │  │
│   │                  ▼                                   │  │
│   │        LLM抽出 or CSS抽出                            │  │
│   └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Supabase (PostgreSQL)                    │
│                                                              │
│   offers テーブル                                             │
│   ├── id (UUID)                                              │
│   ├── site_name (VARCHAR)      -- ハピタス, モッピー          │
│   ├── offer_name (VARCHAR)     -- 案件名                     │
│   ├── reward (INTEGER)         -- 還元額（円）               │
│   ├── original_reward (VARCHAR)-- 元表記（10,000pt等）       │
│   ├── description (TEXT)       -- 案件詳細                   │
│   ├── url (VARCHAR)            -- 案件ページURL              │
│   ├── category (VARCHAR)       -- カテゴリ                   │
│   ├── fetched_at (TIMESTAMP)   -- 取得日時                   │
│   └── created_at (TIMESTAMP)   -- 作成日時                   │
│                                                              │
│   ※ 日付付きで履歴保存 → 将来の傾向分析用                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Next.js (Vercel)                         │
│                                                              │
│   ISR (Incremental Static Regeneration)                      │
│   ├── /                    -- トップページ（検索フォーム）    │
│   ├── /search?q=xxx        -- 検索結果ページ                 │
│   └── revalidate: 86400    -- 1日1回再生成                   │
└─────────────────────────────────────────────────────────────┘
```

## データモデル

### offers テーブル

```sql
CREATE TABLE offers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  site_name VARCHAR(50) NOT NULL,
  offer_name VARCHAR(255) NOT NULL,
  reward INTEGER NOT NULL,
  original_reward VARCHAR(50),
  description TEXT,
  url VARCHAR(500) NOT NULL,
  category VARCHAR(100),
  fetched_at TIMESTAMP WITH TIME ZONE NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

  -- 同一案件の重複防止
  UNIQUE(site_name, url, fetched_at::DATE)
);

-- 検索用インデックス
CREATE INDEX idx_offers_offer_name ON offers(offer_name);
CREATE INDEX idx_offers_fetched_at ON offers(fetched_at DESC);
CREATE INDEX idx_offers_site_name ON offers(site_name);
```

### 検索方式

**MVP: ILIKE検索**

Supabaseの無料枠では日本語全文検索（`to_tsvector('japanese', ...)`）は使えない。
MVPでは ILIKE による部分一致検索で十分。案件数1000件程度なら性能問題なし。

```typescript
// 単一キーワード検索
const { data } = await supabase
  .from('offers')
  .select('*')
  .ilike('offer_name', `%${keyword}%`)
  .order('reward', { ascending: false });

// スペース区切りの複数キーワード（AND検索）
const keywords = searchQuery.split(/\s+/);
let query = supabase.from('offers').select('*');
for (const kw of keywords) {
  query = query.ilike('offer_name', `%${kw}%`);
}
const { data } = await query.order('reward', { ascending: false });
```

**将来の拡張オプション**:
- `pg_trgm` 拡張: 類似検索・タイポ許容（Supabase無料枠で利用可）
- Algolia / Meilisearch: 本格的な日本語検索（有料）

### TypeScript型定義

```typescript
interface Offer {
  id: string;
  siteName: 'hapitas' | 'moppy';
  offerName: string;
  reward: number;           // 円換算
  originalReward: string;   // "10,000pt" など
  description: string;
  url: string;
  category: string;
  fetchedAt: Date;
}
```

## クローリング戦略

### 各サイトの取得方法（検証済み）

| サイト | 取得方法 | 対象URL | 備考 |
|--------|---------|---------|------|
| モッピー | 検索結果ページ | `/search/?word=xxx` | HTMLにデータ直接埋め込み |
| ハピタス | 個別案件ページ | `/item/detail/itemid/xxx/` | meta/titleにポイント情報 |

### robots.txt 確認結果

両サイトとも案件ページへのアクセスは許可されている。

```
# ハピタス
Disallow: /item/redirect/*
# /item/detail/ は許可

# モッピー
Disallow: /ad/j.php
Disallow: /ad/r.php
# /search/, /ad/detail.php は許可
```

### クローリング設定

```python
CRAWL_CONFIG = {
    "delay_between_requests": 3,  # 秒
    "user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) ...",
    "max_retries": 3,
    "timeout": 30,
}
```

## ディレクトリ構成

```
poikatsu-app/
├── src/                          # Next.js フロントエンド
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
│   │   └── offer.py              # LLM抽出スキーマ
│   └── requirements.txt          # Python依存関係
│
├── .github/
│   └── workflows/
│       └── crawl.yml             # 日次クローリング
│
├── docs/                          # ドキュメント
│   ├── requirements.md
│   ├── structure.md
│   ├── mvp.md
│   └── competitor.md
│
└── README.md
```

## コスト見積もり

### MVP（2サイト）

| サービス | 無料枠 | 想定使用量 | 月額費用 |
|---------|-------|-----------|---------|
| Vercel | 100GB帯域 | 1GB程度 | $0 |
| Supabase | 500MB DB | 100MB程度 | $0 |
| GitHub Actions | 2,000分/月 | 60分程度 | $0 |
| OpenAI API (gpt-4o-mini) | - | 200案件/日 | ~$1-3 |

**合計: 月額 $1〜3 程度**（LLM APIのみ）

### サイト数によるスケール

| サイト数 | LLM APIコスト（月） | 備考 |
|---------|-------------------|------|
| 2サイト | $1-3 | MVP |
| 5サイト | $2.5-7.5 | |
| 10サイト | $5-15 | |
| 20サイト | $10-30 | |

**コスト計算の前提**:
```
1サイト × 100案件 × 30日 = 3,000リクエスト/月
1リクエスト ≒ 2,000トークン
gpt-4o-mini: $0.15/1Mトークン（入力）

3,000 × 2,000 = 6Mトークン/月/サイト
6M × $0.15/1M ≒ $0.9/月/サイト
```

**コスト削減オプション**:
- CSS抽出をメインにしてLLMはフォールバックのみ → 大幅削減
- ローカルLLM（Ollama等）を使う → $0（精度は要検証）
