# Relatório de evidências — E2E por jornada de papel

Gerado a partir de `docs/handoff/e2e-evidence/*.json`, produzido pelos testes `internal/app/services/e2e_default_journey_test.go`, `e2e_event_manager_journey_test.go` e `e2e_admin_journey_test.go`. Cada linha é uma chamada HTTP real, contra Postgres real (testcontainers), através do `gin.Engine` completo — não é uma simulação. Regenere com `go test ./internal/app/services/... -run TestE2E && go run ./cmd/e2e-report` sempre que uma jornada mudar.

## Prova de jurisdição: o mesmo endpoint, dois vereditos

`GET /v2/manager/runs/:id` aplicado ao run de **outro** gestor: um Event Manager par leva 404 (fora do seu escopo); um Admin, que também nunca foi atribuído àquela activity, recebe 200 (alçada global). Esta é a prova executável de que "alçadas de nível Admin sempre veem tudo" não é só uma leitura de código — é comportamento testado.

| Jornada | Ator | Chamada | Status | Prova |
|---|---|---|---|---|
| Event Manager | manager-a | `GET /v2/manager/runs/7b7df2d0-eab3-43e6-ac11-fc0f3599a6b6` | 404 | GET /v2/manager/runs/:id de um run alheio retorna 404 -- o escopo do gestor é só o que ele gerencia. |
| Admin | admin | `GET /v2/manager/runs/ed7f09a0-4097-48ca-aade-4c2b296e5034` | 200 | GET /v2/manager/runs/:id retorna 200 para Admin mesmo sem atribuição -- alçada global, o espelho do 404 que um gestor par recebe. |

## Jornada: Default (jogador)

36 chamadas registradas. Fonte: `docs/handoff/e2e-evidence/default.json`.

