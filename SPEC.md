# SPEC.md - ポイ得比較 仕様書

このドキュメントは**確定した仕様のみ**を記載するSingle Source of Truth（SSOT）です。
将来構想や検討中の事項は `ai/backlog/` に記載します。

---

## プロジェクト概要

| 項目 | 内容 |
|------|------|
| サービス名 | ポイ得比較（Poitoku-Hikaku） |
| コンセプト | ポイントサイトの案件を横断検索・比較できるWebサービス |
| URL | https://poitoku-hikaku.com |
| 最重要要件 | **メンテナンスを極力減らすこと** |

---

## 技術スタック

### フロントエンド

| 技術 | 用途 | 選定理由 |
|------|------|----------|
| Next.js (App Router) | フレームワーク | SSG/ISRによるSEO最適化、Vercelとの親和性 |
| TypeScript | 言語 | 型安全な開発 |
| Tailwind CSS | スタイリング | 高速なUI開発 |
| shadcn/ui | UIコンポーネント | Radix UIベース、アクセシビリティ対応 |

### バックエンド / データ収集

| 技術 | 用途 | 選定理由 |
|------|------|----------|
| Python | クローラー言語 | Crawl4AIとの親和性 |
| Crawl4AI | AIクローリング | HTML構造変更への耐性（LLM抽出） |
| gpt-4o-mini | LLMモデル | コスト効率（月$1-3程度） |

### インフラ

| 技術 | 用途 | 選定理由 |
|------|------|----------|
| Supabase | データベース | PostgreSQL、無料枠あり、RLS対応 |
| Vercel | ホスティング | Next.jsとの親和性、無料枠あり |
| GitHub Actions | 日次クローリング | 無料枠で十分 |

---

## データベース設計

### テーブル定義

#### offers（案件データ）

```sql
CREATE TABLE offers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  site_name VARCHAR(50) NOT NULL,
  offer_name VARCHAR(255) NOT NULL,
  reward INTEGER NOT NULL,
  description TEXT,
  url VARCHAR(500) NOT NULL,
  category VARCHAR(100),
  fetched_date DATE NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  UNIQUE(site_name, url, fetched_date)
);

-- 部分一致検索用インデックス
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_offers_offer_name_trgm ON offers USING gin (offer_name gin_trgm_ops);

-- 検索クエリ用インデックス
CREATE INDEX idx_offers_fetched_date ON offers(fetched_date);
CREATE INDEX idx_offers_site_name ON offers(site_name);
```

#### crawl_logs（クローリングログ）

```sql
CREATE TABLE crawl_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  site_name VARCHAR(50) NOT NULL,
  crawl_date DATE NOT NULL,
  model_name VARCHAR(50),
  total_count INTEGER NOT NULL,
  success_count INTEGER NOT NULL,
  error_count INTEGER NOT NULL,
  duration_seconds INTEGER,
  error_messages TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  UNIQUE(site_name, crawl_date)
);
```

### Row Level Security (RLS)

```sql
-- offers テーブル
ALTER TABLE offers ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Allow public read access" ON offers FOR SELECT USING (true);

-- crawl_logs テーブル
ALTER TABLE crawl_logs ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Allow public read access" ON crawl_logs FOR SELECT USING (true);
```

### TypeScript 型定義

```typescript
export interface Offer {
  id: string;
  site_name: string;
  offer_name: string;
  reward: number;
  description: string | null;
  url: string;
  category: string | null;
  fetched_date: string;
  created_at: string;
}

export interface CrawlLog {
  id: string;
  site_name: string;
  crawl_date: string;
  model_name: string | null;
  total_count: number;
  success_count: number;
  error_count: number;
  duration_seconds: number | null;
  error_messages: string | null;
  created_at: string;
}
```

---

## クローリング設計

### 基本方針

| 項目 | 設定 |
|------|------|
| 実行頻度 | 1日1回（00:00 JST） |
| リクエスト間隔 | 3秒以上 |
| robots.txt | 遵守 |

### サーキットブレーカー

クローラーの安定運用のため、以下の条件で即時中断します。

| 条件 | 動作 |
|------|------|
| 429（Rate Limit）/ 403（Forbidden） | 即時中断 |
| 5xx系エラーが連続3回 | 即時中断 |
| タイムアウト（リトライ上限超過） | 中断 |

### 障害時のデータ保持ポリシー

| 状況 | 対応 | 表示 |
|------|------|------|
| クローリング成功 | 新データでDBを更新 | 最新データを表示 |
| 失敗（1日） | 前日のデータを保持 | 「最終更新: 1日前」と表示 |
| 失敗（3日連続） | 3日前のデータを保持 | 「最終更新: 3日前」+ 警告バナー |
| 失敗（7日連続） | 7日前のデータを保持 | 「データが古い可能性があります」と警告 |
| 失敗（14日以上） | データ非表示を検討 | 「現在データを取得できません」 |

**方針**: 古いデータでも「表示しない」より「表示する」方がユーザーに有益。ただし古さを明示して誤解を防ぐ。

---

## 対象ポイントサイト（MVP）

| サイト | URL | ポイント単価 | 紹介コード |
|--------|-----|-------------|-----------|
| ハピタス | https://hapitas.jp/ | 1pt = 1円 | `IGVWXW` |
| モッピー | https://pc.moppy.jp/ | 1pt = 1円 | `6v5NA1ab` |

---

## 機能要件（MVP）

| 機能 | 説明 |
|------|------|
| キーワード検索 | 案件名で横断検索（部分一致） |
| 比較表示 | 還元額の降順でランキング表示 |
| サイト遷移リンク | 各ポイントサイトへのリンク |
| 紹介リンク表示 | 紹介コード/URLの案内 |

---

## 非機能要件

| 要件 | 目標 |
|------|------|
| コスト | 月額 $0〜$20 程度（無料枠活用） |
| パフォーマンス | 検索結果表示 1秒以内 |
| SEO | 検索エンジンからの流入を獲得 |
| 可用性 | 99%以上（Vercel依存） |
| 運用負荷 | 月1回30分以下 |

---

## 成功指標（MVP）

MVPの目的は**収益化ではなく「動く仕組みを作る」こと**。

| 指標 | 目標 | 備考 |
|------|------|------|
| クローリング成功率 | 90%以上 | 10回中9回以上データ取得成功 |
| データ精度 | 95%以上 | ポイント数が正確に取得できる |
| 自動運用期間 | 1週間以上 | 手動介入なしで稼働 |
| 検索応答速度 | 1秒以内 | UX最低ライン |

---

## 法的要件

| 項目 | 対応 |
|------|------|
| robots.txt遵守 | 許可範囲内でクローリング |
| 利用規約 | 公開情報の収集のみ、ログイン不要なページが対象 |
| アクセス頻度 | 1日1回、リクエスト間隔3秒以上 |
| プロモーション表記 | 「本ページはプロモーションを含みます」を明示 |
