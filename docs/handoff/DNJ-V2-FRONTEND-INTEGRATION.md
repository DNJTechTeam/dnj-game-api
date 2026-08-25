# Handoff final de integração — DNJ Game API V2

Este é o documento canônico de handoff da V2 para o frontend. Uma pessoa com
acesso somente a este artefato (Markdown + manifesto JSON + página HTML
publicada + OpenAPI 3.0.3, todos gerados juntos e nunca divergentes — ver
"Como isto é mantido consistente" no final) deve conseguir migrar o
frontend, validar cada fluxo e executar o rollback, sem ler o histórico das
iterações nem o código interno da API.

O clone local do frontend foi consultado somente como fonte de descoberta em
todas as iterações. Nenhum arquivo dele foi alterado, commitado ou enviado —
este handoff não depende de permissão de escrita nesse repositório.

- **Página publicada** (gerada por CI a cada deploy):
  `develop`: <https://dnjtechteam.github.io/dnj-game-api/develop/frontend-integration/> ·
  `produção`: <https://dnjtechteam.github.io/dnj-game-api/production/frontend-integration/>
- **Manifesto legível por máquina**: [`dnj-v2-frontend-integration.json`](dnj-v2-frontend-integration.json)
- **Contrato**: [`docs/openapi/dnj-v2.openapi.yaml`](../openapi/dnj-v2.openapi.yaml) (OpenAPI 3.0.3)

## Faça isto, nesta ordem

1. Leia "Antes de integrar qualquer tela" abaixo — contrato comum a toda a
   V2 (auth, erro, idempotência, paginação). Sem isso, todo fluxo abaixo vai
   dar errado da mesma forma.
2. Copie os quatro helpers de referência (cliente autenticado, refresh/CSRF,
   `Idempotency-Key`, UTC↔fuso-local) para o seu projeto — todo fluxo depende
   de pelo menos um deles.
3. Siga a ordem de rollout por iteração na tabela "Backlog granular" — as
   dependências entre fluxos já estão marcadas; não pule uma dependência
   `blocked`/`pending`.
4. Para cada fluxo `ready`, implemente na ordem: rota V1→V2 → pré-requisitos
   → sequência de chamadas → snippet TypeScript → estados de UI → testes que
   você deve criar → critério de pronto (tudo isso já está na tabela de
   fluxos e no manifesto JSON).
5. Antes de remover qualquer chamada V1/alias, confirme que a V2
   equivalente está em produção e que não há mais tráfego real na rota
   antiga — ver "Remoção segura de aliases" abaixo.
6. Ao terminar um fluxo, marque-o `done` no seu rastreamento local (o estado
   no manifesto JSON deste repositório é atualizado pelo time de backend a
   partir das iterações; o frontend mantém seu próprio rastreamento de
   adoção).

**Trabalho obrigatório desta entrega**: os 24 fluxos da tabela de backlog,
todos já `ready` (nenhum bloqueio de backend pendente).
**Limpeza posterior** (não bloqueia a migração, mas deve ser feita): remover
aliases/proxies V1 listados em "Remoção segura de aliases" depois que a
adoção da V2 estiver confirmada.
**Fora de escopo desta entrega**: qualquer tela nova que não exista hoje no
frontend; os fluxos de escopo `admin-tooling` (usados só pelo painel
administrativo/gestor, não pelo app do jogador) estão documentados aqui por
completude, mas não bloqueiam o app do jogador.

## Antes de integrar qualquer tela

- **Base path**: `/v2`. `/v1` permanece disponível até existir evidência de
  migração completa do consumidor — sem data de desligamento definida (ver
  "Decisões fechadas" em [`docs/implementation/DNJ-V2-STATUS.md`](../implementation/DNJ-V2-STATUS.md)).
