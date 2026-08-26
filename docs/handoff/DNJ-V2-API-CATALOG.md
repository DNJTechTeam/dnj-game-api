# Catálogo de endpoints — DNJ Game API

Catálogo de todo endpoint de negócio da API: para que serve, que papel ele
exige, e qual jornada de teste E2E o exercita de ponta a ponta. Complementa
(não substitui) o contrato de frontend em
[`DNJ-V2-FRONTEND-INTEGRATION.md`](DNJ-V2-FRONTEND-INTEGRATION.md) — este
documento aqui é organizado por **papel/jornada**, não por tela, e por isso
vive num arquivo separado (o manifesto de frontend tem um gate de CI
bijetivo em `cmd/handoff-check` que não deve ganhar uma dimensão nova).

Toda alçada (`Papel` abaixo) é decidida na camada de service, nunca no
router — ver `docs/audits/PERMISSIONS-AUDIT-2026-08.md` para o mapeamento
completo. A coluna `operationId` referencia
`docs/openapi/dnj-v2.openapi.json`/`dnj-v2.operations.yaml`, onde está o
schema completo de request/response; aqui só o essencial para navegar.

A coluna `Jornada` aponta para o teste que exercita o endpoint via HTTP real
(Postgres real, `gin.Engine` completo) — ver evidência em
[`E2E-EVIDENCE-REPORT.md`](E2E-EVIDENCE-REPORT.md). "—" significa que o
endpoint é coberto por testes de integração dedicados
(`*_http_integration_test.go`) em vez de uma das três jornadas.

## Identidade & sessão (`/v2/auth`)

| Método + path | Propósito | Papel | Jornada |
|---|---|---|---|
| `POST /v2/auth/google` | Login/signup via Google ID token | público | — |
| `POST /v2/auth/signup` | Pede signup por e-mail; envia código de verificação | público | Default |
| `POST /v2/auth/signup/verify` | Troca o código por uma sessão (accessToken/refreshToken/csrfToken via cookie + body) | público | Default |
| `POST /v2/auth/refresh` | Renova o access token a partir do refresh token (cookie) + CSRF | público (requer cookie) | — |
| `POST /v2/auth/logout` | Revoga a sessão (refresh token) | requer cookie + CSRF | — |
| `GET /v2/auth/session` | Identidade do usuário autenticado | autenticado | Default |
| `PATCH /v2/auth/onboarding` | Completa CPF + telefone; `groupId` é opcional (pode entrar num grupo depois via convite) | autenticado | Default |

Onboarding legado (`POST /auth/onboarding`, `POST /auth/verification-code`,
`auth_router.go`) segue existindo para o fluxo antigo baseado em webhook de
assinatura, mas foi superado pelo self-service acima para contas novas; tem
cobertura própria em `auth_service_test.go` / `auth_handler_test.go`, sem
jornada dedicada aqui.

## Perfil, grupos e convites

| Método + path | Propósito | Papel | Jornada |
|---|---|---|---|
| `GET /v2/users/me` | Perfil atual | autenticado | — |
| `PATCH /v2/users/me` | Edita `name`/`mobilePhone` (só isso) | autenticado | Default |
| `PATCH`/`POST /v2/users/me/group` | Troca o grupo do usuário atual | autenticado | — |
| `GET /v2/groups` | Busca grupos por nome | autenticado | — |
| `POST /v2/groups` | Cria um grupo — self-service, qualquer autenticado (não achou o seu? cria) | autenticado | Default, Admin |
| `GET /v2/groups/me` | Grupo do usuário atual | autenticado | Default |
| `GET /v2/groups/me/members` | Membros do grupo atual | autenticado | Default |
| `POST /v2/groups/invites/consume` | Consome um código de convite e entra no grupo | autenticado | Default |
| `GET /v2/admin/groups/{groupId}/invites` | Lista convites do grupo (histórico) | ADMIN | Admin |
| `POST /v2/admin/groups/{groupId}/invites` | Cria um convite de uso único (código de 16 bytes, só o hash é persistido) | ADMIN | Default (setup), Admin |
| `POST /v2/admin/groups/{groupId}/invites/{inviteId}/renew` | Revoga o convite atual e emite um novo no lugar | ADMIN | Admin |
| `DELETE /v2/admin/groups/{groupId}/invites/{inviteId}` | Revoga um convite (idempotente se já revogado) | ADMIN | Admin |

