# 対象ポイ活サイト一覧

## 概要

本ドキュメントでは、ポイ得比較でクローリング対象となるポイ活サイトの情報をまとめる。

---

## クロール対象サイト（MVP の 2 サイト + ポイントタウン）

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

### 3. ポイントタウン

| 項目 | 内容 |
|------|------|
| サイト名 | ポイントタウン |
| URL | https://www.pointtown.com/ |
| 運営会社 | GMOメディア株式会社（利用規約 第1条、フッター「Point TownはGMOメディア株式会社（東証グロース上場）が運営しています。」） |
| ポイント単価 | 1pt = 1円（下記） |
| 紹介コード | （未取得） |
| クローリング | ✅ 実装（`crawler/sites/pointtown.yaml`）。利用規約の判定はオーナー確認待ち（下記） |

#### 国外 IP からの到達性（2026-09-27、クラウドセッションの crawler-dev 環境から）

`/robots.txt`・`/`・`/help/terms`・`/category`・カテゴリページ・`/item/N`・`/exchange` がすべて 200。ポイントインカムのような地域遮断は無い。
ただし利用規約 第3条5項で「日本国外から本サービスにアクセスする行為について、制限を行うことができる」としている。
日次クロールの GitHub Actions ランナー（国外）が将来遮断される可能性はある（遮断されたら robots.txt の取得で `robots_unavailable` として止まる）。

#### 利用規約（2026-09-27 確認。https://www.pointtown.com/help/terms 、2006-09-27 制定・2023-10-16 最終改定）

「スクレイピング」「クローリング」「自動取得」の語は無く、**掲載情報の収集・利用を禁じる条項も無い**（ECナビ 第11条4項のような条項が無い）。
権利帰属（第16条）と禁止事項（第17条）の全文を読んだ上で、関係しうる条項は次のとおり：

- 第16条1項（権利帰属）：「文章、画像、プログラムなど本サービスを構成するデータについての知的財産権は、全て当社または当社にライセンスを許諾している者に帰属しており、…会員は、本サービスの利用に必要な範囲において、これらの知的財産権を使用するものとします。」
  → 知的財産権の帰属の宣言。保存するのは案件名・還元額・URL・カテゴリ（事実情報）のみで、説明文や画像は取らない（AGENTS.md）
- 第17条(4)：「本サービスのネットワークまたはシステム等に過度な負担をかける行為。」→ 1 日 1 回・3 秒間隔（下記の実測）で該当しない想定
- 第17条(5)：「BOT、チートツール、その他の技術的手段を利用してポイントを取得または改ざんする行為。」→ ポイントの取得・改ざんの話。公開一覧の閲覧は当たらない。ポイント獲得の遷移用 URL（`/item/redirect/`、robots.txt で禁止）には触れない
- 第17条(7)：「当社の承諾なく本サービスを利用して商業活動をする行為、またはその準備を目的とする行為。」→ **判断が要る条項**。モッピー 第17条①「営利目的の利用」と同じく「会員としてのサービス利用」を指す読みが自然（`docs/06-legal.md`、モッピーはオーナーがそう判断）。ただし紹介報酬を得る比較サイトは「本サービスを利用した商業活動」と読まれる余地がある
- 第17条(14)(17)：「他の会員、第三者もしくは当社に不利益、損害、不快感を与える行為。」「その他、当社が不適切と判断する行為。」→ 運営の裁量条項（ハピタスと同種）。節度が全て
- 第3条5項：「本サービスは、日本国内向けのサービスです。当社は、日本国外から本サービスにアクセスする行為について、制限を行うことができるものとします。」→ 上記の到達性を参照

**判定**: ⚠ 明示の禁止条項は無い（ECナビ型の「掲載情報の無断収集」禁止は無い）ので実装した。第17条(7) の読み方はオーナーの確認事項（追加 PR の冒頭に引用）。

#### robots.txt（2026-09-27 取得。`crawler/testdata/pointtown/robots.txt`）

