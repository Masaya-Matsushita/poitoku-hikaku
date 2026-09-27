# 対象ポイ活サイト一覧

## 概要

本ドキュメントでは、ポイ得比較でクローリング対象となるポイ活サイトの情報をまとめる。

---

## 対象サイト（3サイト。MVP はモッピー・ハピタス、ちょびリッチは 2026-09-27 追加）

### 1. モッピー（Moppy）

| 項目 | 内容 |
|------|------|
| サイト名 | モッピー |
| URL | https://pc.moppy.jp/ |
| 運営会社 | 株式会社セレス |
| 会員数 | 1,000万人以上 |
| ポイント単価 | 1P = 1円 |

#### 紹介情報

| 項目 | 内容 |
|------|------|
| 紹介コード | `6v5NA1ab` |
| 紹介URL | https://pc.moppy.jp/entry/invite.php?invite=6v5NA1ab |
| 紹介報酬（紹介者） | 300P + 紹介相手の獲得ポイントの最大10% |
| 紹介報酬（被紹介者） | 2,000P（ミッションクリア時） |

#### クローリング情報（2026-09-22 再検証。実装は `crawler/sites/moppy.yaml`）

| 項目 | 内容 |
|------|------|
| 取得方法 | カテゴリ一覧の AJAX 断片をパース（検索結果は使わない。全カテゴリを巡回できる） |
| カテゴリ発見 | `GET /ajax/category/get_menu.php` → 「広告ジャンルで探す」の親子カテゴリ 93 件（`/category/list.php?parent_category=N&child_category=N`）。目的別カテゴリ 11 件はジャンルと重複するので対象外 |
| 一覧取得 | `GET /ajax/category/get_list.php?parent_category=N&child_category=N&objective_category=0&current_page=N&af_sorter=1&exclude_purchased=`。30 件/ページ、最終ページ番号は `.a-pagination__list a[current]` の最大値。範囲外のページは 0 件を返す |
| 必要ヘッダ | `X-Requested-With: XMLHttpRequest`（無いと 200 で空応答）。Cookie は不要 |
| JS実行 | 不要（断片はサーバー側で描画済み） |
| 案件要素 | `li.m-list__item > a.block__link[href]`、案件名 `h3.a-list__item__title`、還元額 `em.a-list__item__point`（"10,000P" または "1.0%"）。還元が無い案件は `.a-list__item__point` が無く `p.a-list__item__benefit` に「ポイント対象外」と出る（2026-09-22 時点で 10 件。0 ポイントとして扱う）。同じ要素に物品プレゼントの文言（「ハーゲンダッツ２個」等、保険の一括見積・資料請求の 6 件）が出る案件は、文言を `reward_raw` に残し数値は null（`docs/02-kpi.md`「抽出精度の読み方」） |
| 詳細URL | `/ad/detail.php?site_id=N`（ショッピング系は `/shopping/detail.php?site_id=N`）。`track_ref` 等の追跡パラメータは落とし、特集枠の `s_id` は同じ ID なので `site_id` に寄せる |
| 旧URL | `/ad/?c_id=N`、`/ad/?m_id=N`（sitemap.xml に残る 2010 年の URL）はランキングやトップへリダイレクトされ、使えない。カテゴリページ `/category/list.php` 本体は一覧を JS で読むため、断片を直接取る |

#### robots.txt（2026-09-22 取得。`crawler/testdata/moppy/robots.txt`）

```
User-agent: *
Disallow: /notfound.php
Disallow: /error.php
Disallow: /work.php
Disallow: /ad/j.php
Disallow: /ad/r.php
Disallow: /receive/
Disallow: /img/
Allow: /

Sitemap: https://pc.moppy.jp/sitemap.xml
```

**判定**: ✅ クローリング可能（`/ajax/category/*` と `/ad/detail.php` は許可。クローラーが起動時に取得・検証する）

---

### 2. ハピタス（Hapitas）

| 項目 | 内容 |
|------|------|
| サイト名 | ハピタス |
| URL | https://hapitas.jp/ |
| 運営会社 | 株式会社オズビジョン |
| 会員数 | 500万人以上 |
| ポイント単価 | 1pt = 1円 |

#### 紹介情報

| 項目 | 内容 |
|------|------|
| 紹介コード | `IGVWXW` |
| 紹介URL | https://hapitas.jp/appinvite?i=24798796&route=pcText |
| 紹介報酬（紹介者） | 最大300pt + 紹介相手の獲得ポイントの1%〜40% |
| 紹介報酬（被紹介者） | 最大2,700pt（キャンペーン期間中） |

#### クローリング情報（2026-09-23 再検証。実装は `crawler/sites/hapitas.yaml`）

