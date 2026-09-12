-- Seed de dados sintéticos para o teste de carga simular um evento real.
--
-- POR QUE seedar: as queries mais caras da API (GET /v2/game/overview e
-- /v2/rankings) fazem ROW_NUMBER() com full scan sobre a tabela `users` inteira,
-- filtrando onboarding_complete = TRUE AND role = 'DEFAULT'. Num banco vazio esse
-- scan é gratuito e o teste mede nada. Este script enche `users`, `groups`,
-- `group_memberships` e `point_entries` para dar às window functions o custo real
-- de um evento de N participantes.
--
-- Tudo é prefixado (emails 'k6-load+' e 'k6-mgr+', grupos 'K6LOAD Grupo') para ser
-- idempotente e removível — ver k6-seed-cleanup.sql.
--
-- Parâmetros (psql -v):
--   N = nº de participantes DEFAULT   (ex.: 15000)
--   G = nº de grupos                  (ex.: 200)
--   P = entradas de ponto por usuário (ex.: 5)
--   M = nº de gestores EVENT_MANAGER  (ex.: 20)
--
-- Uso:
--   psql "$DATABASE_URL" -v N=15000 -v G=200 -v P=5 -v M=20 \
--        -f loadtest/k6/scripts/k6-seed.sql

\set ON_ERROR_STOP on
\timing on

BEGIN;

-- 1. Grupos ------------------------------------------------------------------
INSERT INTO groups (name)
SELECT 'K6LOAD Grupo ' || gs
FROM generate_series(1, :G) gs
ON CONFLICT (name) DO NOTHING;

-- 2. Participantes (role DEFAULT, onboarding completo -> entram no ranking) ---
INSERT INTO users (email, name, role, points, onboarding_complete, group_id, created_at, updated_at)
SELECT
  'k6-load+' || g || '@dnj.local',
  'K6 Participante ' || g,
  'DEFAULT',
  0,
  TRUE,
  (SELECT id FROM groups WHERE name = 'K6LOAD Grupo ' || ((g % :G) + 1)),
  now(),
  now()
FROM generate_series(1, :N) g
ON CONFLICT (email) DO NOTHING;

-- 3. Memberships (a fonte de verdade dos joins de ranking) -------------------
INSERT INTO group_memberships (user_id, group_id, joined_at, created_at, updated_at)
SELECT u.id, u.group_id, now(), now(), now()
FROM users u
WHERE u.email LIKE 'k6-load+%' AND u.group_id IS NOT NULL
ON CONFLICT (user_id) DO NOTHING;

-- 4. Ledger de pontos (P por usuário) — dá custo real ao read do overview -----
-- origin='legacy_balance' é o único valor do check que permite todas as FKs nulas,
-- e há uma unique (uma legacy_balance por usuário), então é UMA entrada por usuário.
-- O marcador do seed é reason='k6 load seed'. delta precisa ser <> 0. (:P é ignorado.)
INSERT INTO point_entries (id, user_id, origin, reason, delta, created_at)
SELECT
  gen_random_uuid(),
  u.id,
  'legacy_balance',
  'k6 load seed',
  (random() * 500)::int + 1,
  now()
FROM users u
WHERE u.email LIKE 'k6-load+%'
ON CONFLICT DO NOTHING;

-- 4b. Sincroniza users.points com a soma do ledger (evita divergência que a
--     query de consistência do app sinalizaria). O ranking usa users.points.
UPDATE users
SET points = sub.total
FROM (
  SELECT user_id, COALESCE(SUM(delta), 0) AS total
  FROM point_entries
  WHERE reason = 'k6 load seed'
  GROUP BY user_id
) sub
WHERE users.id = sub.user_id AND users.email LIKE 'k6-load+%';

-- 5. Gestores EVENT_MANAGER (para o cenário `manager`) -----------------------
INSERT INTO users (email, name, role, manager_scope, onboarding_complete, created_at, updated_at)
SELECT
  'k6-mgr+' || g || '@dnj.local',
  'K6 Gestor ' || g,
  'EVENT_MANAGER',
  CASE WHEN g % 2 = 0 THEN 'space' ELSE 'actions' END,
  TRUE,
  now(),
  now()
FROM generate_series(1, :M) g
ON CONFLICT (email) DO NOTHING;

COMMIT;

-- Resumo do que ficou no banco.
SELECT
  (SELECT count(*) FROM users WHERE email LIKE 'k6-load+%') AS participantes,
  (SELECT count(*) FROM users WHERE email LIKE 'k6-mgr+%')  AS gestores,
  (SELECT count(*) FROM groups WHERE name LIKE 'K6LOAD %')  AS grupos,
  (SELECT count(*) FROM point_entries WHERE reason = 'k6 load seed') AS entradas_ponto;