- **Autenticação**: JWT de identidade (sem tenant/customer/papel amplo no
  token), via header `Authorization: Bearer <token>` **ou** cookie
  `identity_token` — ambos funcionam. Fluxo completo (Google, refresh,
  logout, onboarding) em [`docs/auth-and-tokens.md`](../auth-and-tokens.md).
- **Envelope de erro**, igual em toda a V2:
  ```json
  { "code": "STRING_CURTA", "message": "legível", "details": null, "requestId": "..." }
  ```
  `requestId` correlaciona com o evento estruturado `http_request_completed`
  do lado do servidor — inclua-o em todo report de bug.
- **Idempotência**: toda escrita mutante exige header `Idempotency-Key` com
  um UUID v4 gerado pelo cliente, único por ação do usuário (nunca reutilize
  entre ações, nunca derive do payload). A mesma chave + mesmo payload
  repete o resultado original; a mesma chave + payload diferente retorna
  `409 IDEMPOTENCY_KEY_REUSED`.
- **IDs de recurso** trafegam como string no JSON, mesmo quando armazenados
  como inteiro no banco — nunca assuma número, nunca faça aritmética neles.
- **Paginação**: 1-indexada na borda HTTP (`?page=1`), envelope
  `{data,pagination:{currentPage,hasNextPage,limit}}`. Listagens ordenadas
  por tempo real (galeria de Moments) usam `cursor` opaco em vez de `page` —
  nunca decodifique ou construa um cursor no cliente, apenas repasse o que o
  servidor devolveu.
- **UTC no transporte, fuso local só na apresentação**: toda data/hora que
  atravessa a rede é UTC (`RFC3339`, sufixo `Z`); nunca envie um timestamp
  com offset fixo do cliente. Converta para o fuso do usuário apenas na
  última milha, ao renderizar.

## Helpers de referência (TypeScript)

Copie estes cinco helpers como ponto de partida — cada fluxo da tabela
abaixo assume pelo menos um deles.

### Google Identity Services — obtendo o `idToken` para `/auth/google`

O backend nunca gera o `idToken`; ele vem do SDK do Google no cliente.

```html
<script src="https://accounts.google.com/gsi/client" async defer></script>
```

```typescript
google.accounts.id.initialize({
  client_id: GOOGLE_CLIENT_ID, // mesmo valor do GOOGLE_CLIENT_ID configurado no backend
  callback: (response) => {
    // response.credential JÁ é o idToken — envie direto pro POST /auth/google
    apiFetch("/auth/google", { method: "POST", body: JSON.stringify({ idToken: response.credential }) });
  },
});
google.accounts.id.renderButton(buttonEl, { theme: "outline", size: "large" });
// ou, para One Tap em vez de botão:
google.accounts.id.prompt();
```

No Google Cloud Console, o OAuth Client (tipo **Web application**) precisa
ter toda origem do frontend (produção e `http://localhost:3000` em dev) em
**Authorized JavaScript origins** — sem isso o `initialize` falha silenciosamente.

### Cliente autenticado com refresh automático e CSRF

```typescript
const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8081/v2";

async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: "include", // envia o cookie identity_token
    headers: { "Content-Type": "application/json", ...init.headers },
  });
  if (response.status === 401) {
    const refreshed = await apiFetch("/auth/refresh", { method: "POST" });
    if (refreshed.ok) return apiFetch(path, init); // uma única retentativa
  }
  return response;
}
```

### CSRF (só necessário para os endpoints de `/auth` que usam cookie)

```typescript
function withCsrf(init: RequestInit, csrfToken: string): RequestInit {
  return { ...init, headers: { ...init.headers, "X-CSRF-Token": csrfToken } };
}
```

### `Idempotency-Key` por ação do usuário

```typescript
function newIdempotencyKey(): string {
  return crypto.randomUUID(); // gere um valor NOVO a cada toque/submit
}

async function mutate(path: string, method: "POST" | "PUT", body: unknown) {
  return apiFetch(path, {
    method,
    body: JSON.stringify(body),
    headers: { "Idempotency-Key": newIdempotencyKey() },
  });
}
```