| 項目 | 内容 |
|------|------|
| 取得方法 | カテゴリページ（サーバー側描画）をパース。個別ページは巡回しない（2026-02 の案は不要になった） |
| カテゴリ発見 | カテゴリページのヘッダーナビ `a.menu_item_link` から `/category/<slug>/` を 39 件（サイトマップの `sitemap-category.xml.gz` の 40 件から `newest` を除いた集合と同じ） |
| 一覧取得 | `/category/<slug>/`（人気順）と `/category/<slug>/itemtype/newest/sort/<point|registdate|low_rate_point>/`（高ポイント順・新着順・高還元率順）。1 ページ最大 120 件（サイトの limit）。総件数は `#total_item_count[value]` |
| 「もっと見る」 | JS が `/item/ajaxcategoryitems` を呼ぶ仕組み。直接呼ぶと数回で 404 を返すようになったため**使わない**。並び順 4 種の和集合で最大 480 件/カテゴリを網羅する。ジャンル系は最大 333 件（2026-09）なのでほぼ全件、横断系（すぐ獲得 671 件、アプリ 634 件、ポイントアップ中 612 件、無料獲得 602 件）は上位のみ。総件数に届かないカテゴリは crawl 実行ログに「表示上限による取りこぼし」と出る |
| JS実行 | 不要 |
| 案件要素 | `#inner_catalog .item_slots_thumb > a.thumb_slots_link[href]`、案件名 `p.store`、還元額 `p.caption`（"1,100pt" または "1%"）。ピックアップ枠（`/apn/attention_word` 等）は `#inner_catalog` の外なので対象外 |
| 詳細URL | `https://hapitas.jp/item/detail/itemid/N/apn/...`。末尾の `/apn/...` は追跡用なので落として `/item/detail/itemid/N/` に正規化 |
| 還元 0 の案件 | Amazon の特集枠など（総合通販 82 件中 25 件）。`.caption` が無く、リンク先が `/item/redirect-to-client-if-zero-point-item/itemid/N/apn/`（robots.txt で禁止の遷移用 URL。取得しない）。URL は同じ itemid の詳細 URL に寄せ、還元額は「ポイント対象外」（0 ポイント）として記録 |
| 並び順 | 人気順 recommend / 新着順 registdate / 高ポイント順 point / 高還元率順 low_rate_point（昇順は無い） |

#### robots.txt（2026-09-23 取得。`crawler/testdata/hapitas/robots.txt`）

```
User-Agent: *
Allow: /auth/signin
Disallow: /csenquetelight/
Disallow: /index/ajax*
Disallow: /profile/show/member/*
Disallow: /auth/*
Disallow: /item/redirect/*
Disallow: /item/redirect-to-client-if-zero-point-item/*
Sitemap: https://hapitas.jp/published-assets/auto-generated/sitemap/sitemap.xml
```

**判定**: ✅ クローリング可能（`/category/*` と `/item/detail/*` は許可。`/index/ajax*` は禁止なので触らない。クローラーが起動時に取得・検証する）

#### 2026-02 の調査メモからの変更点

- 「個別ページの `<title>` から還元額を取る」方式は不要。カテゴリページに案件名・還元額・URL が揃っている
- 「主要 100 案件のみ」の MVP 方針は撤回。全 39 カテゴリを巡回して約 80 リクエスト/日（3 秒間隔で約 4 分）

### 3. ちょびリッチ（Chobirich）

| 項目 | 内容 |
|------|------|
| サイト名 | ちょびリッチ |
| URL | https://www.chobirich.com/ |
| 運営会社 | 株式会社ちょびリッチ |
| ポイント単価 | **1pt = 1円**（2026-09-27、交換所で「500 ちょびpt → 500 円分」。2026-02 のメモの「2pt = 1円」は古い）。`sites.points_per_yen = 1` |

#### 利用規約（2026-09-27 確認。https://www.chobirich.com/info/article 、令和4年7月29日改訂版）

**判定**: ✅ スクレイピング・クローリング・自動取得を直接禁じる条項は**なし**（「スクレイピング」「クロール」「ロボット」「自動」「機械的」のいずれの語も規約に無い）。モッピーと同型の広い裁量条項があるので、節度（1 日 1 回・3 秒間隔・robots 遵守・公開ページのみ）が全て。

関係する条項：

