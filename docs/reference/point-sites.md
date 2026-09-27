# 対象ポイ活サイト一覧

## 概要

本ドキュメントでは、ポイ得比較でクローリング対象となるポイ活サイトの情報をまとめる。

---

## MVP対象サイト（2サイト）

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

---

## 将来拡張候補

### 3. ポイントインカム

| 項目 | 内容 |
|------|------|
| サイト名 | ポイントインカム |
| URL | https://pointi.jp/ |
| 運営会社 | ファイブゲート株式会社 |
| ポイント単価 | 10pt = 1円 |
| 紹介コード | （未取得） |
| クローリング | 要調査 |

### 4. げん玉

| 項目 | 内容 |
|------|------|
| サイト名 | げん玉 |
| URL | https://www.gendama.jp/ |
| 運営会社 | 株式会社リアルX |
| ポイント単価 | 10pt = 1円 |
| 紹介コード | （未取得） |
| クローリング | 要調査 |

### 5. ちょびリッチ

| 項目 | 内容 |
|------|------|
| サイト名 | ちょびリッチ |
| URL | https://www.chobirich.com/ |
| 運営会社 | 株式会社ちょびリッチ |
| ポイント単価 | 2pt = 1円 |
| 紹介コード | （未取得） |
| クローリング | 要調査 |

### 6. ECナビ — ❌ 対象外（利用規約が掲載情報の無断収集を禁止。2026-09-27 確認）

| 項目 | 内容 |
|------|------|
| サイト名 | ECナビ |
| URL | https://ecnavi.jp/ |
| 運営会社 | 株式会社DIGITALIO |
| ポイント単価 | 10pt = 1円（ヘルプ「何ポイントからポイント交換できますか？」：ECナビ 1pt = PeX 1pt。PeX は 10pt = 1円） |
| 紹介コード | （未取得） |
| クローリング | **実装しない**（下記の利用規約 第11条4項） |

#### 利用規約（2026-09-27 確認。https://ecnavi.jp/terms/ 、2009-10-15 制定・2026-02-16 最終改訂）

「スクレイピング」「クローリング」の語は無いが、第11条（権利帰属）第4項が掲載情報の収集そのものを禁じている：

> 本サービスに掲載しているショップ登録情報、商品価格、投稿データ、キャンペーン情報、懸賞情報などの無断収集・利用・販売・再販・営業を禁止します。

ポイ得比較が保存する「案件名・還元額・URL・カテゴリ」はショップ登録情報・キャンペーン情報そのもので、比較表示はその「利用」にあたる。
モッピー（直接の禁止条項なし）・ハピタス（広い裁量条項のみ）と違い、明示の禁止なので**頻度や節度で回避できる性質のものではない**。
追加するにはDIGITALIOから許諾を得る必要がある（許諾の取得はオーナーの判断）。

関連する他の条項（参考）：

- 第9条（禁止事項）：「本サービスを複製、改変、翻案等する行為」「本サービスのネットワークまたはシステム等に過度な負荷をかける行為」「本サービスの運営を妨害するおそれのある行為」「その他、当社が不適切と判断する行為」
- 第9条の柱書は「該当すると当社が判断する行為」まで含む（運営の裁量が広い）
- 不正行為ポリシー（`/illegal_policy/`）：「常識の範囲を超えて予想し得ない手段（プログラムを利用する等）を用いてポイントを取得するもの」を不正とする。ポイント取得の話なので一覧の閲覧には直接当たらないが、違反時は保有ポイント全失効

#### robots.txt（2026-09-27 取得）

```
User-agent: *
Disallow: /research/chinju_lesson/search/
Disallow: /contents/divination/omikuji/result/
Disallow: /contents/divination/tarot/result/
Disallow: /contents/divination/western_astrology/result/
Disallow: /contents/divination/western_astrology/ranking/
Allow: /

Sitemap: https://ecnavi.jp/sitemap.php
```

robots.txt 上は案件ページを含めほぼ全体が許可。**robots.txt の許可は利用規約の禁止を上書きしない**ので、判定は規約で「対象外」。

観測メモ：2026-09-27 時点で CloudFront の一部エッジ（`IAD61`）が `/robots.txt` に対して自分自身への 301（`Location: https://ecnavi.jp:443/robots.txt`、`Server: awselb/2.0` のキャッシュ）を返し、リダイレクトが無限ループした。
別エッジ（`CDG50`・`JFK52`）とクエリ付きの URL は 200。将来許諾を得て実装する場合、GitHub Actions のランナー（米国）から robots.txt が取れず起動時検証で止まる可能性がある。

#### 紹介制度（2026-09-27、`/contents/invite/`）

| 項目 | 内容 |
|------|------|
| 登録ボーナス | 紹介した友達が登録後 90 日以内に本人認証を完了すると、双方に 1,500pt（= 150円） |
| 紹介ボーナス | 友達がサービス・ショッピング・モニター・旅行予約で獲得したポイントの 5〜100%（案件ごとに詳細ページに記載） |
| キャンペーン（2026-09） | 友達が登録翌月末までにポイント交換すると紹介者に 5,000pt、友達に Amazon ギフト 1,000円分 |
| 掲載ルール | `/contents/invite/blog/`（ブログでの紹介方法）はログイン必須で未確認

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
| ちょびリッチ | 2pt = 1円 | 5,000円 |
| ECナビ | 10pt = 1円 | 1,000円 |
| ワラウ | 1pt = 1円 | 10,000円 |
| ポイントタウン | 1pt = 1円 | 10,000円 |

**MVP対象のモッピー・ハピタスは両方とも 1pt = 1円 なので換算不要。**

---

## クローリング優先度

| 優先度 | サイト | 理由 |
|-------|--------|------|
| 1 | モッピー | 最大手、HTMLにデータ埋め込み、取得しやすい |
| 2 | ハピタス | 大手、メタデータから取得可能 |
| 3 | ポイントインカム | 大手、要調査 |
| — | ECナビ | 対象外。利用規約 第11条4項が掲載情報の無断収集を禁止（2026-09-27） |
| 5 | ちょびリッチ | 中堅、要調査 |

---

## 追加時の調査項目

新しいサイトを追加する際は、以下を調査する：

1. **robots.txt**: クローリング許可範囲
2. **利用規約**: スクレイピング禁止条項の有無。「スクレイピング」の語が無くても「掲載情報の無断収集」を禁じる条項（ECナビ 第11条4項）は禁止条項として扱う。robots.txt の許可は規約の禁止を上書きしない
3. **データ取得方法**: HTML埋め込みかJS動的ロードか
4. **ポイント単価**: 円換算レート
5. **紹介制度**: 紹介コード/URLの取得
6. **案件一覧の取得方法**: 検索ページ、カテゴリページ等