## Conteúdo público (activities, spaces, agenda)

| Método + path | Propósito | Papel | Jornada |
|---|---|---|---|
| `GET /v2/spaces` | Lista espaços físicos publicados | público | — |
| `GET /v2/schedule` | Agenda pública de activities | público | Default |
| `GET /v2/activities` | Catálogo público de activities | público | Default |
| `GET /v2/activities/{activityId}` | Detalhe de uma activity publicada | público | — |
| `GET /v2/users/me/favorites` | Lista as activities favoritadas | autenticado | Default |
| `PUT /v2/users/me/favorites/{activityId}` | Favorita uma activity (204) | autenticado | Default |
| `DELETE /v2/users/me/favorites/{activityId}` | Remove o favorito (204) | autenticado | — |

## Activities/spaces — administração e ciclo de vida

| Método + path | Propósito | Papel | Jornada |
|---|---|---|---|
| `GET /v2/admin/spaces` | Lista espaços (visão administrativa) | ADMIN | — |
| `POST /v2/admin/spaces` | Cria um espaço | ADMIN | Default, EM, Admin (setup) |
| `PATCH /v2/admin/spaces/{spaceId}` | Edita um espaço | ADMIN | — |
| `GET /v2/admin/activities` | Lista activities (visão administrativa) | ADMIN | — |
| `POST /v2/admin/activities` | Cria uma activity — nasce em `draft` | ADMIN | Default, EM, Admin (setup) |
| `PATCH /v2/admin/activities/{activityId}` | Edita uma activity | ADMIN | — |
| `POST /v2/manager/activities/{activityId}/start` | Transiciona `draft`/`paused` → `active` | ADMIN ou EVENT_MANAGER atribuído | Default, EM, Admin (setup) |
| `POST /v2/manager/activities/{activityId}/pause` | Transiciona `active` → `paused` | ADMIN ou EVENT_MANAGER atribuído | — |
| `GET /v2/admin/activities/{activityId}/managers` | Lista os gestores responsáveis pela activity | ADMIN | Admin |
| `PUT /v2/admin/activities/{activityId}/managers/{userId}` | Atribui um Event Manager a uma activity | ADMIN | Default, EM, Admin (setup) |
| `DELETE /v2/admin/activities/{activityId}/managers/{userId}` | Remove a atribuição | ADMIN | Admin |

## Staff & papéis

| Método + path | Propósito | Papel | Jornada |
|---|---|---|---|
| `GET /v2/admin/staff` | Lista staff — sem `role` lista ADMIN+EVENT_MANAGER juntos; `role=ADMIN` ou `role=EVENT_MANAGER` filtra um dos dois | ADMIN | Admin |
| `PATCH /v2/admin/users/{userId}/role` | Promove/rebaixa entre `DEFAULT` e `EVENT_MANAGER` (nunca concede/remove ADMIN por aqui) | ADMIN | EM, Admin (promoção); Admin (rebaixe) |

## Jogo: runs, QR, ranking

