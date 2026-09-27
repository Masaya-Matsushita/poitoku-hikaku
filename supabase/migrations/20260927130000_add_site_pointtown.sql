-- 対象サイトにポイントタウンを追加する（crawler/sites/pointtown.yaml）
--
-- points_per_yen = 1：交換ページ（https://www.pointtown.com/exchange、2026-09-27 確認）に
-- 「ポイントは1ポイント＝1円相当」とあり、Amazonギフトカードは最低交換額 100 ポイント → 100 円分（手数料無料）、
-- Apple Gift Card は 500 ポイント → 500 円分（手数料無料）。
-- 利用規約・robots.txt の確認結果は docs/reference/point-sites.md。

insert into public.sites (id, name, url, points_per_yen) values
  ('pointtown', 'ポイントタウン', 'https://www.pointtown.com/', 1)
on conflict (id) do nothing;
