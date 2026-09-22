-- ポイ得比較 初期スキーマ
--
-- 旧 SPEC.md（git: fc55174^:SPEC.md）の offers / crawl_logs を土台に、以下の方針で再設計した。
--   * 履歴を積む：案件の同一性（offers）と日次の還元額（offer_snapshots）を分ける。
--     旧設計は offers に fetched_date を持たせて毎日行を増やしていたが、案件ページ生成や
--     過去最高額の算出には「案件 1 行 + 履歴 N 行」の方が扱いやすい
--   * 将来の案件正規化（サイト横断マッチング、Phase 4）に備え、サイト側の案件 ID を
--     external_id として持つ。正規化テーブル自体は必要になった時に追加する
--   * サイトはマスタ（sites）に出し、ポイント単価（points_per_yen）を持たせて円換算できるようにする
--   * 転載しない：保存するのは案件名・還元額・URL・カテゴリのみ（AGENTS.md）。旧設計の
--     description 列は持たない
--   * 書き込みはクローラー（secret key、RLS を通らない）のみ。anon（publishable key）は読み取り専用
--
-- 適用方法は supabase/README.md を参照。

-- ---------------------------------------------------------------------------
-- 拡張
-- ---------------------------------------------------------------------------

-- 案件名の部分一致検索用（Supabase では extensions スキーマに置く慣例）
create extension if not exists pg_trgm with schema extensions;

-- ---------------------------------------------------------------------------
-- 共通：updated_at の自動更新
-- ---------------------------------------------------------------------------

create or replace function public.set_updated_at()
returns trigger
language plpgsql
set search_path = ''
as $$
begin
  new.updated_at = now();
  return new;
end;
$$;

-- ---------------------------------------------------------------------------
-- sites：対象ポイントサイトのマスタ
-- ---------------------------------------------------------------------------

create table public.sites (
  id             text primary key,
  name           text not null,
  url            text not null,
  points_per_yen integer not null default 1 check (points_per_yen > 0),
  is_active      boolean not null default true,
  created_at     timestamptz not null default now(),
  updated_at     timestamptz not null default now()
);

comment on table  public.sites is '対象ポイントサイトのマスタ。id は crawler/sites/<id>.yaml と一致させる';
comment on column public.sites.id is 'スラッグ（moppy, hapitas ...）';
comment on column public.sites.points_per_yen is '1円あたりのポイント数。モッピー・ハピタスは 1、ポイントインカムは 10';
comment on column public.sites.is_active is 'false にするとクロール対象から外す';

create trigger sites_set_updated_at
  before update on public.sites
  for each row execute function public.set_updated_at();

-- ---------------------------------------------------------------------------
-- offers：サイトごとの案件（同一性の単位。還元額は持たず offer_snapshots に積む）
-- ---------------------------------------------------------------------------

create table public.offers (
  id            uuid primary key default gen_random_uuid(),
  site_id       text not null references public.sites (id),
  external_id   text,
  name          text not null,
  url           text not null,
  category      text,
  first_seen_on date not null,
  last_seen_on  date not null,
  created_at    timestamptz not null default now(),
  updated_at    timestamptz not null default now(),
  unique (site_id, url)
);

comment on table  public.offers is 'サイトごとの案件。1 案件 1 行。日次の還元額は offer_snapshots';
comment on column public.offers.external_id is 'サイト側の案件 ID（URL から抽出）。将来のサイト横断マッチングの手がかり';
comment on column public.offers.name is '案件名（表示名そのまま。正規化名は将来別列で持つ）';
comment on column public.offers.url is '案件ページの URL。サイト内で一意';
comment on column public.offers.first_seen_on is '初めて観測した日（JST）';
comment on column public.offers.last_seen_on is '最後に観測した日（JST）。今日でなければ掲載終了の可能性';

create index offers_site_id_idx on public.offers (site_id);
create index offers_external_id_idx on public.offers (site_id, external_id);
create index offers_last_seen_on_idx on public.offers (last_seen_on desc);
create index offers_name_trgm_idx on public.offers using gin (name extensions.gin_trgm_ops);

create trigger offers_set_updated_at
  before update on public.offers
  for each row execute function public.set_updated_at();

-- ---------------------------------------------------------------------------
-- offer_snapshots：日次の還元額履歴（1 案件 × 1 日 = 1 行）
-- ---------------------------------------------------------------------------

create table public.offer_snapshots (
  id             bigint generated always as identity primary key,
  offer_id       uuid not null references public.offers (id) on delete cascade,
  crawled_on     date not null,
  reward_raw     text not null,
  reward_points  integer check (reward_points >= 0),
  reward_percent numeric(6, 2) check (reward_percent >= 0),
  created_at     timestamptz not null default now(),
  unique (offer_id, crawled_on)
);

