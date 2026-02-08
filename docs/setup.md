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
asdf install nodejs 22.22.0
asdf install python 3.13.12
```

### 確認

```bash
# プロジェクトディレクトリで実行
node --version
# 期待値: v22.22.0

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
# 期待値: 9.x.x 以上
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

### 確認

```bash
# 環境変数に一時的にセット
export OPENAI_API_KEY="sk-xxxxxxxxxxxxxxxxxxxxxxxx"

# APIが動作するか確認
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  | head -c 200

# 期待値: {"object":"list","data":[... のようなJSON
```

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
   - **anon public key**: `eyJxxxxxxxx...`（長い文字列）

### offersテーブルの作成

1. 左メニュー「SQL Editor」をクリック
2. 「New query」をクリック
3. 以下のSQLを貼り付けて「Run」：

```sql
-- offersテーブル作成
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

-- インデックス
CREATE INDEX idx_offers_offer_name ON offers(offer_name);
CREATE INDEX idx_offers_fetched_at ON offers(fetched_at DESC);
CREATE INDEX idx_offers_site_name ON offers(site_name);
```

4. 「Success. No rows returned」と表示されればOK

### 確認

1. 左メニュー「Table Editor」をクリック
2. `offers` テーブルが表示されていればOK

---

## 5. 環境変数ファイルの作成

プロジェクトルートに `.env.local` を作成します。

### 手順

```bash
cd /poitoku-hikaku

# 環境変数ファイル作成
cat << 'EOF' > .env.local
# Supabase
NEXT_PUBLIC_SUPABASE_URL=https://xxxxxxxx.supabase.co
NEXT_PUBLIC_SUPABASE_ANON_KEY=eyJxxxxxxxx...

# OpenAI (クローラー用、Next.jsからは使わない)
OPENAI_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxx
EOF
```

### 実際の値に置き換え

```bash
# エディタで開いて、手順3, 4で取得した値に置き換える
code .env.local  # VS Codeの場合
```

### .gitignoreの確認

`.env.local` がGitにコミットされないことを確認：

```bash
# .gitignoreに追記（まだなければ）
echo ".env.local" >> .gitignore
echo ".env" >> .gitignore
```

---

## 6. Vercelアカウントの作成（任意）

デプロイ時に必要です。

### 手順

1. [Vercel](https://vercel.com/) にアクセス
2. 「Start Deploying」→ GitHubアカウントでサインイン
3. サインアップ完了

---

## 7. Crawl4AIの動作確認（任意だが推奨）

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

- [ ] asdf で Node.js 22.22.0 インストール済み
- [ ] asdf で Python 3.13.12 インストール済み
- [ ] pnpm インストール済み
- [ ] OpenAI APIキー発行済み & 課金設定済み
- [ ] Supabaseプロジェクト作成済み
- [ ] Supabase offersテーブル作成済み
- [ ] `.env.local` ファイル作成済み（実際の値を設定）
- [ ] `.gitignore` に `.env.local` 追加済み
- [ ] （任意）Crawl4AI動作確認OK

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
- anon keyが正しくコピーされているか確認
- プロジェクトが「Active」状態か確認（Paused状態だと接続不可）