### UTC ↔ fuso local, sem timezone fixo

```typescript
// Nunca "America/Sao_Paulo" fixo — use o fuso real do dispositivo.
function toLocalDisplay(utcIso: string): string {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(utcIso));
}

function toUtcIso(localDate: Date): string {
  return localDate.toISOString(); // Date já normaliza para UTC internamente
}
```

## Backlog granular (25 fluxos, cobertura 1:1 com as 69 operações publicadas)

Fonte de verdade estruturada: [`dnj-v2-frontend-integration.json`](dnj-v2-frontend-integration.json)
(validado em CI contra o OpenAPI pelo `cmd/handoff-check` — `make openapi`
falha se um `operationId` publicado ficar sem fluxo, ou se um fluxo
referenciar um `operationId` que não existe).

| Fluxo | Escopo | Iteração | Prioridade | Estado | Dependências | Dono |
|---|---|---:|---|---|---|---|
| Tela de login | frontend | 2 | P0 | ready | — | frontend |
| Tela de cadastro/login por email | frontend | 11 | P0 | ready | — | frontend |
| Bootstrap de app (sessão + refresh) | frontend | 2 | P0 | ready | Tela de login | frontend |
| Completar cadastro (onboarding) | frontend | 2 | P0 | ready | Tela de login | frontend |
| Ação de sair | frontend | 2 | P1 | ready | Bootstrap de app | frontend |
| Meu perfil | frontend | 3 | P1 | ready | Bootstrap de app | frontend |
| Meu grupo e membros | frontend | 3 | P1 | ready | Bootstrap de app | frontend |
| Entrar em grupo por convite/código | frontend | 3 | P2 | ready | Bootstrap de app | frontend |
| Painel admin — convites de grupo | admin-tooling | 3 | P2 | ready | — | admin-panel |
| Agenda e detalhe de atividade | frontend | 5 | P0 | ready | Bootstrap de app | frontend |
| Favoritar atividade | frontend | 5 | P1 | ready | Agenda e detalhe | frontend |
| Abertura da tela Game (3 GETs em paralelo) | frontend | 6 | P0 | ready | Bootstrap de app | frontend |
| Scanner de QR | frontend | 6 | P0 | ready | Abertura da tela Game | frontend |
| Catálogo de jogos e ranking | frontend | 6 | P1 | ready | Bootstrap de app | frontend |
| Painel do gestor de atividade | admin-tooling | 6 | P1 | ready | — | manager-panel |
| Ciclo de vida do run (criar/QR/start/…) | admin-tooling | 6 | P1 | ready | Painel do gestor | manager-panel |
| Composer de foto (checksum→intenção→S3→complete→Moment) | frontend | 7 | P0 | ready | Abertura da tela Game | frontend |
| Galeria (abas feed/mine/group) | frontend | 7 | P0 | ready | Bootstrap de app | frontend |
| Curtir Moment | frontend | 7 | P1 | ready | Galeria | frontend |
| Fila de moderação corretiva (admin) | admin-tooling | 7 | P2 | ready | — | moderation-panel |
| Preferências de notificação | frontend | 8 | P1 | ready | Bootstrap de app | frontend |
| Lista de notificações e badge | frontend | 8 | P0 | ready | Bootstrap de app | frontend |
| Envio administrativo de notificação | admin-tooling | 8 | P2 | ready | — | admin-panel |
| Configuração administrativa (spaces/activities/staff/managers) | admin-tooling | 4 | P2 | ready | — | admin-panel |
| Enablers de plataforma (healthcheck/readiness) | enabler | 1 | P0 | done | — | platform |

Para cada linha, o manifesto JSON traz `blockers` (vazio para todas as 25 —
nenhuma pendência de backend nesta entrega) e `acceptanceTest` (critério
objetivo de pronto). Nenhum fluxo amplo ficou sem decompor: cada linha acima
corresponde a uma tela ou ação concreta, nunca a um domínio inteiro como
"integrar agenda".

