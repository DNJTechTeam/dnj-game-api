#!/usr/bin/env bash
# Preflight da suíte k6: valida ambiente e credenciais ANTES de subir carga.
# Uso: bash loadtest/k6/scripts/k6-preflight.sh
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT}/.env.k6"
fail=0
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; }
warn() { printf '  \033[33m!\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=1; }

echo "== Preflight k6 =="

# 1. k6 presente
if command -v k6 >/dev/null 2>&1; then ok "k6 $(k6 version 2>/dev/null | head -1)"; else bad "k6 não encontrado no PATH"; fi

# 2. .env.k6
if [[ -f "$ENV_FILE" ]]; then
  ok ".env.k6 encontrado"
  set -a; # shellcheck disable=SC1090
  source "$ENV_FILE"; set +a
else
  warn ".env.k6 ausente — usando variáveis de ambiente já exportadas"
fi

BASE_URL="${K6_BASE_URL:-https://ttwkfudhvvhuhp5yvsoydxggum0ictpg.lambda-url.sa-east-1.on.aws/v2}"
ok "alvo: ${BASE_URL}"

# 3. Segredo
if [[ -n "${JWT_IDENTITY_SECRET:-}" ]]; then ok "JWT_IDENTITY_SECRET presente (${#JWT_IDENTITY_SECRET} chars)"; else bad "JWT_IDENTITY_SECRET vazio — sem ele não há tokens autenticados"; fi

# 4. Identidades
USERS_FILE="${K6_USERS_FILE:-${ROOT}/data/users.json}"
if [[ -f "$USERS_FILE" ]]; then
  cnt=$(grep -o '"id"' "$USERS_FILE" | wc -l | tr -d ' ')
  ok "identidades: ${cnt} em ${USERS_FILE}"
  [[ "$cnt" -lt 100 ]] && warn "poucos ids (${cnt}); VUs reutilizarão ids (wrap-around)"
elif [[ -n "${K6_USER_IDS:-}" ]]; then
  ok "identidades via K6_USER_IDS"
else
  bad "sem identidades: crie ${USERS_FILE} (scripts/k6-export-users.sql) ou defina K6_USER_IDS"
fi

# 5. Limites do SO para muitos VUs
soft=$(ulimit -Sn)
if [[ "$soft" -ge 65535 ]]; then ok "ulimit -n = ${soft}"; else warn "ulimit -n = ${soft} (baixo). Para 5000 VUs: 'ulimit -n 1048576'"; fi

# 6. RAM disponível (~1 MB por VU + folga). 5000 VUs ~ 4-6 GB.
if command -v free >/dev/null 2>&1; then
  avail=$(free -m | awk '/^Mem/{print $7}')
  if [[ "$avail" -ge 6000 ]]; then ok "RAM disponível: ${avail} MB"; else warn "RAM disponível: ${avail} MB. 5000 VUs pedem ~6 GB; feche apps ou reduza K6_PARTICIPANTS"; fi
fi

# 7. Healthcheck (sem auth)
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "${BASE_URL}/healthcheck" || echo 000)
if [[ "$code" == "200" ]]; then ok "GET /healthcheck -> 200"; else bad "GET /healthcheck -> ${code} (API inacessível?)"; fi

# 8. Token válido: forja um JWT com o primeiro id e chama /auth/session
if [[ -n "${JWT_IDENTITY_SECRET:-}" ]]; then
  first_id=""
  [[ -f "$USERS_FILE" ]] && first_id=$(grep -o '"id"[^,]*' "$USERS_FILE" | head -1 | grep -o '[0-9]\+' | head -1)
  [[ -z "$first_id" && -n "${K6_USER_IDS:-}" ]] && first_id="${K6_USER_IDS%%,*}"
  if [[ -n "$first_id" ]]; then
    tok=$(K6_TEST_UID="$first_id" k6 run --quiet - <<'JS' 2>/dev/null | grep -o 'TOKEN=.*' | cut -d= -f2
import { signIdentityToken } from './lib/jwt.js';
export default function(){ console.log('TOKEN=' + signIdentityToken(__ENV.K6_TEST_UID, __ENV.JWT_IDENTITY_SECRET, 600)); }
JS
)
    if [[ -n "$tok" ]]; then
      sc=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 -H "Authorization: Bearer ${tok}" "${BASE_URL}/auth/session")
      if [[ "$sc" == "200" ]]; then ok "GET /auth/session (token forjado id=${first_id}) -> 200"; else bad "GET /auth/session -> ${sc} (segredo errado ou id inexistente em develop)"; fi
    else
      warn "não consegui forjar token no preflight (rode do diretório loadtest/k6)"
    fi
  fi
fi

echo
if [[ "$fail" -eq 0 ]]; then echo -e "\033[32mPreflight OK.\033[0m"; else echo -e "\033[31mPreflight com falhas — corrija antes de rodar carga.\033[0m"; fi
exit $fail
