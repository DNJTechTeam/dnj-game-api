# Suíte de carga k6 — DNJ Game API

Teste de carga que reproduz o tráfego real do front contra a **API V2 develop**
(Lambda Function URL). Gera um **relatório HTML** self-contained ao fim de cada
execução, com tabelas por rota e uma seção "onde observar" (Supabase, Lambda
Insights, CloudWatch). A análise do volume de requisições do front está em
[`../../docs/load-testing-k6.md`](../../docs/load-testing-k6.md).

> **Guia de execução e handoff:** [`../README.md`](../README.md) — como rodar, o que
> cada perfil faz, como observar o comportamento ao vivo e o que entregar quando
> outra pessoa for executar. Este arquivo é só a referência técnica da suíte.

## Requisitos

- [k6](https://k6.io) ≥ 0.50 (`k6 version`). Testado com v2.2.0.
- Bash, `curl`, `python3` (só para o preflight e utilitários).
- Para autenticar VUs: o **`JWT_IDENTITY_SECRET` de develop** e uma lista de
  **IDs de usuários reais** de develop (ver abaixo).

## Configuração

Tudo é por variável de ambiente. Copie o exemplo e preencha:

```bash
cp loadtest/k6/.env.k6.example loadtest/k6/.env.k6
# edite loadtest/k6/.env.k6 e preencha JWT_IDENTITY_SECRET
```

| Variável | Papel | Default |
|---|---|---|
| `K6_BASE_URL` | URL da API V2 alvo | Lambda develop (embutida) |
| `JWT_IDENTITY_SECRET` | segredo HS256 p/ forjar tokens | **(obrigatório p/ auth)** |
| `K6_USERS_FILE` | JSON de identidades | `./loadtest/k6/data/users.json` |
| `K6_USER_IDS` | alternativa: ids separados por vírgula | — |
| `K6_TOKEN_TTL_SECONDS` | validade dos tokens forjados | `10800` (3 h) |
| `PROFILE` | `smoke\|soak\|spike\|full` | `smoke` |
| `K6_REQUEST_TIMEOUT` | timeout por request | `10s` |
| `K6_OUT_DIR` | pasta dos relatórios | `./loadtest/k6/out` |
| `K6_LAMBDA_FUNCTION_NAME` | nome da Lambda (rotula queries do relatório) | — |

Overrides de dimensionamento: `K6_PARTICIPANTS`, `K6_HOLD`, `K6_RAMP`,
`K6_DISPLAYS`, `K6_MANAGERS`, `K6_SPIKE_VUS`, `K6_MUT_RATE`.

### Como os tokens são gerados

O middleware de auth só valida a assinatura HS256 e as claims (`iss`, `aud`, `exp`);
**não consulta o banco**. Por isso o k6 assina JWTs localmente
(`lib/jwt.js`), sem passar pelo fluxo de e-mail/OTP. Cada JWT usa um `sub` = id de
usuário **real** de develop. Sem ids reais, as rotas por-usuário (game/overview,
notifications) respondem com dados vazios ou erro.

### Obter os IDs de usuários

Rode contra o Postgres de develop e salve em `data/users.json`:

```bash
psql "$DEVELOP_DATABASE_URL" -Atc "$(cat loadtest/k6/scripts/k6-export-users.sql)" \
  > loadtest/k6/data/users.json
```

Formato aceito: `[{ "id": "123", "role": "DEFAULT" }, ...]` ou `["123","124"]`.
Inclua alguns `EVENT_MANAGER` para o cenário de gestor. O arquivo é gitignored.

## Executar

```bash
# 1. Preflight: valida ulimit, RAM, healthcheck e um token real.
make k6-preflight

# 2. Smoke (50 VUs, ~2 min) — SEMPRE rode isto antes da carga pesada.
PROFILE=smoke make k6-run

# 3. Carga alvo (≥ 5000 VUs, todos os cenários coordenados).
PROFILE=full  make k6-run
```

Sem o Makefile:

```bash
set -a; source loadtest/k6/.env.k6; set +a
cd loadtest/k6 && k6 run -e PROFILE=smoke main.js
```

Para 5000 VUs, eleve o limite de descritores **antes** de rodar:

```bash
ulimit -n 1048576
```

## Perfis

| Perfil | Participantes | Outros cenários | Duração | Uso |
|---|---:|---|---|---|
| `smoke` | 50 | telão ×2 | ~2 min | validação segura; zero 5xx tolerado |
| `soak` | 1500 | telão, gestor, mutações | ~20 min | carga sustentada |
| `spike` | 1000 | pico coordenado ×800 | ~5 min | todos abrindo o Game juntos |
| `full` | **5000** | telão, gestor, mutações, pico ×1500 | ~25 min | teste alvo |

## Saída

Ao fim, em `out/`:
- `report-<perfil>-<timestamp>.html` — relatório visual (abra no navegador).
- `summary-<perfil>-<timestamp>.json` — dados brutos do k6.

O HTML traz: KPIs globais, tabela por rota (reqs, p50/p95/p99/max), thresholds
pass/fail, e a seção **"Onde observar e o quê"** com queries prontas de CloudWatch
Logs Insights e checklist de Supabase e Lambda Insights.

## Correlação com os logs da API

Cada request leva um `X-Request-ID` = `k6-<cenário>-<vu>-<iter>-<seq>`, ecoado na
resposta e no log `http_request_completed` (com `route`, `status`, `latencyMs`,
`dbQueries`, `dbMs`). No CloudWatch Logs Insights, `filter requestId like /^k6-/`
liga cada linha de log à iteração do k6 que a gerou.

## Estrutura

```
loadtest/k6/
├── main.js                # entrypoint: options + handleSummary
├── config/profiles.js     # perfis e thresholds
├── lib/                   # jwt.js, users.js, http.js, routes.js
├── scenarios/             # participant, display, manager, spike, mutations
├── report/                # html.js + observability.js
├── scripts/               # k6-preflight.sh, k6-export-users.sql, k6-aws-discover.sh
├── data/                  # users.json (gitignored) + users.example.json
└── out/                   # relatórios (gitignored)
```

## Cuidados

- **Não** envie header `Origin` (o CORS aborta 403) nem parâmetros de query extras
  (allowlist estrita → 400). O `lib/http.js` já respeita isso.
- Mutações usam `Idempotency-Key` UUID único por chamada; reusar a chave devolveria
  a resposta anterior e falsearia a medição.
- O escopo padrão evita QR (bloqueio de 10 min/usuário) e upload de mídia.
