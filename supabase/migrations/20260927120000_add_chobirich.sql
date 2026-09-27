-- ちょびリッチ（chobirich）をクロール対象に追加する
--
-- id は crawler/sites/chobirich.yaml の id と一致させる。
-- ポイント単価は 1pt = 1円（交換所で 500 ちょびpt = 500 円分、2026-09-27 に確認）なので points_per_yen = 1。
-- 2026-02 の調査メモ（docs/reference/point-sites.md）と初回スキーマの列コメントは「2pt = 1円」だったが、
-- 現在の交換レートと合わないため、列コメントも実態に合わせて書き直す。

insert into public.sites (id, name, url, points_per_yen) values
  ('chobirich', 'ちょびリッチ', 'https://www.chobirich.com/', 1)
on conflict (id) do nothing;

comment on column public.sites.points_per_yen is '1円あたりのポイント数。1pt=1円 のモッピー・ハピタス・ちょびリッチは 1、10pt=1円 のポイントインカムは 10。円換算は reward_points / points_per_yen（current_offers.reward_yen）';
