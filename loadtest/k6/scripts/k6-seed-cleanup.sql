-- Remove TODO o dado sintético criado por k6-seed.sql, e nada além dele.
--
-- Segurança: o seed marca tudo com prefixos exclusivos —
--   users.email   LIKE 'k6-load+%'  ou  'k6-mgr+%'
--   groups.name   LIKE 'K6LOAD %'
--   point_entries.origin = 'SEED'
-- Este cleanup apaga apenas linhas com esses marcadores, então dados reais
-- (sem os prefixos) NÃO são tocados. Ainda assim: faça backup antes, como você
-- já planeja.
--
-- Uso:
--   psql "$DATABASE_URL" -f loadtest/k6/scripts/k6-seed-cleanup.sql

\set ON_ERROR_STOP on
\timing on

BEGIN;

-- Ordem respeita as FKs: filhos antes dos pais.

-- Ledger sintético.
DELETE FROM point_entries
WHERE reason = 'k6 load seed'
   OR user_id IN (SELECT id FROM users WHERE email LIKE 'k6-load+%' OR email LIKE 'k6-mgr+%');

-- Memberships dos usuários sintéticos.
DELETE FROM group_memberships
WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'k6-load+%' OR email LIKE 'k6-mgr+%');

-- Qualquer moment/like/notification sintético eventualmente criado durante o teste
-- (as mutações leves usam usuários sintéticos). Só apaga o que estiver ligado a eles.
-- media_moments e afins referenciam user_id; ajuste os nomes se seu schema divergir.
DO $$
BEGIN
  IF to_regclass('public.moment_likes') IS NOT NULL THEN
    EXECUTE 'DELETE FROM moment_likes WHERE user_id IN (SELECT id FROM users WHERE email LIKE ''k6-load+%'' OR email LIKE ''k6-mgr+%'')';
  END IF;
  IF to_regclass('public.moments') IS NOT NULL THEN
    EXECUTE 'DELETE FROM moments WHERE user_id IN (SELECT id FROM users WHERE email LIKE ''k6-load+%'' OR email LIKE ''k6-mgr+%'')';
  END IF;
  IF to_regclass('public.notifications') IS NOT NULL THEN
    EXECUTE 'DELETE FROM notifications WHERE user_id IN (SELECT id FROM users WHERE email LIKE ''k6-load+%'' OR email LIKE ''k6-mgr+%'')';
  END IF;
  IF to_regclass('public.refresh_sessions') IS NOT NULL THEN
    EXECUTE 'DELETE FROM refresh_sessions WHERE user_id IN (SELECT id FROM users WHERE email LIKE ''k6-load+%'' OR email LIKE ''k6-mgr+%'')';
  END IF;
END $$;

-- Usuários sintéticos (participantes + gestores).
DELETE FROM users WHERE email LIKE 'k6-load+%' OR email LIKE 'k6-mgr+%';

-- Grupos sintéticos (só depois que as memberships saíram).
DELETE FROM groups WHERE name LIKE 'K6LOAD %';

COMMIT;

-- Confirma que não sobrou nada com os marcadores.
SELECT
  (SELECT count(*) FROM users WHERE email LIKE 'k6-load+%' OR email LIKE 'k6-mgr+%') AS users_restantes,
  (SELECT count(*) FROM groups WHERE name LIKE 'K6LOAD %') AS grupos_restantes,
  (SELECT count(*) FROM point_entries WHERE reason = 'k6 load seed') AS entradas_restantes;