| 条項 | 内容（引用） | 判断 |
|------|------|------|
| 第13条（禁止事項）① | 「本サービスを不正の目的をもって利用したり、営利を目的とした利用をすること。」 | 会員としてのサービス利用（ポイント獲得）を指す読みが自然。公開一覧の閲覧は範囲外と判断（モッピー第17条①と同じ扱い）。**オーナー確認事項** |
| 第13条（禁止事項）⑨ | 「上記各号の他、法令、この規約もしくは公序良俗に反する行為、本サービスの運営を妨害する行為、当社の信用を毀損し、もしくは当社の財産を侵害する行為、または他者もしくは当社に不利益を与える行為。」 | 1 日 1 回・約 150 リクエスト・3 秒間隔なら運営の妨害には当たらない想定 |
| 第15条（ユーザー資格の停止、取消） | 「その他当社がユーザーとして不適当と判断した場合。」ほか。事前通知なしで停止・取消できる | 最大リスクはアカウント停止（`docs/06-legal.md`）。モッピー・ハピタスと同じ |
| 第19条（著作権等） | 「本サービスを構成する画面及び本サービスに関する著作権は当社その他の権利者に帰属しており、これを複製、頒布、譲渡、貸与、翻訳、使用許諾、転載、商品化、再利用等する行為は法律及び著作権に関する条約により禁じられています。」 | 保存・表示は案件名・還元額・URL・カテゴリのみ（AGENTS.md）。画像・説明文・口コミは取らない |
| 第17条（当プログラムで提供される情報について） | 提携サイト等の情報の正確性等を保証しない | 当サイト側の「還元額は取得時点のもの」注記で対応（モッピー第20条と同じ） |

#### 紹介情報（2026-09-27、https://www.chobirich.com/introduction/ ）

| 項目 | 内容 |
|------|------|
| 紹介制度 | **あり**。紹介 URL（末尾に会員番号）または紹介コード（有効期限なし）。特典は同じ |
| 紹介コード | （未取得。オーナーのアカウントで発行する） |
| 紹介報酬（紹介者） | 新規登録特典 50pt（友達が登録月内に 1pt 以上獲得）＋ 初めて広告利用特典 150pt（登録後 30 日以内に初回広告利用）＋ ダウン報酬 最大 50%（広告ごとに 1〜50%。**最後に紹介した友達の登録日が 1 年以内の人のみ**）＋ 今日のポイントプレゼントおすそ分け 10% |
| 紹介報酬（被紹介者） | ウェルカムボーナス 150pt ＋ スタートダッシュボーナス 最大 2,000pt（翌月末までの獲得ポイントで変動） |
| 掲載ルール | Web 掲載を禁じる記載なし。NG は「公序良俗に反するサイト・方法での紹介」「弊社が不正による紹介・登録と判断した場合」。紹介用素材（バナー）は直リンク・加工・印刷物での使用等が禁止 |

ダウン報酬は「1 年以内に誰かを紹介していること」が条件なので、紹介が途切れると受け取れなくなる（モッピー・ハピタスには無い条件）。

#### クローリング情報（2026-09-27 検証。実装は `crawler/sites/chobirich.yaml`）

| 項目 | 内容 |
|------|------|
| 取得方法 | カテゴリ一覧ページ（サーバー側描画）をページ送りでパース。個別ページは巡回しない |
| カテゴリ発見 | トップページ `/` のサイドメニュー `a.SideColCateMenu__link--category` から 20 件：「お買い物」`/shopping/shop/101〜111`（11 件）＋「サービス」`/earn/apply/101,103,104,106〜111`（9 件）。フッターにだけ出る `/earn/apply/102`・`105`、`/shopping/shop/112` は 0 件の旧カテゴリなので対象外 |
| 一覧取得 | `/shopping/shop/<N>?page=<P>` または `/earn/apply/<N>?page=<P>`。30 件/ページ、並び順は既定（おすすめ順）。最終ページ番号は `.com-pagination__list a` のテキストの最大値 |
| JS実行 | 不要。Cookie・特殊ヘッダも不要 |
| 案件要素 | `li.ad-category__ad`、案件名 `.ad-category__ad__name--text`、還元額 `.ad-category__ad__pt`（"2,000pt" / "1.5%" / "1.5％"）、詳細 URL は最初の `a[href*="/ad_details/"]`（2 本目は口コミ `#shopping_rate`） |
| 還元アップ中 | `<s>3,000pt</s>→3,500pt` と旧額を取り消し線で併記。`reward_exclude_selector: s` で旧額を除き「→3,500pt」を記録（数値化は 3,500） |
| 還元 0 の案件 | Amazon・ANAのふるさと納税などは `.ad-category__ad__pt` が空（→「ポイント対象外」）、ヨリヤス等は "0"（→ 0 ポイント） |
| 詳細URL | `/ad_details/N` と `/ad_details/N/` が混在するので `/ad_details/N/` に正規化 |
| 規模（dry-run 実測） | 2026-09-27 の dry-run：**107 リクエスト**（robots 1 + トップ 1 + 一覧 105 ページ）、**所要 5 分 19 秒**（3 秒間隔）、案件 **2,848 件**、数値化率 99.93%（2 件：単位の無い "2,000" と "-"）、エラー 0。全 20 カテゴリで最終ページ（最大 13 ページ）まで到達し取りこぼし無し |

