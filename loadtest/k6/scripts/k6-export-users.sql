-- Exporta identidades reais de develop para a suíte k6.
-- Rode contra o Postgres de develop e salve o resultado em loadtest/k6/data/users.json.
--
-- Via psql (produz o JSON pronto):
--   psql "$DEVELOP_DATABASE_URL" -Atc "$(cat loadtest/k6/scripts/k6-export-users.sql)" \
--     > loadtest/k6/data/users.json
--
-- Ajuste o LIMIT ao nº de VUs desejado (ids são reutilizados se houver menos que VUs).
-- Inclui alguns EVENT_MANAGER para o cenário de gestor; o resto DEFAULT.

SELECT json_agg(row_to_json(t))
FROM (
  (
    SELECT id::text AS id, role
    FROM users
    WHERE deleted_at IS NULL AND role = 'EVENT_MANAGER'
    ORDER BY id
    LIMIT 20
  )
  UNION ALL
  (
    SELECT id::text AS id, role
    FROM users
    WHERE deleted_at IS NULL AND role = 'DEFAULT'
    ORDER BY id
    LIMIT 5000
  )
) t;
