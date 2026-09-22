-- テーブル権限の明示的な付与
--
-- 初回スキーマ（20260922000000）はローカル CLI のリンク経由（一時ログインロール）で適用されたため、
-- Supabase が postgres ロール向けに用意している既定の権限付与（anon / authenticated / service_role への
-- GRANT）がテーブルに付いていない。その結果、クローラー（secret key = service_role）の
-- crawl_logs への INSERT が「permission denied for table crawl_logs」（42501）で失敗した（2026-09-22）。
--
-- ここで既存のテーブル・ビュー・シーケンスに権限を明示的に付ける。RLS のポリシー（読み取り公開）は
-- 20260922000000 のままで、GRANT はポリシーの前提となるテーブル権限。
-- 以後のマイグレーションは CI（deploy.yml）が postgres ロールで適用するため、新しいテーブルには
-- 既定の権限付与が効く。手元のリンク経由で適用した場合は同様の GRANT を忘れないこと（supabase/README.md）。

grant usage on schema public to anon, authenticated, service_role;

-- 読み取り：公開する 3 テーブルとビュー（RLS のポリシーが行を制御する）
grant select on public.sites, public.offers, public.offer_snapshots, public.current_offers
  to anon, authenticated;

-- 書き込み：クローラー（service_role、RLS を通らない）
grant select, insert, update, delete on public.sites, public.offers, public.offer_snapshots, public.crawl_logs
  to service_role;
grant select on public.current_offers to service_role;

-- identity 列（offer_snapshots.id, crawl_logs.id）のシーケンス。INSERT に USAGE が要る
grant usage, select on all sequences in schema public to service_role;