comment on table  public.offer_snapshots is '日次の還元額。価格履歴・過去最高額・買い時判定の元データ';
comment on column public.offer_snapshots.crawled_on is 'クロール日（JST）。1 日 1 回なので日付で一意';
comment on column public.offer_snapshots.reward_raw is '還元額の表示文字列そのまま（"10,000P", "1.5%" 等）。抽出精度の検証と修復に使う';
comment on column public.offer_snapshots.reward_points is '固定額の場合のポイント数。数値抽出に失敗したら null（KPI: 抽出精度）';
comment on column public.offer_snapshots.reward_percent is '購入額に対する率の場合の値（%）。固定額の案件では null';

create index offer_snapshots_crawled_on_idx on public.offer_snapshots (crawled_on desc);

-- ---------------------------------------------------------------------------
-- crawl_logs：クロール実行ログ（KPI の一次データ。docs/02-kpi.md）
-- ---------------------------------------------------------------------------

create table public.crawl_logs (
  id              bigint generated always as identity primary key,
  site_id         text not null references public.sites (id),
  crawled_on      date not null,
  started_at      timestamptz not null default now(),
  finished_at     timestamptz,
  status          text not null default 'running'
                  check (status in ('running', 'success', 'partial', 'failed', 'aborted')),
  request_count   integer not null default 0,
  offer_count     integer not null default 0,
  parsed_count    integer not null default 0,
  error_count     integer not null default 0,
  abort_reason    text,
  errors          jsonb not null default '[]'::jsonb,
  crawler_version text,
  created_at      timestamptz not null default now()
);

comment on table  public.crawl_logs is 'サイト × 実行ごとのクロール結果。クロール成功率・抽出精度の計測元';
comment on column public.crawl_logs.status is 'running: 実行中 / success: 全件正常 / partial: 一部エラー / failed: データ取得なし / aborted: サーキットブレーカーで停止';
comment on column public.crawl_logs.request_count is '発行した HTTP リクエスト数';
comment on column public.crawl_logs.offer_count is '取得した案件数（KPI: 対象案件数）';
comment on column public.crawl_logs.parsed_count is '数値抽出に成功した件数（KPI: 抽出精度 = parsed_count / offer_count）';
comment on column public.crawl_logs.abort_reason is '停止理由（http_429, http_403, server_error_streak, robots_disallow ...）';
comment on column public.crawl_logs.errors is 'エラーの要約配列。Routine の原因分類用。件数上限はクローラー側で制御する';
comment on column public.crawl_logs.crawler_version is 'クローラーの git SHA。結果と実装の対応付けに使う';

create index crawl_logs_site_id_crawled_on_idx on public.crawl_logs (site_id, crawled_on desc);

-- ---------------------------------------------------------------------------
-- current_offers：各案件の最新スナップショットを結合したビュー（フロントの静的生成用）
-- ---------------------------------------------------------------------------

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
  snap.crawled_on,
  snap.reward_raw,
  snap.reward_points,
  snap.reward_percent,
  case
    when snap.reward_points is not null
      then round(snap.reward_points::numeric / s.points_per_yen)::integer
  end as reward_yen
from public.offers as o
join public.sites as s on s.id = o.site_id
join lateral (
  select os.crawled_on, os.reward_raw, os.reward_points, os.reward_percent
  from public.offer_snapshots as os
  where os.offer_id = o.id
  order by os.crawled_on desc
  limit 1
) as snap on true;

comment on view public.current_offers is '案件ごとの最新還元額。reward_yen はサイトのポイント単価で円換算した値';

-- ---------------------------------------------------------------------------
-- RLS：読み取りは公開、書き込みはポリシーなし（secret key のみ）
-- ---------------------------------------------------------------------------

alter table public.sites           enable row level security;
alter table public.offers          enable row level security;
alter table public.offer_snapshots enable row level security;
alter table public.crawl_logs      enable row level security;

create policy "sites: public read"
  on public.sites for select
  to anon, authenticated
  using (true);

create policy "offers: public read"
  on public.offers for select
  to anon, authenticated
  using (true);

create policy "offer_snapshots: public read"
  on public.offer_snapshots for select
  to anon, authenticated
  using (true);

-- crawl_logs は運用データなので公開しない。KPI レポートは secret key を持つ GitHub Actions が読む。
-- フロントが「最終更新日」を出す時は offer_snapshots.crawled_on を使う

-- ---------------------------------------------------------------------------
-- 初期データ：対象サイト（docs/reference/point-sites.md）
-- ---------------------------------------------------------------------------

insert into public.sites (id, name, url, points_per_yen) values
  ('moppy',   'モッピー', 'https://pc.moppy.jp/', 1),
  ('hapitas', 'ハピタス', 'https://hapitas.jp/',  1)
on conflict (id) do nothing;