| # | Passo | Papel | Ator | Método | Path | Status | O que prova |
|---|---|---|---|---|---|---|---|
| 1 | admin promove usuário a EVENT_MANAGER | ADMIN | admin | PATCH | `/v2/admin/users/31/role` | 200 | PATCH /v2/admin/users/:userId/role promove DEFAULT->EVENT_MANAGER (200). |
| 2 | jogador pede o signup por e-mail | PUBLIC | player | POST | `/v2/auth/signup` | 200 | POST /v2/auth/signup dispara um código de verificação por e-mail (200). |
| 3 | jogador confirma o código | PUBLIC | player | POST | `/v2/auth/signup/verify` | 200 | POST /v2/auth/signup/verify troca o código por uma sessão (accessToken) e cria a conta (200). |
| 4 | jogador confere a própria sessão | DEFAULT | player | GET | `/v2/auth/session` | 200 | GET /v2/auth/session confirma a identidade (200). |
| 5 | jogador completa o onboarding (sem grupo) | DEFAULT | player | PATCH | `/v2/auth/onboarding` | 200 | PATCH /v2/auth/onboarding aceita groupId omitido -- escolher um grupo é opcional e pode vir depois (200). |
| 6 | admin cria um convite para o grupo | ADMIN | admin | POST | `/v2/admin/groups/3/invites` | 201 | POST .../invites cria um convite de uso único (201). |
| 7 | jogador consome o convite e entra no grupo | DEFAULT | player | POST | `/v2/groups/invites/consume` | 200 | POST /v2/groups/invites/consume vincula o jogador ao grupo do convite (200). |
| 8 | jogador vê o próprio grupo | DEFAULT | player | GET | `/v2/groups/me` | 200 | GET /v2/groups/me confirma a associação (200). |
| 9 | jogador lista os membros do grupo | DEFAULT | player | GET | `/v2/groups/me/members` | 200 | GET /v2/groups/me/members lista o próprio jogador como membro (200). |
| 10 | jogador atualiza o próprio perfil | DEFAULT | player | PATCH | `/v2/users/me` | 200 | PATCH /v2/users/me só permite editar name e mobilePhone (200). |
| 11 | admin cria o espaço default-journey | ADMIN | admin | POST | `/v2/admin/spaces` | 201 | POST /v2/admin/spaces cria um espaço com sucesso (201). |
| 12 | admin cria a activity default-journey | ADMIN | admin | POST | `/v2/admin/activities` | 201 | POST /v2/admin/activities cria uma activity em draft (201). |
| 13 | admin atribui o gestor à activity default-journey | ADMIN | admin | PUT | `/v2/admin/activities/de7439ff-2199-40bc-ae8a-15c87cc6f435/managers/31` | 200 | PUT .../managers/:userId torna este Event Manager responsável pela activity (200). |
| 14 | gestor inicia a activity default-journey | EVENT_MANAGER | manager | POST | `/v2/manager/activities/de7439ff-2199-40bc-ae8a-15c87cc6f435/start` | 200 | POST /v2/manager/activities/:id/start transiciona draft->active (200). |
| 15 | jogador vê a agenda pública | DEFAULT | player | GET | `/v2/schedule` | 200 | GET /v2/schedule lista activities publicadas, sem exigir autenticação (200). |
| 16 | jogador lista activities | DEFAULT | player | GET | `/v2/activities` | 200 | GET /v2/activities lista o catálogo público (200). |
| 17 | jogador favorita a activity | DEFAULT | player | PUT | `/v2/users/me/favorites/de7439ff-2199-40bc-ae8a-15c87cc6f435` | 204 | PUT .../favorites/:activityId marca a activity como favorita (204). |
| 18 | jogador lista os favoritos | DEFAULT | player | GET | `/v2/users/me/favorites?page=1` | 200 | GET .../favorites confirma a activity favoritada (200). |
| 19 | gestor cria o run para o jogador entrar | EVENT_MANAGER | manager | POST | `/v2/manager/runs` | 201 | POST /v2/manager/runs cria o run que o jogador vai escanear (201). |
| 20 | gestor gera o QR do run | EVENT_MANAGER | manager | POST | `/v2/manager/runs/bce0241e-97cf-46cb-8b78-24d35a5347fb/qr` | 201 | POST .../qr emite o QR que o jogador vai validar (201). |
| 21 | jogador faz check-in via QR | DEFAULT | player | POST | `/v2/qr/validate` | 201 | POST /v2/qr/validate cria a Participation do jogador no run (201). |
| 22 | jogador vê o próprio run atual | DEFAULT | player | GET | `/v2/activity-runs/current` | 200 | GET /v2/activity-runs/current mostra o run em que o jogador está participando (200). |
| 23 | jogador vê a própria participação atual | DEFAULT | player | GET | `/v2/participations/current` | 200 | GET /v2/participations/current mostra a Participation recém-criada (200). |
| 24 | player pede uma intenção de upload | DEFAULT | player | POST | `/v2/media/upload-intents` | 201 | POST /v2/media/upload-intents devolve uma URL assinada para upload direto ao storage (201). |
| 25 | player confirma o upload | DEFAULT | player | POST | `/v2/media/e1049ab7-9245-4c9e-b146-0a8975b09395/complete` | 200 | POST /v2/media/:id/complete valida a imagem e marca o asset como available (200). |
| 26 | player publica um moment | DEFAULT | player | POST | `/v2/moments` | 201 | POST /v2/moments nasce pending e já aparece no feed (201). |
| 27 | jogador vê o próprio moment no feed | DEFAULT | player | GET | `/v2/moments?scope=feed` | 200 | GET /v2/moments?scope=feed já mostra o moment (pending e approved aparecem -- só rejected some). |
| 28 | jogador curte o próprio moment | DEFAULT | player | POST | `/v2/moments/5a585cdd-43a8-4519-9b0c-8656f4be0639/likes` | 200 | POST /v2/moments/:momentId/likes alterna a curtida (200). |
| 29 | jogador vê seu overview no jogo | DEFAULT | player | GET | `/v2/game/overview` | 200 | GET /v2/game/overview mostra rankings e o histórico de pontos do jogador (200). |
| 30 | jogador vê o ranking individual | DEFAULT | player | GET | `/v2/rankings?scope=individual&page=1` | 200 | GET /v2/rankings é público mas também acessível autenticado (200). |
| 31 | jogador vê as preferências de notificação | DEFAULT | player | GET | `/v2/notifications/preferences` | 200 | GET /v2/notifications/preferences retorna os defaults (200). |
| 32 | jogador desliga notificações de pontos | DEFAULT | player | PUT | `/v2/notifications/preferences` | 200 | PUT /v2/notifications/preferences aceita atualização parcial (200). |
| 33 | admin avisa a todos (para o jogador ler) | ADMIN | admin | POST | `/v2/admin/notifications` | 201 | POST /v2/admin/notifications alcança o jogador recém-cadastrado (201). |
| 34 | jogador lista as próprias notificações | DEFAULT | player | GET | `/v2/notifications` | 200 | GET /v2/notifications mostra o aviso do admin, ainda não lido (200). |
| 35 | jogador marca a notificação como lida | DEFAULT | player | POST | `/v2/notifications/aaf2efe4-a69f-459e-b0d1-c44f4cc9f428/read` | 200 | POST .../read marca a notificação como lida (200). |
| 36 | jogador tenta acessar a tela de staff | DEFAULT | player | GET | `/v2/admin/staff` | 403 | GET /v2/admin/staff rejeita um jogador comum com 403 FORBIDDEN. |