`User-agent: AdsBot-Google` のグループは `/category/`・`/item/` 等を禁止しているが、Google の広告審査ボット向けで、このクローラーは `User-agent: *` のグループに従う。

```
User-agent: *
Disallow: /accountLogin/
Disallow: /accountLogin
Disallow: /login/
Disallow: /login
Disallow: /item/redirect/
Disallow: /item/redirect
Disallow: /game/redirect/
Disallow: /game/redirect
Disallow: /exchange/gift/kumapon
Disallow: /exchange/gift/paypay-money-light/callback
Disallow: /vod/
Disallow: /ptu/
Sitemap: https://www.pointtown.com/sitemap
Sitemap: https://www.pointtown.com/articles/sitemap_index.xml
```

**判定**: ✅ クローリング可能（`/category` と `/category/*`、`/item/N` は `*` に対して許可。`/item/redirect/` は禁止なので取得しない。クローラーが起動時に取得・検証する。`User-agent: *` 以外のグループを使わないことは `crawl_test.go` で確認）

#### ポイント単価（2026-09-27、`/exchange`）

交換ページに「ポイントは1ポイント＝1円相当」。実データでも Amazonギフトカードは最低交換額 100 ポイント → 100 円分（手数料無料）、Apple Gift Card は 500 ポイント → 500 円分（手数料無料）。
現金（銀行振込）は 550 ポイント → 500 円（手数料 50 ポイント）。`sites.points_per_yen = 1`。

#### クローリング情報（2026-09-27 検証。実装は `crawler/sites/pointtown.yaml`）

| 項目 | 内容 |
|------|------|
| 取得方法 | カテゴリページ（サーバー側描画）をパース。個別ページは巡回しない |
| カテゴリ発見 | `/category` のリンクから `/category/<shopping|service>/<slug>` を 26 件（ショッピング 15・サービス 11）。同じ URL の画像リンク（`.u-expand-link`、テキスト無し）が先に並ぶので除く。`beauty`・`other` は両グループにあるので group と slug の組で区別 |
| 一覧取得 | `/category/<group>/<slug>/<N>`。20 件/ページ（`/1` は 1 ページ目と同じ）。最終ページ番号は `.c-pager__nav a` のテキストの最大値。範囲外のページは 404。総件数は見出しの「全N件」（`.c-sec__ttl small`） |
| JS実行 | 不要（「次の20件を見る」は JS だが、番号リンクは通常のリンク） |
| 案件要素 | `.l-column__main .l-card > .l-card__item`。案件名は `.c-card__ttl a` のテキストが長いと「…」で省略されるため、サムネイル `img` の `alt`（全文）から取る（`name_attr`）。還元額は `.c-af-incentive__point`（"12,000" または "3%"）。固定額の単位 pt は CSS で描かれ HTML に無いので `reward_unit: pt` で付けて記録する。ポイントアップ中の案件は元の額（`.c-af-incentive__point-origin`）も並ぶが、今の額だけ取る |
| 詳細URL | `https://www.pointtown.com/item/N`（追跡パラメータ無し） |
| 実測（2026-09-27 dry-run） | 26 カテゴリ・99 ページ、**101 リクエスト**（robots.txt・`/category` を含む）、**5 分 0 秒**（3 秒間隔）。重複排除後 1,622 件、還元額の数値化 1,622 件（100%）、エラー 0、表示上限の取りこぼし 0（全カテゴリで「全N件」に到達）。還元 0 の案件は "0" と表示され（UQモバイル等）、`0pt` → 0 ポイントとして記録される |

---

## 将来拡張候補

### 3. ポイントインカム

| 項目 | 内容 |
|------|------|
| サイト名 | ポイントインカム |
| URL | https://pointi.jp/ |
| 運営会社 | 株式会社セレス（モッピーと同じ。遮断ページの著作権表記「Copyright 2006 CERES INC.」と、規約 PDF `sp.pointi.jp/pdf/agreement.pdf` の検索結果の冒頭「株式会社セレス（以下、当社）が提供する…」から。2026-02 の記録はファイブゲート株式会社） |
| ポイント単価 | 10pt = 1円（2026-02 の記録。2026-09-27 時点で未再検証） |
| 紹介コード | （未取得） |
| クローリング | **保留**（国外 IP からは全 URL が遮断され、規約・robots.txt を確認できない。下記） |

