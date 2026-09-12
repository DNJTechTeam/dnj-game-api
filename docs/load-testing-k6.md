# Teste de carga k6 — DNJ Game API

Suíte k6 que reproduz o tráfego real do front (`AlexandrePatob/dnj`, Next.js PWA)
contra a API V2. Vive em [`loadtest/k6/`](../loadtest/k6/). Este documento traz a
**análise do volume de requisições do front** que embasa os cenários; o passo a
guia de execução, o handoff e o runbook de observabilidade estão em
[`loadtest/README.md`](../loadtest/README.md); a referência técnica da suíte em
[`loadtest/k6/README.md`](../loadtest/k6/README.md).

## Volume e cadência de requisições do front

O front bate **direto na Lambda** (sem proxy), com timeout de 10 s por request e,
nas mutações, 3 tentativas sem backoff em 0/408/429/5xx (`src/lib/api/client.ts`).
Nenhuma chamada à API passa por cache do service worker.

### Pollings por papel

| Papel / tela | Endpoint | Método | Cadência | Origem no front |
|---|---|---|---|---|
| Participante (shell, todas as telas) | `/special-events/active?target=app` | GET | **15 s** | `dnj-app.tsx:313` |
| Participante (shell) | `/notifications` | GET | **15 s** | `dnj-app.tsx:328` |
| Participante (shell) | `/activities?kind=challenge` | GET | **15 s** | `dnj-app.tsx:342` |
| Participante (shell, exceto tela game) | `/game/overview` | GET | **15 s** | `dnj-app.tsx:360` |
| Participante (tela game, run ativa) | `/activity-runs/current` | GET | **5 s** | `game-screen.tsx:405` |
| Participante (tela game, mount) | `/game/overview`, `/participations/current` | GET | mount | `game-screen.tsx` |
| Participante (mount app) | `/auth/session` | GET | mount | `dnj-app.tsx:216` |
| Participante (home) | `/schedule?view=home` | GET | mount | `home-screen.tsx:31` |
| Participante (galeria) | `/moments?scope=feed\|mine\|group` | GET | mount / aba | `gallery-screen.tsx:420` |
| Telão / TV (público, sem token) | `/rankings?scope=individual&page=1` | GET | **15 s** | `live-ranking-display.tsx:284` |
| Telão / TV | `/rankings?scope=groups&page=1` | GET | **15 s** | `live-ranking-display.tsx:284` |
| Telão / TV | `/live-display?target=tv\|screen` | GET | **15 s** | `live-ranking-display.tsx:284` |
| Gestor (EVENT_MANAGER) | `/manager/game-overview` | GET | **15 s** (com guard de visibilidade) | `manager-dashboard.tsx:215,236` |

Mutações relevantes (ação do usuário): `POST /qr/validate` (bloqueio 10 min/usuário),
`POST /moments/:id/likes` (toggle), `POST /notifications/:id/read`,
`POST /moments[/challenge]`, fluxo de upload de mídia. Auto-release de QR de evento
especial dispara `POST /manager/special-events/qr` 30 s após o teaser.

### Baseline de carga por participante

Os quatro pollings do shell **não têm guard de visibilidade**: seguem disparando
com a aba em segundo plano (só param offline). São desalinhados em fase, sem batching.

| Tela aberta | Pollings simultâneos | Requests/min por aba |
|---|---|---|
| Home / cronograma / mapa / galeria / conta | special-events + notifications + challenge + game/overview (15 s) | **16 req/min** |
| Tela game (sem run) | special-events + notifications + challenge (15 s) | **12 req/min** |
| Tela game (com run ativa) | os 3 acima + activity-runs/current (5 s) | **24 req/min** |

Com **N** participantes, o piso é ≈ **16·N req/min** só de polling, mais 12 req/min
por telão/TV aberto (3 GETs a cada 15 s). Para 5000 participantes: ≈ **1333 req/s**
de baseline sustentado, sem contar picos de abertura de tela nem mutações.

### Como isso vira cenários k6

| Cenário k6 | Reproduz | Executor |
|---|---|---|
| `participant` | shell + tela game (16–24 req/min por VU, fase com jitter) | `ramping-vus` até ≥ 5000 |
| `display` | telão/TV (3 GETs públicos a cada 15 s) | `constant-vus` |
| `manager` | gestor (game-overview 15 s) | `constant-vus` |
| `spike` | todos abrindo o Game ao mesmo tempo (game/overview pesado) | `ramping-arrival-rate`, `startTime` coordenado |
| `mutations` | likes + mark-read idempotentes | `constant-arrival-rate` |

## Endpoints mais pesados (priorize na observação)

1. **`GET /game/overview`** — 6 queries, 4 window-function scans sobre `users` + ledger sem LIMIT. O mais caro.
2. **`GET /rankings`** (ambos scopes) — CTE com `ROW_NUMBER()` sobre todos os usuários.
3. **`GET /moments?scope=feed`** — 1 + 20 (N+1) + 20 assinaturas S3.
4. **`GET /manager/game-overview`** — 4 + 2 por run aberto.

## Risco número 1: pool de conexões

`DB_MAX_OPEN_CONNS=4` por container Lambda, idle = open. A Lambda serve 1 request por
container; a concorrência real vira **containers × 4** conexões no pooler Supavisor do
Supabase. Um pico de concorrência pode saturar o pooler antes de saturar a CPU da
Lambda. Acompanhe as conexões do pooler no Supabase durante todo o teste.

## Perfis e execução

Ver [`loadtest/k6/README.md`](../loadtest/k6/README.md). Perfis: `smoke` (validação),
`soak` (sustentado), `spike` (pico coordenado), `full` (≥ 5000 VUs, todos os cenários).