## Jornada: Event Manager

24 chamadas registradas. Fonte: `docs/handoff/e2e-evidence/event_manager.json`.

| # | Passo | Papel | Ator | Método | Path | Status | O que prova |
|---|---|---|---|---|---|---|---|
| 1 | admin promove usuário a EVENT_MANAGER | ADMIN | admin | PATCH | `/v2/admin/users/34/role` | 200 | PATCH /v2/admin/users/:userId/role promove DEFAULT->EVENT_MANAGER (200). |
| 2 | admin promove usuário a EVENT_MANAGER | ADMIN | admin | PATCH | `/v2/admin/users/35/role` | 200 | PATCH /v2/admin/users/:userId/role promove DEFAULT->EVENT_MANAGER (200). |
| 3 | admin cria o espaço em-journey-a | ADMIN | admin | POST | `/v2/admin/spaces` | 201 | POST /v2/admin/spaces cria um espaço com sucesso (201). |
| 4 | admin cria a activity em-journey-a | ADMIN | admin | POST | `/v2/admin/activities` | 201 | POST /v2/admin/activities cria uma activity em draft (201). |
| 5 | admin atribui o gestor à activity em-journey-a | ADMIN | admin | PUT | `/v2/admin/activities/ac255ed6-7947-4069-9ecd-6462c83f3ed5/managers/34` | 200 | PUT .../managers/:userId torna este Event Manager responsável pela activity (200). |
| 6 | gestor inicia a activity em-journey-a | EVENT_MANAGER | manager | POST | `/v2/manager/activities/ac255ed6-7947-4069-9ecd-6462c83f3ed5/start` | 200 | POST /v2/manager/activities/:id/start transiciona draft->active (200). |
| 7 | admin cria o espaço em-journey-b | ADMIN | admin | POST | `/v2/admin/spaces` | 201 | POST /v2/admin/spaces cria um espaço com sucesso (201). |
| 8 | admin cria a activity em-journey-b | ADMIN | admin | POST | `/v2/admin/activities` | 201 | POST /v2/admin/activities cria uma activity em draft (201). |
| 9 | admin atribui o gestor à activity em-journey-b | ADMIN | admin | PUT | `/v2/admin/activities/e9fefd36-1cf1-485b-861e-9d3cb200be59/managers/35` | 200 | PUT .../managers/:userId torna este Event Manager responsável pela activity (200). |
| 10 | gestor inicia a activity em-journey-b | EVENT_MANAGER | manager | POST | `/v2/manager/activities/e9fefd36-1cf1-485b-861e-9d3cb200be59/start` | 200 | POST /v2/manager/activities/:id/start transiciona draft->active (200). |
| 11 | gestor A vê seu próprio painel | EVENT_MANAGER | manager-a | GET | `/v2/manager/game-overview` | 200 | GET /v2/manager/game-overview só lista activities atribuídas a este gestor (não vê a activity do gestor B). |
| 12 | gestor A cria um run | EVENT_MANAGER | manager-a | POST | `/v2/manager/runs` | 201 | POST /v2/manager/runs cria um run para uma activity própria (201). |
| 13 | gestor A gera QR do run | EVENT_MANAGER | manager-a | POST | `/v2/manager/runs/0704d4d0-d432-4c36-8d8a-69903bd07c96/qr` | 201 | POST .../qr emite um QR válido para check-in (201). |
| 14 | gestor A vê o detalhe do próprio run | EVENT_MANAGER | manager-a | GET | `/v2/manager/runs/0704d4d0-d432-4c36-8d8a-69903bd07c96` | 200 | GET /v2/manager/runs/:id retorna 200 para o run do próprio gestor. |
| 15 | gestor A inicia o run | EVENT_MANAGER | manager-a | POST | `/v2/manager/runs/0704d4d0-d432-4c36-8d8a-69903bd07c96/start` | 200 | POST .../start transiciona scheduled->active (200). |
| 16 | gestor A pausa o run | EVENT_MANAGER | manager-a | POST | `/v2/manager/runs/0704d4d0-d432-4c36-8d8a-69903bd07c96/pause` | 200 | POST .../pause transiciona active->paused (200). |
| 17 | gestor A retoma o run | EVENT_MANAGER | manager-a | POST | `/v2/manager/runs/0704d4d0-d432-4c36-8d8a-69903bd07c96/resume` | 200 | POST .../resume transiciona paused->active (200). |
| 18 | gestor A finaliza o run (sem participantes) | EVENT_MANAGER | manager-a | POST | `/v2/manager/runs/0704d4d0-d432-4c36-8d8a-69903bd07c96/results` | 200 | POST .../results finaliza o run mesmo sem check-ins (200). |
| 19 | gestor B cria um run na própria activity | EVENT_MANAGER | manager-b | POST | `/v2/manager/runs` | 201 | POST /v2/manager/runs cria um run para a activity do gestor B (201). |
| 20 | gestor A tenta ver o run do gestor B | EVENT_MANAGER | manager-a | GET | `/v2/manager/runs/7b7df2d0-eab3-43e6-ac11-fc0f3599a6b6` | 404 | GET /v2/manager/runs/:id de um run alheio retorna 404 -- o escopo do gestor é só o que ele gerencia. |
| 21 | gestor tenta listar staff | EVENT_MANAGER | manager-a | GET | `/v2/admin/staff` | 403 | Superfície admin-only rejeita Event Manager com 403 FORBIDDEN (listar staff). |
| 22 | gestor tenta criar espaço | EVENT_MANAGER | manager-a | POST | `/v2/admin/spaces` | 403 | Superfície admin-only rejeita Event Manager com 403 FORBIDDEN (criar espaço). |
| 23 | gestor tenta moderar um moment | EVENT_MANAGER | manager-a | POST | `/v2/admin/moments/fb41bbc4-4bf6-49e0-82ae-3d4d2d5bc969/moderation` | 403 | Superfície admin-only rejeita Event Manager com 403 FORBIDDEN (moderar um moment). |
| 24 | gestor tenta disparar notificação broadcast | EVENT_MANAGER | manager-a | POST | `/v2/admin/notifications` | 403 | Superfície admin-only rejeita Event Manager com 403 FORBIDDEN (disparar notificação broadcast). |