## Detalhe dos fluxos P0 (bloqueiam o app do jogador se faltarem)

Os fluxos abaixo têm grafo de request completo (gatilho, ordem, fan-out,
payload, cache, cancelamento, retry, polling) já documentado — este handoff
não duplica esse conteúdo, apenas aponta e resume o essencial para
implementar.

| Fluxo | Rota(s) V1 → V2 | Sequência resumida | Grafo completo |
|---|---|---|---|
| Tela de login | `POST /api/auth/google` → `POST /v2/auth/google` | 1 chamada; `onboardingComplete=false` redireciona para onboarding | [`auth-and-tokens.md`](../auth-and-tokens.md) |
| Cadastro/login por email | novo em V2, substitui o `/v1/auth/onboarding` passwordless (morto — depende de um webhook de parceiro que nunca chegou a existir) | `POST /auth/signup` (sempre `CODE_SENT`) → usuário digita o código do email → `POST /auth/signup/verify` (mesma resposta de sessão do login Google); 429 no reenvio antes do cooldown | [`auth-and-tokens.md#cadastrologin-por-email-v2`](../auth-and-tokens.md#cadastrologin-por-email-v2) |
| Bootstrap de app | novo em V2 | `GET /auth/session`; em 401, `POST /auth/refresh` uma vez, então repete | [`auth-and-tokens.md`](../auth-and-tokens.md) |
| Completar cadastro | novo em V2 | `PATCH /auth/onboarding` com CPF/telefone/grupo | [`auth-and-tokens.md`](../auth-and-tokens.md) |
| Agenda e detalhe | `GET /api/v1/schedule` → `GET /v2/schedule` | schedule → detalhe sob demanda ao abrir card | [`agenda-content.md`](../agenda-content.md) |
| Abertura da tela Game | `/api/v1/game/overview` + 2 rotas → 3 GETs `/v2` em paralelo | `getGameOverview` + `getCurrentActivityRun` + `getCurrentParticipation` disparados juntos, nunca em cascata; 204 vira estado vazio | [`game-frontend-handoff.md#grafo-de-requests--abertura-do-game`](../game-frontend-handoff.md) |
| Scanner de QR | `/api/v1/qr/validate` (chave no corpo) → `POST /v2/qr/validate` (chave no header) | 1 chamada; corpo só `{qrToken}` | [`game-frontend-handoff.md#grafo-de-requests--scan`](../game-frontend-handoff.md) |
| Composer de foto | `POST /api/v1/moments` multipart → 4 chamadas V2 | checksum local → `createMediaUploadIntent` → `PUT` direto no S3 → `completeMediaUpload` → `createMoment` | [`game-frontend-handoff.md#grafo-de-requests--cálculo-de-checksum-e-upload-em-quatro-etapas`](../game-frontend-handoff.md) e [`media-moments.md`](../media-moments.md) |
| Galeria | `GET /api/v1/moments?scope=` → `GET /v2/moments?scope=&cursor=` | `scope` obrigatório e único; cursor opaco, nunca reconstruído | [`game-frontend-handoff.md#grafo-de-requests--abrir-a-galeria-três-abas`](../game-frontend-handoff.md) |
| Lista de notificações e badge | novo em V2 | `GET /notifications` traz `unreadCount`; `POST /notifications/{id}/read` idempotente | [`game-frontend-handoff.md#grafo-de-requests--notificações-iteração-8`](../game-frontend-handoff.md) e [`notifications.md`](../notifications.md) |

## Matriz tela/fluxo → operação → status → teste automatizado

Gerada a partir de `docs/openapi/dnj-v2.operations.yaml` e do manifesto de
handoff — cada linha tem um teste automatizado real por trás (arquivo em
`internal/`, não um placeholder).

| Tela/Fluxo | Operação HTTP | operationId | Status publicados | Teste automatizado |
|---|---|---|---|---|
| Abertura da tela Game (3 GETs em paralelo) | GET /activity-runs/current | `getCurrentActivityRun` | 200,204,400,401,403,409,500 | `iteration6_service_test.go` |
| Abertura da tela Game (3 GETs em paralelo) | GET /game/overview | `getGameOverview` | 200,400,401,403,409,500 | `iteration6_service_test.go` |
| Abertura da tela Game (3 GETs em paralelo) | GET /participations/current | `getCurrentParticipation` | 200,204,400,401,403,409,500 | `iteration6_service_test.go` |
| Agenda e detalhe de atividade | GET /activities | `listPublicActivities` | 200,400,404,500 | `iteration5_service_test.go` |
| Agenda e detalhe de atividade | GET /activities/{activityId} | `getPublicActivity` | 200,400,404,500 | `iteration5_service_test.go` |
| Agenda e detalhe de atividade | GET /schedule | `getSchedule` | 200,400,500 | `iteration5_service_test.go` |
| Agenda e detalhe de atividade | GET /spaces | `listSpaces` | 200,500 | `iteration4_service_test.go` |
| Ação de sair | POST /auth/logout | `logoutIdentitySession` | 200,403,500 | `identity_handler_test.go` |
| Bootstrap de app (checagem de sessão + refresh silencioso) | GET /auth/session | `getCurrentIdentitySession` | 200,401,500 | `identity_handler_test.go` |
| Bootstrap de app (checagem de sessão + refresh silencioso) | POST /auth/refresh | `refreshIdentitySession` | 200,401,403,500 | `identity_handler_test.go` |
| Catálogo de jogos e ranking | GET /games | `listGames` | 200,400,500 | `iteration6_service_test.go` |
| Catálogo de jogos e ranking | GET /games/{gameId} | `getGame` | 200,400,404,500 | `iteration6_service_test.go` |
| Catálogo de jogos e ranking | GET /rankings | `listRankings` | 200,400,500 | `iteration6_service_test.go` |
| Ciclo de vida do run (criar/QR/start/pause/resume/finalizar/cancelar) | POST /manager/runs | `createManagerRun` | 201,400,401,403,404,409,500 | `iteration6_service_test.go` |
| Ciclo de vida do run (criar/QR/start/pause/resume/finalizar/cancelar) | POST /manager/runs/{runId}/cancel | `cancelManagerRun` | 200,400,401,403,404,409,500 | `iteration6_service_test.go` |
| Ciclo de vida do run (criar/QR/start/pause/resume/finalizar/cancelar) | POST /manager/runs/{runId}/pause | `pauseManagerRun` | 200,400,401,403,404,409,500 | `iteration6_service_test.go` |
| Ciclo de vida do run (criar/QR/start/pause/resume/finalizar/cancelar) | POST /manager/runs/{runId}/qr | `rotateManagerRunQR` | 201,400,401,403,404,409,500 | `iteration6_service_test.go` |
| Ciclo de vida do run (criar/QR/start/pause/resume/finalizar/cancelar) | POST /manager/runs/{runId}/results | `finalizeManagerRunResults` | 200,400,401,403,404,409,500 | `iteration6_service_test.go` |
| Ciclo de vida do run (criar/QR/start/pause/resume/finalizar/cancelar) | POST /manager/runs/{runId}/resume | `resumeManagerRun` | 200,400,401,403,404,409,500 | `iteration6_service_test.go` |
| Ciclo de vida do run (criar/QR/start/pause/resume/finalizar/cancelar) | POST /manager/runs/{runId}/start | `startManagerRun` | 200,400,401,403,404,409,500 | `iteration6_service_test.go` |
| Completar cadastro (CPF/telefone/grupo) | PATCH /auth/onboarding | `completeIdentityOnboarding` | 200,400,401,404,409,500 | `identity_handler_test.go` |
| Composer de foto (checksum → intenção → PUT S3 → complete → criar Moment) | POST /media/upload-intents | `createMediaUploadIntent` | 201,400,401,403,409,413,415,500,503 | `media_moment_service_test.go` |
| Composer de foto (checksum → intenção → PUT S3 → complete → criar Moment) | POST /media/{mediaAssetId}/complete | `completeMediaUpload` | 200,400,401,403,404,409,410,413,422,500,503 | `media_moment_service_test.go` |
| Composer de foto (checksum → intenção → PUT S3 → complete → criar Moment) | POST /moments | `createMoment` | 201,400,401,403,404,409,500 | `media_moment_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | DELETE /admin/activities/{activityId}/managers/{userId} | `removeAdminActivityManager` | 204,400,401,403,404,409,500 | `admin_installation_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | GET /admin/activities | `listAdminActivities` | 200,400,401,403,500 | `admin_installation_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | GET /admin/activities/{activityId}/managers | `listAdminActivityManagers` | 200,400,401,403,404,500 | `admin_installation_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | GET /admin/spaces | `listAdminSpaces` | 200,400,401,403,500 | `admin_installation_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | GET /admin/staff | `listAdminStaff` | 200,400,401,403,500 | `admin_installation_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | PATCH /admin/activities/{activityId} | `updateAdminActivity` | 200,400,401,403,404,409,500 | `admin_installation_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | PATCH /admin/spaces/{spaceId} | `updateAdminSpace` | 200,400,401,403,404,409,500 | `admin_installation_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | PATCH /admin/users/{userId}/role | `updateAdminUserRole` | 200,400,401,403,404,409,500 | `admin_installation_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | POST /admin/activities | `createAdminActivity` | 201,400,401,403,404,409,500 | `admin_installation_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | POST /admin/spaces | `createAdminSpace` | 201,400,401,403,409,500 | `admin_installation_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | POST /manager/activities/{activityId}/pause | `pauseActivity` | 200,400,401,403,404,409,500 | `iteration4_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | POST /manager/activities/{activityId}/start | `startActivity` | 200,400,401,403,404,409,500 | `iteration4_service_test.go` |
| Configuração administrativa (spaces/activities/staff/managers) | PUT /admin/activities/{activityId}/managers/{userId} | `assignAdminActivityManager` | 200,400,401,403,404,409,500 | `admin_installation_service_test.go` |
| Curtir Moment | POST /moments/{momentId}/likes | `toggleMomentLike` | 200,400,401,403,404,409,500 | `media_moment_service_test.go` |
| Entrar em grupo por convite/código | POST /groups/invites/consume | `consumeGroupInvite` | 200,400,401,404,500 | `group_invite_service_test.go` |
| Envio administrativo de notificação | POST /admin/notifications | `sendAdminNotification` | 201,400,401,403,409,500 | `notification_service_test.go` |
| Favoritar atividade | DELETE /users/me/favorites/{activityId} | `deleteCurrentUserFavorite` | 204,400,401,409,500 | `iteration5_service_test.go` |
| Favoritar atividade | GET /users/me/favorites | `listCurrentUserFavorites` | 200,400,401,409,500 | `iteration5_service_test.go` |
| Favoritar atividade | PUT /users/me/favorites/{activityId} | `putCurrentUserFavorite` | 204,400,401,404,409,500 | `iteration5_service_test.go` |
| Fila de moderação corretiva (admin) | GET /admin/moments/moderation | `listMomentModeration` | 200,400,401,403,500 | `media_moment_service_test.go` |
| Fila de moderação corretiva (admin) | POST /admin/moments/{momentId}/moderation | `moderateMoment` | 200,400,401,403,404,409,500 | `media_moment_service_test.go` |
| Galeria (abas feed/mine/group) | GET /moments | `listMoments` | 200,400,401,403,409,500 | `media_moment_service_test.go` |
| Lista de notificações e badge de não lidas | GET /notifications | `listNotifications` | 200,401,403,409,500 | `notification_service_test.go` |
| Lista de notificações e badge de não lidas | POST /notifications/{notificationId}/read | `markNotificationRead` | 200,400,401,403,404,409,500 | `notification_service_test.go` |
| Meu grupo e membros | GET /groups | `listGroups` | 200,400,401,500 | `group_service_test.go` |
| Meu grupo e membros | GET /groups/me | `getCurrentGroup` | 200,401,500 | `group_service_test.go` |
| Meu grupo e membros | GET /groups/me/members | `listCurrentGroupMembers` | 200,401,404,500 | `group_service_test.go` |
| Meu grupo e membros | PATCH /users/me/group | `updateCurrentUserGroup` | 200,400,401,404,500 | `group_service_test.go` |
| Meu grupo e membros | POST /users/me/group | `updateCurrentUserGroupCompatibility` | 200,400,401,404,500 | `group_service_test.go` |
| Meu perfil | GET /users/me | `getCurrentProfile` | 200,401,500 | `profile_service_test.go` |
| Meu perfil | PATCH /users/me | `updateCurrentProfile` | 200,400,401,500 | `profile_service_test.go` |
| Painel admin — convites de grupo | DELETE /admin/groups/{groupId}/invites/{inviteId} | `revokeGroupInvite` | 204,401,403,404,409,500 | `group_invite_service_test.go` |
| Painel admin — convites de grupo | GET /admin/groups/{groupId}/invites | `listGroupInvites` | 200,401,403,404,500 | `group_invite_service_test.go` |
| Painel admin — convites de grupo | POST /admin/groups/{groupId}/invites | `createGroupInvite` | 201,401,403,404,500 | `group_invite_service_test.go` |
| Painel admin — convites de grupo | POST /admin/groups/{groupId}/invites/{inviteId}/renew | `renewGroupInvite` | 201,401,403,404,409,500 | `group_invite_service_test.go` |
| Painel do gestor de atividade | GET /manager/game-overview | `getManagerGameOverview` | 200,400,401,403,409,500 | `iteration6_service_test.go` |
| Painel do gestor de atividade | GET /manager/runs/{runId} | `getManagerRun` | 200,400,401,403,404,409,500 | `iteration6_service_test.go` |
| Preferências de notificação | GET /notifications/preferences | `getNotificationPreferences` | 200,401,403,409,500 | `notification_service_test.go` |
| Preferências de notificação | PUT /notifications/preferences | `updateNotificationPreferences` | 200,400,401,403,409,500 | `notification_service_test.go` |
| Scanner de QR | POST /qr/validate | `validateGameQR` | 200,201,400,401,403,409,410,500 | `iteration6_service_test.go` |
| Tela de login | POST /auth/google | `authenticateWithGoogle` | 200,400,401,409,500 | `identity_handler_test.go` |
| Cadastro/login por email | POST /auth/signup | `signupWithEmail` | 200,400,429,500 | `email_signup_service_test.go` |
| Cadastro/login por email | POST /auth/signup/verify | `verifyEmailSignup` | 200,400,401,500 | `email_signup_service_test.go` |
| n/a — usado por load balancer/monitoramento, não por UI | GET /healthcheck | `getHealthcheck` | 200 | `healthcheck_router_test.go` |
| n/a — usado por load balancer/monitoramento, não por UI | GET /readiness | `getReadiness` | 200,503 | `healthcheck_router_test.go` |

## Estados vazios e de retry

- **204 No Content** (ex: `getCurrentActivityRun`, `getCurrentParticipation`
  sem participação ativa) é um estado vazio válido, não um erro — renderize
  um placeholder, nunca uma mensagem de falha.
- **409** em rotas de escrita geralmente significa estado incompatível (ex:
  run já finalizado, moderação já aplicada) — trate como "ação não
  disponível mais", nunca como falha transitória para retry automático.
- **Retry** só é seguro em erros de rede/timeout ou `5xx`, sempre reenviando
  a mesma `Idempotency-Key` usada na tentativa original — nunca gere uma
  chave nova para um retry da mesma intenção.
- **Polling** (ex: acompanhar um run ativo): use backoff exponencial com
  jitter e pare ao detectar um estado terminal (`completed`, `cancelled`);
  nunca faça polling de uma tela desmontada — cancele o intervalo no
  cleanup do efeito/componente.

## Remoção segura de aliases

Antes de remover uma rota `/api/v1/...` ou um proxy/Route Handler legado:

1. Confirme que a tela equivalente em V2 está em produção há tempo
   suficiente para capturar tráfego real (não apenas testado manualmente).
2. Confirme, via observabilidade estruturada (`http_request_completed`
   filtrado pela rota antiga), que o tráfego real na rota V1 caiu a zero ou
   a um nível residual explicável (ex: apenas clientes desatualizados em
   cache).
3. Remova o alias/proxy primeiro, mantenha a rota V1 real no backend até uma
   decisão explícita de desligamento — remover o alias do frontend não
   requer coordenação com o backend; desligar a rota V1 no backend requer.
4. Nunca remova um alias só porque a tela V2 "parece funcionar" em
   ambiente local — a confirmação é sempre por tráfego real observado.

## Checklist de aceite e rollback por fluxo

Para cada fluxo da tabela de backlog, antes de considerar `done`:

- [ ] Sequência de chamadas implementada exatamente como na "Sequência
      resumida"/grafo completo (nenhuma chamada extra, nenhuma cascata onde
      deveria ser paralelo).
