#!/usr/bin/env bash
# Ciclo completo e seguro do teste de carga:
#   1. seed de N usuários sintéticos (prefixados, isolados dos dados reais)
#   2. export dos IDs para data/users.json
#   3. k6 run do perfil escolhido
#   4. CLEANUP dos dados sintéticos — roda SEMPRE (trap), mesmo em erro ou Ctrl+C
#
# Requer: psql no PATH, um DATABASE_URL apontando para o banco alvo (o MESMO que a
# Lambda de teste usa), e o .env.k6 preenchido para o k6.
#
# ATENÇÃO: seeda e apaga dados no banco de $SEED_DATABASE_URL. Rode contra o banco
# de teste/pré-lançamento, faça backup antes, e confirme que é o alvo certo.
#
# Uso:
#   SEED_DATABASE_URL='postgres://...:5432/postgres' \
#   PROFILE=full K6_PARTICIPANTS=15000 \
#   bash loadtest/k6/scripts/k6-run-managed.sh
#
# Variáveis:
#   SEED_DATABASE_URL  (obrigatória) conexão psql para seed/cleanup
#   N,G,P,M            tamanhos do seed (default 15000 / 200 / 5 / 20)
#   PROFILE            perfil k6 (default full)
#   SKIP_CLEANUP=1     não apaga ao final (para inspecionar; limpe depois à mão)
#   SEED_ONLY=1        só seeda e exporta, não roda nem limpa
set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"      # loadtest/k6
ROOT="$(cd "$DIR/../.." && pwd)"                            # raiz do repo
# Carrega .env.k6 cedo: SEED_DATABASE_URL e JWT_IDENTITY_SECRET podem morar lá.
if [[ -f "$DIR/.env.k6" ]]; then set -a; . "$DIR/.env.k6"; set +a; fi
: "${SEED_DATABASE_URL:?defina SEED_DATABASE_URL (no .env.k6 ou no ambiente)}"
N="${N:-15000}"; G="${G:-200}"; P="${P:-5}"; M="${M:-20}"
PROFILE="${PROFILE:-full}"

command -v psql >/dev/null || { echo "psql não encontrado no PATH"; exit 1; }
command -v k6   >/dev/null || { echo "k6 não encontrado no PATH"; exit 1; }

cleanup() {
  if [[ "${SKIP_CLEANUP:-0}" == "1" ]]; then
    echo ">> SKIP_CLEANUP=1: dados sintéticos MANTIDOS. Limpe com k6-seed-cleanup.sql."
    return
  fi
  echo ">> [4/4] cleanup dos dados sintéticos..."
  psql "$SEED_DATABASE_URL" -f "$DIR/scripts/k6-seed-cleanup.sql"
}
trap cleanup EXIT

echo ">> [1/4] seed: N=$N grupos=$G pontos/usuário=$P gestores=$M"
psql "$SEED_DATABASE_URL" -v N="$N" -v G="$G" -v P="$P" -v M="$M" \
  -f "$DIR/scripts/k6-seed.sql"

echo ">> [2/4] export dos IDs -> data/users.json"
mkdir -p "$DIR/data"
psql "$SEED_DATABASE_URL" -Atc \
  "SELECT json_agg(row_to_json(t)) FROM (
     (SELECT id::text AS id, role FROM users WHERE email LIKE 'k6-mgr+%'  AND deleted_at IS NULL ORDER BY id LIMIT $M)
     UNION ALL
     (SELECT id::text AS id, role FROM users WHERE email LIKE 'k6-load+%' AND deleted_at IS NULL ORDER BY id LIMIT $N)
   ) t;" > "$DIR/data/users.json"
echo "   $(grep -o '\"id\"' "$DIR/data/users.json" | wc -l | tr -d ' ') identidades exportadas"

if [[ "${SEED_ONLY:-0}" == "1" ]]; then
  echo ">> SEED_ONLY=1: seed e export prontos; não vou rodar o k6 nem limpar."
  trap - EXIT   # não limpa: usuário vai rodar manualmente
  exit 0
fi

echo ">> [3/4] k6 run (PROFILE=$PROFILE)"
if [[ -f "$DIR/.env.k6" ]]; then set -a; . "$DIR/.env.k6"; set +a; fi
( cd "$DIR" && k6 run -e PROFILE="$PROFILE" main.js )

echo ">> teste concluído; cleanup no trap EXIT."
