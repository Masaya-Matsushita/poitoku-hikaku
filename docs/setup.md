# 開発 事前準備ガイド

開発の着手前に、以下の準備を上から順に完了させてください。

---

### プラグインの追加

※ asdf でバージョン管理している前提

```bash
# Python プラグイン追加
asdf plugin add python

# Node.js プラグイン追加
asdf plugin add nodejs
```

### バージョンのインストール

```bash
cd /poitoku-hikaku

# .tool-versions に定義されたバージョンをインストール
asdf install

# または個別にインストール
asdf install nodejs 24.13.0
asdf install python 3.13.12
```

### 確認

```bash
# プロジェクトディレクトリで実行
node --version
# 期待値: v24.13.0

python --version
# 期待値: Python 3.13.12
```

---

## 2. pnpm のインストール

```bash
# corepack で pnpm を有効化（Node.js に同梱）
corepack enable
corepack prepare pnpm@latest --activate

# 確認
pnpm --version
# 期待値: 10.x.x 以上
```

---

## 3. OpenAI APIキーの発行

Crawl4AIのLLM抽出機能で使用します。

### 手順

1. [OpenAI Platform](https://platform.openai.com/) にアクセス
2. アカウント作成（またはログイン）
3. 左メニュー「API keys」→「Create new secret key」
4. キー名を入力（例: `poitoku-hikaku`）して作成
5. **表示されたキーをコピーして保存**（二度と表示されません）

### 課金設定（重要）

無料クレジットがない場合、APIは動作しません。

1. 左メニュー「Settings」→「Billing」
2. 「Add payment method」でクレジットカード登録
3. 「Add credits」で $5〜10 程度チャージ

---

## 4. Supabaseプロジェクトの作成

データベース（PostgreSQL）として使用します。

### 手順

1. [Supabase](https://supabase.com/) にアクセス
2. 「Start your project」→ GitHubアカウントでサインイン
3. 「New project」をクリック
4. 以下を入力：
   - **Organization**: 自分のOrganization（なければ作成）
   - **Project name**: `poitoku-hikaku`
   - **Database Password**: 強力なパスワードを設定（**メモしておく**）
   - **Region**: `Northeast Asia (Tokyo)` を選択
5. 「Create new project」をクリック（作成に1-2分かかる）

### 接続情報の取得

プロジェクト作成後：

1. 左メニュー「Project Settings」（歯車アイコン）
2. 「API」タブを選択
3. 以下をメモ：

- **Project URL**: `https://xxxxxxxx.supabase.co`
- **Publishable key**: `sb_publishable_xxxxxxxx...`（フロントエンド用）
- **Secret key**: `sb_secret_xxxxxxxx...`（クローラー用）

### offersテーブルの作成

1. 左メニュー「SQL Editor」をクリック
2. 「New query」をクリック
3. 以下のSQLを貼り付けて「Run」：

```sql
-- pg_trgm拡張を有効化（部分一致検索用）
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- offersテーブル作成
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

  -- 同一案件の重複防止（日付単位）
  UNIQUE(site_name, url, fetched_date)
);

-- インデックス
CREATE INDEX idx_offers_offer_name ON offers USING GIN (offer_name gin_trgm_ops);
CREATE INDEX idx_offers_fetched_date ON offers(fetched_date DESC);
CREATE INDEX idx_offers_site_name ON offers(site_name);
```

4. 「Success. No rows returned」と表示されればOK

### セキュリティ設定（重要）

**⚠️ 必ず実行してください。RLS未設定のままpublishable keyを公開すると、全データが漏洩・改ざんされる危険があります。**

1. 同じSQL Editorで、以下のSQLを実行：

```sql
-- 1. RLS（Row Level Security）を有効化
-- CREATE TABLE後はデフォルトでOFFなので、必ず有効化する
ALTER TABLE public.offers
  ENABLE ROW LEVEL SECURITY;

-- 2. publicロール（匿名ユーザー）からの全権限を剥奪
-- これにより、RLSポリシー以外のアクセスをブロック
REVOKE ALL ON public.offers FROM public;

-- 3. SELECTポリシー：匿名ユーザーも全件取得可能
-- このプロジェクトは公開情報のみ扱うため、匿名アクセスを許可
CREATE POLICY offers_select_public
  ON public.offers
  FOR SELECT
  TO public
  USING (true);

-- 4. INSERT/UPDATE/DELETEはポリシーなし
-- クローラーはsecret keyを使用するため、RLSをバイパスして実行可能
-- 匿名ユーザーからの書き込みは自動的にブロックされる
```

2. 「Success. No rows returned」と表示されればOK

### セキュリティ確認

1. 左メニュー「Database」→「Security Advisor」をクリック
2. 「Run Advisor」をクリック
3. `offers` テーブルが「OK」と表示されていれば問題なし
4. 警告がある場合は「Fix guide」を参照して修正

### 確認

1. 左メニュー「Table Editor」をクリック
2. `offers` テーブルを選択
3. 右上に「1 RLS policy」と表示されていればRLS有効

---

## 5. 環境変数ファイルの作成

プロジェクトルートに `.env.local` を作成します。

```
# Supabase
NEXT_PUBLIC_SUPABASE_URL=https://xxxxxxxx.supabase.co
NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=sb_publishable_xxxxxxxx...

# OpenAI (クローラー用、Next.jsからは使わない)
OPENAI_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxx
```

**注意**: クローラー用のSecret keyは、GitHub Actionsの環境変数に設定してください（`.env.local`には含めない）。

### 参考 APIキーについて

| キー | 形式 | 用途 | 公開可否 | 注意事項 |
|------|------|------|---------|---------|
| **Publishable key** | `sb_publishable_...` | フロントエンド（ブラウザ）用 | ✅ 公開OK | RLSが正しく設定されていれば安全 |
| **Secret key** | `sb_secret_...` | バックエンド・クローラー用 | ❌ **絶対に公開禁止** | RLSをバイパスする最強権限。漏洩すると全データ操作可能 |

**セキュリティチェック**:
- ✅ RLSが有効になっているか確認（前のセクション参照）
- ✅ `.env.local` は `.gitignore` に含まれているか確認
- ✅ GitHubにコミットしていないか確認

## 6. Vercelアカウントの作成

デプロイ時に必要です。

### 手順

1. [Vercel](https://vercel.com/) にアクセス
2. 「Start Deploying」→ GitHubアカウントでサインイン
3. サインアップ完了

---

## 7. Crawl4AIの動作確認

環境が正しくセットアップされているか確認します。

### 手順

```bash
cd /poitoku-hikaku

# クローラー用ディレクトリ作成
mkdir -p crawler
cd crawler

# Python仮想環境作成
python -m venv venv
source venv/bin/activate

# Crawl4AIインストール
pip install crawl4ai

# Playwright（ブラウザ自動化）のセットアップ
playwright install chromium
```

### 動作確認スクリプト

```bash
# テストスクリプト作成
cat << 'EOF' > test_crawl4ai.py
import asyncio
from crawl4ai import AsyncWebCrawler

async def test():
    async with AsyncWebCrawler() as crawler:
        result = await crawler.arun(url="https://example.com")
        print("Status:", "OK" if result.success else "FAILED")
        print("Title:", result.metadata.get("title", "N/A"))
        print("Content length:", len(result.markdown))

asyncio.run(test())
EOF

# 実行
python test_crawl4ai.py
```

### 期待される出力

```
Status: OK
Title: Example Domain
Content length: 約200-500
```

### クリーンアップ

```bash
# テストファイル削除
rm test_crawl4ai.py

# 仮想環境を抜ける
deactivate
```

---

## チェックリスト

すべて完了したら、以下にチェックを入れてください：

- [x] asdf で Node.js 24.13.0 インストール済み
- [x] asdf で Python 3.13.12 インストール済み
- [x] pnpm インストール済み
- [x] OpenAI APIキー発行済み & 課金設定済み
- [x] Supabaseプロジェクト作成済み
- [x] Supabase offersテーブル作成済み
- [x] **Supabase RLS有効化 & セキュリティポリシー設定済み** ⚠️
- [x] **Security Advisorで警告がないことを確認済み** ⚠️
- [x] `.env.local` ファイル作成済み
- [x] `.env.local` が `.gitignore` に含まれていることを確認済み
- [x] Vercel サインアップ完了
- [x] Crawl4AI動作確認OK

---

## トラブルシューティング

### asdf install でエラーが出る

```bash
# プラグインを最新化
asdf plugin update --all

# 依存関係のインストール（Python）
# macOS の場合
brew install openssl readline sqlite3 xz zlib tcl-tk

# 再度インストール
asdf install
```

### Crawl4AIインストールでエラーが出る

```bash
# 依存関係を先にインストール
pip install --upgrade pip
pip install playwright
playwright install-deps  # システム依存関係のインストール
playwright install chromium
pip install crawl4ai
```

### OpenAI APIで401エラー

- APIキーが正しくコピーされているか確認
- 課金設定が完了しているか確認
- キーの有効期限が切れていないか確認

### Supabaseで接続エラー

- Project URLが `https://` で始まっているか確認
- publishable keyが正しくコピーされているか確認
- プロジェクトが「Active」状態か確認（Paused状態だと接続不可）
- 環境変数名が `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` になっているか確認

### Supabaseでデータが取得できない（RLS関連）

- RLSが有効になっているか確認（Table Editorで右上に「1 RLS policy」と表示されるか）
- SELECTポリシーが作成されているか確認（Database → Policies）
- Security Advisorで警告がないか確認（Database → Security Advisor → Run Advisor）
- エラーメッセージを確認（「permission denied」の場合はRLSポリシーの問題）

### APIキーが漏洩した可能性がある

1. Supabaseダッシュボード → Project Settings → API
2. 「Generate new secret」で新しいキーを生成
3. `.env.local` を更新
4. 古いキーを無効化（または削除）
5. 動作確認を実施