- [ ] `acceptanceTest` do manifesto JSON validado manualmente pelo menos uma
      vez contra `develop`.
- [ ] Estados vazios (`204`), de erro (`4xx` relevantes) e de retry tratados
      — não só o caminho feliz.
- [ ] `Idempotency-Key` gerada por ação do usuário nas rotas de escrita,
      nunca reutilizada nem derivada do payload.
- [ ] Testes do frontend criados para o fluxo (o teste automatizado listado
      na matriz acima cobre o **backend**; o frontend precisa do seu
      próprio teste de integração/E2E para a tela).
- **Rollback**: reverter para a rota V1 equivalente (ainda ativa) é sempre
  seguro enquanto o alias V1 não tiver sido removido (ver seção anterior);
  nenhuma migration desta janela é destrutiva o suficiente para impedir essa
  reversão a qualquer momento.

## Como isto é mantido consistente

- `docs/handoff/dnj-v2-frontend-integration.json` é validado em CI por
  `cmd/handoff-check` (`make openapi`, parte de `make validate`): todo
  `operationId` publicado no OpenAPI deve pertencer a exatamente um fluxo, e
  todo fluxo deve referenciar apenas `operationId`s publicados. Divergência
  falha o build.
- A página HTML publicada (`/develop/frontend-integration/` e
  `/production/frontend-integration/`) é gerada a partir do mesmo JSON em
  tempo de build (`scripts/publish-openapi-docs.sh`) — nunca editada à mão,
  nunca pode ficar dessincronizada do manifesto.
- O artefato do workflow (`dnj-v2-frontend-handoff-{develop,production}`,
  retenção de 90 dias) contém a página renderizada, este Markdown, o
  manifesto JSON e o OpenAPI 3.0.3 completo — baixável direto do run do
  GitHub Actions, sem precisar clonar o repositório.
- Os exemplos de chamada/resposta executáveis vivem no próprio OpenAPI
  (blocos `example` por operação em `docs/openapi/dnj-v2.openapi.yaml`,
  renderizados na Swagger UI publicada) — este documento não duplica um
  segundo conjunto de exemplos que poderia divergir do primeiro.
