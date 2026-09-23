-- offer_snapshots を区間方式に変更する（ADR-0005）
--
-- 「1 案件 × 1 日 = 1 行」は、数十サイトに広げると Free の 500MB を数ヶ月で使い切る
-- （30 サイトで約 40 日。試算は docs/adr/0005-snapshot-storage.md）。
-- 還元額（reward_raw）が変わった時だけ行を作り、valid_from / valid_to で有効期間を持つ形に変える。
--
-- 区間の意味（両端を含む閉区間）：
--   valid_from = この還元額を初めて観測した日（JST）
--   valid_to   = この還元額を最後に観測した日。null = 現在有効（以後の変化を観測していない）
--   「今日も掲載されていた」は offers.last_seen_on で表す。snapshots は還元額の履歴だけを持つ
--   同じ案件の現在有効な行は 1 つ（部分ユニークインデックス）
--
-- 既存の offer_snapshots（2026-09-22〜23 の 2 日分）は移行せず削除する（オーナー承認済み、2026-09-23）。
-- offers（first_seen_on）と crawl_logs（クロール履歴）は残す。次のクロールで全案件に現在有効な行が作られる。

drop view if exists public.current_offers;
drop table if exists public.offer_snapshots;

create table public.offer_snapshots (
  id             bigint generated always as identity primary key,
  offer_id       uuid not null references public.offers (id) on delete cascade,
  valid_from     date not null,
  valid_to       date,
  reward_raw     text not null,
  reward_points  integer check (reward_points >= 0),
  reward_percent numeric(6, 2) check (reward_percent >= 0),
  created_at     timestamptz not null default now(),
  check (valid_to is null or valid_to >= valid_from)
);

comment on table  public.offer_snapshots is '還元額の変化履歴。reward_raw が変わった時だけ行を作る区間テーブル（ADR-0005）。掲載中かどうかは offers.last_seen_on';
comment on column public.offer_snapshots.valid_from is 'この還元額を初めて観測した日（JST）';
comment on column public.offer_snapshots.valid_to is 'この還元額を最後に観測した日（閉区間）。null = 現在有効';
comment on column public.offer_snapshots.reward_raw is '還元額の表示文字列そのまま。変化の判定はこの列の一致で行う';
comment on column public.offer_snapshots.reward_points is '固定額の場合のポイント数。数値抽出に失敗したら null';
comment on column public.offer_snapshots.reward_percent is '率の場合の値（%）。固定額の案件では null';

-- 現在有効な行は案件ごとに 1 つ
create unique index offer_snapshots_current_idx on public.offer_snapshots (offer_id) where valid_to is null;
-- 案件の履歴（価格履歴グラフ）
create index offer_snapshots_offer_id_valid_from_idx on public.offer_snapshots (offer_id, valid_from desc);
-- 「今日変化した案件」（レポート・異常検知・X 速報）
create index offer_snapshots_valid_from_idx on public.offer_snapshots (valid_from desc);

alter table public.offer_snapshots enable row level security;
create policy "offer_snapshots: public read"
  on public.offer_snapshots for select
  to anon, authenticated
  using (true);

grant select on public.offer_snapshots to anon, authenticated;
grant select, insert, update, delete on public.offer_snapshots to service_role;
grant usage, select on all sequences in schema public to service_role;

-- 現在の還元額：valid_to が null の行を引く。reward_since はその還元額になった日
create view public.current_offers
with (security_invoker = true)
as
select
  o.id,
  o.site_id,
  s.name as site_name,
  s.points_per_yen,
  o.external_id,
  o.name,
  o.url,
  o.category,
  o.first_seen_on,
  o.last_seen_on,
  snap.valid_from as reward_since,
  snap.reward_raw,
  snap.reward_points,
  snap.reward_percent,
  case
    when snap.reward_points is not null
      then round(snap.reward_points::numeric / s.points_per_yen)::integer
  end as reward_yen
from public.offers as o
join public.sites as s on s.id = o.site_id
join public.offer_snapshots as snap on snap.offer_id = o.id and snap.valid_to is null;

comment on view public.current_offers is '案件ごとの現在の還元額（offer_snapshots の valid_to が null の行）。reward_since はその還元額になった日、掲載中かは last_seen_on で判断';
grant select on public.current_offers to anon, authenticated, service_role;