| Método + path | Propósito | Papel | Jornada |
|---|---|---|---|
| `GET /v2/games` | Catálogo de games publicados (kind=competitive) | público | — |
| `GET /v2/games/{gameId}` | Detalhe de um game publicado | público | — |
| `GET /v2/rankings` | Ranking individual ou por grupo | público (também acessível autenticado) | Default |
| `GET /v2/game/overview` | Visão do jogador: ranking, posição atual, histórico de pontos | DEFAULT | Default |
| `GET /v2/activity-runs/current` | Run atual do jogador (204 se nenhum) | DEFAULT | Default |
| `GET /v2/participations/current` | Participation atual do jogador (204 se nenhuma) | DEFAULT | Default |
| `POST /v2/qr/validate` | Faz check-in via QR — cria a Participation | DEFAULT | Default, Admin (setup) |
| `GET /v2/manager/game-overview` | Painel do gestor: só as activities atribuídas a ele (ADMIN vê todas) | ADMIN ou EVENT_MANAGER | EM |
| `POST /v2/manager/runs` | Cria um run para uma activity gerenciável (kind=competitive, active) | ADMIN ou EVENT_MANAGER atribuído | EM, Default, Admin |
| `GET /v2/manager/runs/{runId}` | Detalhe do run — **prova de alçada**: 404 para gestor não-atribuído, 200 para Admin sempre | ADMIN ou EVENT_MANAGER atribuído | EM (404 negativo), Admin (200 superset) |
| `POST /v2/manager/runs/{runId}/qr` | Emite um QR válido para check-in | ADMIN ou EVENT_MANAGER atribuído | EM, Default, Admin |
| `POST /v2/manager/runs/{runId}/start` | Inicia o run | ADMIN ou EVENT_MANAGER atribuído | EM |
| `POST /v2/manager/runs/{runId}/pause` | Pausa o run | ADMIN ou EVENT_MANAGER atribuído | EM |
| `POST /v2/manager/runs/{runId}/resume` | Retoma o run | ADMIN ou EVENT_MANAGER atribuído | EM |
| `POST /v2/manager/runs/{runId}/results` | Finaliza o run com os resultados dos participantes | ADMIN ou EVENT_MANAGER atribuído | EM |
| `POST /v2/manager/runs/{runId}/cancel` | Cancela o run | ADMIN ou EVENT_MANAGER atribuído | — |

## Mídia e Moments

| Método + path | Propósito | Papel | Jornada |
|---|---|---|---|
| `POST /v2/media/upload-intents` | Devolve uma URL assinada (S3) para upload direto | DEFAULT | Default, Admin |
| `POST /v2/media/{mediaAssetId}/complete` | Confirma o upload, sanitiza/revalida a imagem, marca `available` | DEFAULT | Default, Admin |
| `GET /v2/moments?scope=feed\|mine\|group` | Lista moments — feed mostra `pending`+`approved` (só `rejected` some) | DEFAULT | Default |
| `POST /v2/moments` | Publica um moment — **nasce sempre `pending`**, mas já visível (curadoria não bloqueia o dia a dia) | DEFAULT | Default, Admin |
| `POST /v2/moments/{momentId}/likes` | Alterna curtida | DEFAULT | Default |
| `GET /v2/admin/moments/moderation?queue=general\|challenge` | Fila de curadoria — só itens `pending`, sem travar publicação | ADMIN | Admin |
| `POST /v2/admin/moments/{momentId}/moderation` | Ações: `approve` (tira da fila), `deny_points` (reverte prêmio), `delete_photo` (rejeita + apaga) | ADMIN | Admin |

## Notificações

| Método + path | Propósito | Papel | Jornada |
|---|---|---|---|
| `GET /v2/notifications/preferences` | Preferências de notificação do usuário | DEFAULT | Default |
| `PUT /v2/notifications/preferences` | Atualiza preferências (parcial) | DEFAULT | Default |
| `GET /v2/notifications` | Lista notificações do usuário | DEFAULT | Default |
| `POST /v2/notifications/{notificationId}/read` | Marca como lida | DEFAULT | Default |
| `POST /v2/admin/notifications` | Broadcast — sem `targetUserIds` alcança todo DEFAULT com announcement habilitado | ADMIN | Default, Admin |

## Infra / fora do escopo de jornada

| Método + path | Propósito | Nota |
|---|---|---|
| `GET /healthcheck`, `GET /v2/healthcheck`, `GET /v2/readiness` | Liveness/readiness | Smoke check na jornada Admin |
| `POST /subscriptions/webhook` | Ingestão de webhook de assinatura (integração externa) | Cobertura própria em `subscription_webhook_*_test.go`; fora das jornadas de papel |
| `GET/POST/PUT/PATCH/DELETE /tasks...` | CRUD de `Task` | Recurso-molde do template (`docs/new-resource-guide.md`), não é domínio de produto; fora das jornadas |