#### 2026-09-27 の調査結果：国外 IP からのアクセス遮断

クラウドセッション（crawler-dev 環境）から取得を試みた結果。モッピー・ハピタスの robots.txt は同じ環境から 200 で取れる。

| URL | 結果 |
|------|------|
| `https://pointi.jp/robots.txt` | 302 → `https://pointi.jp/information.php?cn=2&sn=1` |
| `https://pointi.jp/`、`https://pointi.jp/contents/rule/` | 同上 |
| 遷移先 `information.php?cn=2&sn=1` | 200。「アクセス制御に関するお知らせ　このページはお住まいの地域からご利用になれません　大変申し訳ございませんが、Point Incomeは国内のみご利用可能となります。」 |
| `https://sp.pointi.jp/pdf/agreement.pdf`（規約 PDF） | 取得不可（crawler-dev 環境のネットワーク許可に `sp.pointi.jp` が無く、egress プロキシが 403。許可しても本体と同じ地域制限の可能性がある） |

| 調査項目 | 結果 |
|------|------|
| 利用規約のスクレイピング禁止条項 | **未確認**（本文を取得できない。Web 検索の二次情報は「複数端末での同時プレイ禁止」「家族間のアカウント共有禁止」程度で、自動取得に関する条項の有無は判断できない） |
| robots.txt の許可範囲 | **未確認**（robots.txt 自体が遮断ページへ 302） |
| ポイント単価 | 未再検証（10pt = 1円 は 2026-02 の記録） |
| 紹介制度 | 未確認 |
| 1 回の実行のリクエスト数・所要時間 | 実測不可 |

**判定**: ⏸ 保留。規約と robots.txt を確認できないので実装しない。加えて日次クロールは GitHub Actions の `ubuntu-latest`（GitHub ホストのランナー。国外のデータセンター）で動くため、実装しても本番クロールが同じ遮断を受ける見込み（推測。ランナーからは未検証）。
地域制限を VPN・国内プロキシで回避するのはアクセス制御の回避であり、「運営を不快にさせない」線（`docs/lessons.md`）を越えるのでやらない。

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

クロール対象に追加した（2026-09-27）。上の「3. ポイントタウン」を参照。

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

**クロール対象のモッピー・ハピタス・ポイントタウンはいずれも 1pt = 1円 なので換算不要。**

---

## クローリング優先度

| 優先度 | サイト | 理由 |
|-------|--------|------|
| 1 | モッピー | 最大手、HTMLにデータ埋め込み、取得しやすい |
| 2 | ハピタス | 大手、メタデータから取得可能 |
| — | ポイントタウン | 実装済み（2026-09-27）。規約 第17条(7) の読み方はオーナー確認 |
| 3 | ポイントインカム | 大手。国外 IP を遮断しており、GitHub Actions からは取得できない見込み（2026-09-27、保留） |
| 4 | ECナビ | 大手、要調査 |
| 5 | ちょびリッチ | 中堅、要調査 |

---

## 追加時の調査項目

新しいサイトを追加する際は、以下を調査する：

1. **robots.txt**: クローリング許可範囲
2. **利用規約**: スクレイピング禁止条項の有無
3. **データ取得方法**: HTML埋め込みかJS動的ロードか
4. **ポイント単価**: 円換算レート
5. **紹介制度**: 紹介コード/URLの取得
6. **案件一覧の取得方法**: 検索ページ、カテゴリページ等
7. **国外 IP からの到達性**: 日次クロールは GitHub Actions（国外）で動く。robots.txt が 200 で返るかを最初に確かめる（ポイントインカムは国外 IP を遮断していた。2026-09-27）