## Jornada: Admin

33 chamadas registradas. Fonte: `docs/handoff/e2e-evidence/admin.json`.

| # | Passo | Papel | Ator | Método | Path | Status | O que prova |
|---|---|---|---|---|---|---|---|
| 1 | smoke check de healthcheck | PUBLIC | - | GET | `/v2/healthcheck` | 200 | GET /v2/healthcheck responde sem autenticação (200). |
| 2 | admin promove usuário a EVENT_MANAGER | ADMIN | admin | PATCH | `/v2/admin/users/27/role` | 200 | PATCH /v2/admin/users/:userId/role promove DEFAULT->EVENT_MANAGER (200). |
| 3 | admin promove usuário a EVENT_MANAGER | ADMIN | admin | PATCH | `/v2/admin/users/28/role` | 200 | PATCH /v2/admin/users/:userId/role promove DEFAULT->EVENT_MANAGER (200). |
| 4 | admin lista todo o staff (sem filtro) | ADMIN | admin | GET | `/v2/admin/staff` | 200 | GET /v2/admin/staff sem role lista ADMIN + EVENT_MANAGER juntos. |
| 5 | admin filtra staff por role=ADMIN | ADMIN | admin | GET | `/v2/admin/staff?role=ADMIN` | 200 | GET /v2/admin/staff?role=ADMIN só lista administradores. |
| 6 | admin cria o espaço admin-journey-y | ADMIN | admin | POST | `/v2/admin/spaces` | 201 | POST /v2/admin/spaces cria um espaço com sucesso (201). |
| 7 | admin cria a activity admin-journey-y | ADMIN | admin | POST | `/v2/admin/activities` | 201 | POST /v2/admin/activities cria uma activity em draft (201). |
| 8 | admin atribui o gestor à activity admin-journey-y | ADMIN | admin | PUT | `/v2/admin/activities/1279971a-7fd4-4803-b95c-be1050ce3f6a/managers/28` | 200 | PUT .../managers/:userId torna este Event Manager responsável pela activity (200). |
| 9 | gestor inicia a activity admin-journey-y | EVENT_MANAGER | manager | POST | `/v2/manager/activities/1279971a-7fd4-4803-b95c-be1050ce3f6a/start` | 200 | POST /v2/manager/activities/:id/start transiciona draft->active (200). |
| 10 | gestor Y cria um run | EVENT_MANAGER | manager-y | POST | `/v2/manager/runs` | 201 | POST /v2/manager/runs cria um run próprio do gestor Y (201). |
| 11 | admin acessa o run do gestor Y sem estar atribuído | ADMIN | admin | GET | `/v2/manager/runs/ed7f09a0-4097-48ca-aade-4c2b296e5034` | 200 | GET /v2/manager/runs/:id retorna 200 para Admin mesmo sem atribuição -- alçada global, o espelho do 404 que um gestor par recebe. |
| 12 | gestor Y gera QR do run | EVENT_MANAGER | manager-y | POST | `/v2/manager/runs/ed7f09a0-4097-48ca-aade-4c2b296e5034/qr` | 201 | POST .../qr emite um QR para check-in de jogadores (201). |
| 13 | admin lista gestores da activity Y | ADMIN | admin | GET | `/v2/admin/activities/1279971a-7fd4-4803-b95c-be1050ce3f6a/managers` | 200 | GET .../managers lista o gestor Y como responsável. |
| 14 | admin remove o gestor Y da activity | ADMIN | admin | DELETE | `/v2/admin/activities/1279971a-7fd4-4803-b95c-be1050ce3f6a/managers/28` | 204 | DELETE .../managers/:userId desvincula o gestor (204). |
| 15 | admin rebaixa o gestor Y para DEFAULT | ADMIN | admin | PATCH | `/v2/admin/users/28/role` | 200 | PATCH .../role rebaixa EVENT_MANAGER->DEFAULT depois que os assignments foram removidos (200). |
| 16 | jogador faz check-in via QR | DEFAULT | player | POST | `/v2/qr/validate` | 201 | POST /v2/qr/validate cria a Participation usada pelos moments seguintes. |
| 17 | player pede uma intenção de upload | DEFAULT | player | POST | `/v2/media/upload-intents` | 201 | POST /v2/media/upload-intents devolve uma URL assinada para upload direto ao storage (201). |
| 18 | player confirma o upload | DEFAULT | player | POST | `/v2/media/a0deefca-f666-41eb-9c0b-c9976d531d03/complete` | 200 | POST /v2/media/:id/complete valida a imagem e marca o asset como available (200). |
| 19 | player publica um moment | DEFAULT | player | POST | `/v2/moments` | 201 | POST /v2/moments nasce pending e já aparece no feed (201). |
| 20 | player pede uma intenção de upload | DEFAULT | player | POST | `/v2/media/upload-intents` | 201 | POST /v2/media/upload-intents devolve uma URL assinada para upload direto ao storage (201). |
| 21 | player confirma o upload | DEFAULT | player | POST | `/v2/media/405cc847-0c44-4eb6-b4f6-fa5d519a5efd/complete` | 200 | POST /v2/media/:id/complete valida a imagem e marca o asset como available (200). |
| 22 | player publica um moment | DEFAULT | player | POST | `/v2/moments` | 201 | POST /v2/moments nasce pending e já aparece no feed (201). |
| 23 | admin lista a fila de moderação (challenge) | ADMIN | admin | GET | `/v2/admin/moments/moderation?queue=challenge&page=1` | 200 | GET .../moderation?queue=challenge mostra o moment vinculado à participation, ainda pendente. |
| 24 | admin aprova o primeiro moment | ADMIN | admin | POST | `/v2/admin/moments/4445d21f-e2c5-4bad-ade6-18d68d70149e/moderation` | 200 | POST .../moderation com action=approve tira o moment da fila pendente (200). |
| 25 | admin lista a fila de moderação (general) | ADMIN | admin | GET | `/v2/admin/moments/moderation?queue=general&page=1` | 200 | GET .../moderation?queue=general mostra o moment espontâneo, ainda pendente. |
| 26 | admin rejeita o segundo moment | ADMIN | admin | POST | `/v2/admin/moments/830118f7-5da4-48f7-b8f2-f700aa69ec0b/moderation` | 200 | POST .../moderation com action=delete_photo derruba a foto e rejeita o moment (200). |
| 27 | jogador cria o próprio grupo | DEFAULT | player | POST | `/v2/groups` | 201 | POST /v2/groups é self-service -- qualquer autenticado pode criar (201). |
| 28 | admin cria um convite para o grupo | ADMIN | admin | POST | `/v2/admin/groups/2/invites` | 201 | POST .../invites cria um convite de uso único (201). |
| 29 | admin renova o convite | ADMIN | admin | POST | `/v2/admin/groups/2/invites/1/renew` | 201 | POST .../renew revoga o convite antigo e emite um novo (201). |
| 30 | admin revoga o convite renovado | ADMIN | admin | DELETE | `/v2/admin/groups/2/invites/2` | 204 | DELETE .../invites/:inviteId revoga o convite (204). |
| 31 | admin lista convites do grupo | ADMIN | admin | GET | `/v2/admin/groups/2/invites` | 200 | GET .../invites lista o histórico (original + renovado, ambos revogados). |
| 32 | admin dispara notificação broadcast | ADMIN | admin | POST | `/v2/admin/notifications` | 201 | POST /v2/admin/notifications sem targetUserIds alcança todo mundo com announcement habilitado (201). |
| 33 | jogador vê a notificação recebida | DEFAULT | player | GET | `/v2/notifications` | 200 | GET /v2/notifications do jogador contém o broadcast enviado pelo admin. |