#### robots.txt（2026-09-27 取得。`crawler/testdata/chobirich/robots.txt`）

```
User-Agent: *
Sitemap: https://www.chobirich.com/sitemap.xml
Allow: /ads.txt

User-Agent: *
Disallow: /regist/
Disallow: /reminder/
Disallow: /account/
Disallow: /openid/
Disallow: /member/
Disallow: /ad_details/56546/
Disallow: /ad_details/redirect/
Disallow: /ad_details/forword_external_site/
Disallow: /*utm_
Disallow: /cm/ad/
Disallow: /cm/om/
Disallow: /cm/en/
```

**判定**: ✅ クローリング可能（`/`、`/shopping/shop/*`、`/earn/apply/*` は許可。遷移用の `/ad_details/redirect/` と会員ページは禁止で、どちらも取得しない）。

`User-Agent: *` のグループが 2 つに分かれている。RFC 9309 では同じ UA のグループは結合して解釈するが、2026-09-27 まで `crawler/internal/robots` は最初のグループだけを見ていたため、このままだと Disallow が全部無視されるところだった（同 PR で結合するよう修正）。

---

## 将来拡張候補

### 4. ポイントインカム

| 項目 | 内容 |
|------|------|
| サイト名 | ポイントインカム |
| URL | https://pointi.jp/ |
| 運営会社 | ファイブゲート株式会社 |
| ポイント単価 | 10pt = 1円 |
| 紹介コード | （未取得） |
| クローリング | 要調査 |

### 5. げん玉

| 項目 | 内容 |
|------|------|
| サイト名 | げん玉 |
| URL | https://www.gendama.jp/ |
| 運営会社 | 株式会社リアルX |
| ポイント単価 | 10pt = 1円 |
| 紹介コード | （未取得） |
| クローリング | 要調査 |

### 6. ECナビ

| 項目 | 内容 |
|------|------|
| サイト名 | ECナビ |
| URL | https://ecnavi.jp/ |
| 運営会社 | 株式会社DIGITALIO |
| ポイント単価 | 10pt = 1円 |
| 紹介コード | （未取得） |
| クローリング | 要調査 |

### 7. ワラウ

| 項目 | 内容 |
|------|------|
| サイト名 | ワラウ |
| URL | https://www.warau.jp/ |
| 運営会社 | 株式会社オープンスマイル |
| ポイント単価 | 1pt = 1円 |
| 紹介コード | （未取得） |
| クローリング | 要調査 |

### 8. ポイントタウン

| 項目 | 内容 |
|------|------|
| サイト名 | ポイントタウン |
| URL | https://www.pointtown.com/ |
| 運営会社 | GMOメディア株式会社 |
| ポイント単価 | 1pt = 1円 |
| 紹介コード | （未取得） |
| クローリング | 要調査 |

---

## ポイント単価まとめ

サイトによってポイント単価が異なるため、円換算時に注意が必要。

| サイト | ポイント単価 | 10,000pt = |
|--------|------------|-----------|
| モッピー | 1P = 1円 | 10,000円 |
| ハピタス | 1pt = 1円 | 10,000円 |
| ポイントインカム | 10pt = 1円 | 1,000円 |
| げん玉 | 10pt = 1円 | 1,000円 |
| ちょびリッチ | 1pt = 1円（2026-09 確認。旧 2pt = 1円） | 10,000円 |
| ECナビ | 10pt = 1円 | 1,000円 |
| ワラウ | 1pt = 1円 | 10,000円 |
| ポイントタウン | 1pt = 1円 | 10,000円 |

**対象のモッピー・ハピタス・ちょびリッチはいずれも 1pt = 1円 なので換算不要。**

---

## クローリング優先度

| 優先度 | サイト | 理由 |
|-------|--------|------|
| 1 | モッピー | 最大手、HTMLにデータ埋め込み、取得しやすい |
| 2 | ハピタス | 大手、メタデータから取得可能 |
| 4 | ポイントインカム | 大手、要調査 |
| 5 | ECナビ | 大手、要調査 |
| 3 | ちょびリッチ | 2026-09-27 追加（オーナー指定） |

---

## 追加時の調査項目

新しいサイトを追加する際は、以下を調査する：

1. **robots.txt**: クローリング許可範囲
2. **利用規約**: スクレイピング禁止条項の有無
3. **データ取得方法**: HTML埋め込みかJS動的ロードか
4. **ポイント単価**: 円換算レート
5. **紹介制度**: 紹介コード/URLの取得
6. **案件一覧の取得方法**: 検索ページ、カテゴリページ等
