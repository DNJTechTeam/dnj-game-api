-- Seed de moments para o teste de carga exercitar o FEED de verdade.
--
-- POR QUE: GET /v2/moments?scope=feed faz, por página, um N+1 (uma busca de mídia
-- por item) mais ~20 assinaturas de URL do S3. Com a tabela de moments vazia, esse
-- custo nunca dispara e o teste não representa o pico de moments. Este script cria
-- moments "free" visíveis no feed, cada um com seu media_asset "available", para o
-- feed fazer o trabalho real. O objeto no S3 não precisa existir: a assinatura é só
-- HMAC sobre a chave, o custo dispara mesmo com chave fictícia.
--
-- Visibilidade exigida pelo feed (moment_repository.go): publication_status='public',
-- moderation_status<>'rejected', media_assets.state='available', retention no futuro,
-- autor não deletado e com onboarding completo (os k6-load+ satisfazem).
--
-- Marcadores para o cleanup: staging/final object key começam com 'k6load/'.
--
-- Parâmetro: M = nº de moments (ex.: 5000).
-- Uso: psql "$DATABASE_URL" -v M=5000 -f loadtest/k6/scripts/k6-seed-moments.sql

\set ON_ERROR_STOP on
\timing on

BEGIN;

WITH u AS (
  SELECT id, (row_number() OVER (ORDER BY id) - 1) AS rn
  FROM users WHERE email LIKE 'k6-load+%'
),
ucount AS (SELECT GREATEST(count(*),1) AS c FROM u),
base AS (
  SELECT
    i,
    gen_random_uuid() AS aid,
    gen_random_uuid() AS mid,
    (SELECT id FROM u WHERE u.rn = (i % (SELECT c FROM ucount))) AS uid
  FROM generate_series(1, :M) AS i
),
ins_assets AS (
  INSERT INTO media_assets
    (id, owner_user_id, provider, bucket, staging_object_key, final_object_key,
     content_type, bytes, checksum_sha256, state, upload_expires_at, retention_due_at,
     available_at, created_at, updated_at)
  SELECT
    aid, uid, 's3', 'dnj-moments',
    'k6load/staging/' || i, 'k6load/final/' || i,
    'image/jpeg', 500000,
    encode(sha256(('k6' || i)::bytea), 'base64'),
    'available',
    now() + interval '1 hour', now() + interval '30 days',
    now(), now(), now()
  FROM base
  RETURNING 1
)
INSERT INTO moments
  (id, user_id, participation_id, activity_id, media_asset_id, origin,
   publication_status, moderation_status, reward_status, points_awarded,
   captured_at, created_at, updated_at)
SELECT
  mid, uid, NULL, NULL, aid, 'free',
  'public', 'approved', 'not_applicable', 0,
  now() - ((i % 720) || ' minutes')::interval, now(), now()
FROM base;

COMMIT;

SELECT
  (SELECT count(*) FROM moments WHERE publication_status='public' AND media_asset_id IN
     (SELECT id FROM media_assets WHERE staging_object_key LIKE 'k6load/%')) AS moments_feed,
  (SELECT count(*) FROM media_assets WHERE staging_object_key LIKE 'k6load/%') AS assets;
