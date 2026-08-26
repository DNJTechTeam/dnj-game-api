# Auditoria de alçadas de permissão — agosto/2026

## Contexto

Durante o trabalho na moderação de moments desta iteração, foi identificado
e corrigido um bug real de alçada: `GET /v2/admin/staff` só aceitava
`role=EVENT_MANAGER` — um Admin não conseguia listar outros Admins nem "todo
o staff" de uma vez. Isso levantou a pergunta maior: existe algum outro
lugar do sistema em que um papel de nível mais alto (Admin) é bloqueado por
uma checagem pensada para um papel mais baixo, ou em que um Event Manager
enxerga/mexe em algo fora do seu escopo?

Esta auditoria mapeou **toda** checagem de `role` em `internal/app/services/`
e nos middlewares, e complementa a leitura de código com uma prova executável
(não apenas estática): os testes de jornada em
`internal/app/services/e2e_*_journey_test.go` exercitam o mesmo endpoint
(`GET /v2/manager/runs/:id`) como um Event Manager e como um Admin sobre um
run que nenhum dos dois criou, e comparam os dois resultados lado a lado —
ver a seção "Prova de jurisdição" em
[`E2E-EVIDENCE-REPORT.md`](../handoff/E2E-EVIDENCE-REPORT.md).

## Achado principal: nenhuma checagem de role nos routers

`internal/infrastructure/api/middlewares/auth_middlewares.go`
(`AuthenticationMiddleware`) só valida o JWT e injeta o `userId` no
contexto — nunca lê nem checa `Role`. Nenhum arquivo em
`internal/presentation/api/routers/` referencia papéis. **Toda** decisão de
autorização vive na camada de `service`, chamada explicitamente por cada
método.

## Tabela de alçadas

| Onde | Exige | Admin tem bypass? | Veredito |
|---|---|---|---|
| `game_service.go:136` `participant()` | `role == DEFAULT` | Não — proposital | OK (ver nota 1) |
| `game_service.go:347` `manager()` | `role IN (ADMIN, EVENT_MANAGER)`; devolve `global = role==ADMIN` | Sim, via `global` threading em 7 call sites (`FindRunForManager`, `ListManageableGames`, `FindOpenRunForManager`, `FindManageableActivityForUpdate`) | OK |
| `activity_service.go:68` `transition()` | `role IN (ADMIN, EVENT_MANAGER)`; `FindAuthorizedForUpdate(..., global=role==ADMIN)` | Sim | OK |
| `media_moment_helpers.go:79` `requireDefaultActor()` | `role == DEFAULT` | Não — proposital | OK (ver nota 1) |
| `media_moment_helpers.go:116` `requireAdminActor()` | `role == ADMIN` | N/A (é o próprio Admin) | OK — EM também é excluído aqui (ver nota 2) |
| `admin_installation_service.go:70` `authorizeAdmin()` | `role == ADMIN` | N/A | OK |
| `admin_installation_service.go:564` `ListStaff()` | — | — | **Corrigido nesta iteração** (ver abaixo) |
| `admin_installation_service.go:616` `UpdateUserRole()` | não permite alterar quem já é ADMIN | — | OK, proposital (guarda contra escalonar/rebaixar Admin por esta rota) |
| `admin_installation_service.go:702` `AssignManager()` | alvo precisa ser `EVENT_MANAGER` | — | OK, proposital (Admin já tem acesso global via `manager()`, não precisa de assignment) |
| `group_invite_service.go:51` `requireAdmin()` | `role == ADMIN` | N/A | OK |

## Bug corrigido: `ListStaff`

Antes: `if filter == nil \|\| filter.Role != string(RoleEventManager) { 400 }`,
sempre chamando `ListByRole(EVENT_MANAGER)` — não dava para listar Admins nem
"todo o staff".

Depois: `filter.Role` vazio lista `ADMIN + EVENT_MANAGER` juntos; `ADMIN` ou
`EVENT_MANAGER` explícito filtra por um dos dois; qualquer outro valor
continua `400 INVALID_REQUEST`. `UserRepositoryInterface.ListByRole` passou
a aceitar `[]UserRole` (antes um único `UserRole`). Prova viva: jornada do
Admin, passos "admin lista todo o staff (sem filtro)" e "admin filtra staff
por role=ADMIN" — ver evidência.

## Duas questões de produto em aberto (não são bugs)

1. **Admin também deveria poder agir como jogador?** Hoje
   `requireDefaultActor`/`participant()` excluem Admin/EM de criar moment,
   upload de mídia, curtir, participar de challenge e ler notificações
   próprias — sempre com `403 FORBIDDEN`. Isso é consistente (staff não é
   jogador), mas significa que uma conta Admin não consegue usar essas telas
   nem para QA manual. Se o produto quiser isso, é uma mudança deliberada
   nesses dois helpers, não um bug de alçada.
2. **Event Manager deveria poder moderar moments/mandar broadcast dos
   próprios eventos?** Hoje `ListModeration`/`Moderate`/`AdminSend` exigem
   `role == ADMIN` estrito — um EM que criou a activity e o run não pode
   moderar as fotos dela nem avisar os participantes. Também não é um bug
   (o design atual concentra moderação/broadcast em Admin), mas pode valer a
   pena revisitar se o volume de eventos por EM crescer.

## Como reproduzir a prova

```bash
go test ./internal/app/services/... -run TestE2E
go run ./cmd/e2e-report
```

Isso regenera `docs/handoff/e2e-evidence/*.json` e
`docs/handoff/E2E-EVIDENCE-REPORT.md`, incluindo a seção "Prova de
jurisdição" que compara o 404 (Event Manager em run alheio) com o 200
(Admin no mesmo run).
