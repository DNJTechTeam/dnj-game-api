# Instalação única, espaços, atividades e operação V2

Este guia acompanha o contrato `2.3.1` em
`docs/openapi/dnj-v2.openapi.yaml`. O DNJ é uma instalação de edição única:
não existe tabela `events`, coluna `event_id`, seleção de evento ou escopo
multi-evento.

## Modelo persistido

- `spaces` representa somente locais físicos. IDs são UUID, `slug` é único,
  `map_reference` é opcional e a descoberta pública ordena por `name,id`.
- `activities` representa ações configuráveis. `space_id` é opcional e fica
  nulo quando a ação não depende de local. `kind` aceita `schedule`,
  `checkpoint`, `challenge`, `competitive` e `live`; `status` aceita `draft`,
  `active`, `paused`, `completed` e `archived`.
- `starts_at` e `ends_at` são `timestamptz`; quando ambos existem, início deve
  ser anterior ao fim. Pontos de check-in, pontos de Moment e cooldown são
  inteiros não negativos.
- `allows_moment=true` só é válido para `checkpoint`, `challenge`,
  `competitive` ou `live`. Isso é apenas elegibilidade estrutural: criação de
  Participation, QR, Moment e premiação continuam nas iterações próprias.
- `activity_manager_assignments(activity_id,user_id)` possui unicidade composta
  e é a única fonte de escopo de um gestor.
- `operation_audit` registra ator, ação, entidade, metadados mínimos de estado e
  chave de idempotência. Não armazena email, documento, token, segredo ou corpo
  da requisição.
- `admin_operations` preserva separadamente o status HTTP e o resultado seguro
  original das escritas administrativas. A unicidade por ator/chave e o
  fingerprint da intenção impedem efeitos duplicados e reutilização cruzada
  sem transformar o audit em armazenamento de corpos.

As migrations `expand_iteration4_installation_activities`,
`backfill_iteration4_installation_activities` e
`contract_iteration4_installation_activities` preservam as tabelas e colunas
das Iterações 1–3. O backfill não apaga registros, e a contract adiciona checks,
FKs e índices determinísticos. As migrations adicionais
`expand_iteration4_admin_enabler`, `backfill_iteration4_admin_enabler` e
`contract_iteration4_admin_enabler` adicionam referência textual auditável,
resultado idempotente e índices de listagem. Nenhuma migration cria ou consulta
`events`.

## Papéis e permissões

| Handoff | Papel V2 existente | Permissões nesta iteração |
|---|---|---|
| `admin` | `ADMIN` | Escopo global para iniciar/pausar qualquer Activity e acesso exclusivo à configuração administrativa, papéis permitidos e assignments. |
| `manager` | `EVENT_MANAGER` | Pode iniciar/pausar somente Activities explicitamente atribuídas no banco. Não altera configuração, papel ou assignments. |
| `participant` | `DEFAULT` | Descoberta pública de espaços; nenhuma operação privilegiada. |

O JWT continua sendo somente de identidade. Em cada operação privilegiada a
API lê o usuário, o papel e o assignment atuais no banco. Um
`EVENT_MANAGER` sem assignment recebe o mesmo `404 NOT_FOUND` de uma Activity
inexistente, impedindo enumeração. Um `DEFAULT` recebe `403 FORBIDDEN` sem que
o ID informado conceda qualquer escopo. `ADMIN` não depende de assignment.

## Contrato HTTP implementado

`GET /v2/spaces?page=1` é público e retorna o array já esperado pelo mapa do
frontend:

```json
[
  {
    "id": "11111111-1111-4111-8111-111111111111",
    "name": "Capela",
    "slug": "capela",
    "mapReference": "map:capela"
  }
]
```

A paginação é exposta em `X-Page`, `X-Limit` e `X-Has-Next-Page`; o limite é
20. A resposta continua sendo um array para compatibilidade direta com o
frontend existente.

Para iniciar ou pausar:

```http
POST /v2/manager/activities/22222222-2222-4222-8222-222222222222/start
Authorization: Bearer <identity-token>
Idempotency-Key: 33333333-3333-4333-8333-333333333333
```

```json
{"id":"22222222-2222-4222-8222-222222222222","status":"active"}
```

`start` aceita `draft → active` e `paused → active`; `pause` aceita somente
`active → paused`; `conclude` aceita `active → completed` e `paused →
completed`, mas somente para `kind` `challenge`, `competitive` ou `live` — uma
Activity `schedule` ou `checkpoint` (que já nasce `active`, ver seção
administrativa) retorna `409 ACTIVITY_STATE_CONFLICT` em `conclude`.
`completed` e `archived` não podem ser reabertos. Cada transição bloqueia a
Activity, atualiza seu estado e grava auditoria na mesma transação. Retry com
a mesma chave, ator, ação e Activity retorna o resultado original; reutilizar
a chave para outra intenção retorna `409 IDEMPOTENCY_KEY_REUSED`. Uma nova
chave sobre estado (ou kind) incompatível retorna `409
ACTIVITY_STATE_CONFLICT`.

## Contrato administrativo publicado

Todas as rotas abaixo ficam sob `/v2`, exigem JWT de identidade e consultam o
papel atual no banco. Somente `ADMIN` recebe acesso. As listagens usam envelope
`{data,pagination}`, limite 20 e ordem determinística por `name,id`.

| Operação | Corpo estrito / regra principal |
|---|---|
| `GET /admin/spaces?page=1` | Configuração paginada de Spaces. |
| `POST /admin/spaces` | `slug`, `name`, `mapReference?`; retorna 201. |
| `PATCH /admin/spaces/{spaceId}` | Um ou mais dentre `slug`, `name`, `mapReference`; `null` limpa somente `mapReference`. |
| `GET /admin/activities?page=1` | Todos os campos persistidos e o status atual. |
| `POST /admin/activities` | `spaceId?`, `slug`, `name`, `description?`, `kind`, `startsAt?`, `endsAt?`, `checkInPoints`, `momentPoints`, `cooldownSeconds`, `allowsMoment`; status inicial depende do `kind` — `schedule` e `checkpoint` nascem `active`, `challenge`, `competitive` e `live` nascem `draft`. |
| `PATCH /admin/activities/{activityId}` | Os mesmos campos configuráveis. `status` é uma extensão exclusiva do PATCH e aceita somente `archived`; `active` deve ser pausada antes. |
| `GET /admin/staff?role=DEFAULT&page=1` | Lista participantes onboarded que podem ser promovidos; também aceita `ADMIN` ou `EVENT_MANAGER` para staff operacional. |
| `PATCH /admin/users/{userId}/role` | Aceita somente `DEFAULT` ou `EVENT_MANAGER`; não concede nem remove `ADMIN`. |
| `GET /admin/activities/{activityId}/managers?page=1` | Lista assignments persistidos. |
| `PUT /admin/activities/{activityId}/managers/{userId}` | Exige usuário existente, onboarding completo e papel atual `EVENT_MANAGER`; é idempotente. |
| `DELETE /admin/activities/{activityId}/managers/{userId}` | Remove assignment; ausência já é sucesso 204 idempotente. |

### Datas, UTC e exibição local

`startsAt` e `endsAt` representam instantes absolutos. O transporte HTTP e a
persistência usam UTC; respostas são normalizadas para RFC 3339 com sufixo `Z`
(por exemplo, `2026-08-24T18:00:00Z`). Se a API receber um RFC 3339 com offset,
ela preserva o instante e o normaliza para UTC antes de persistir e responder.

No frontend, UTC não deve ser exibido como se fosse horário de parede. O
consumidor deve interpretar o valor como instante, formatá-lo no fuso local
atual do dispositivo para exibição e, ao editar, converter a escolha local de
volta para UTC antes do envio. Nunca acrescente `Z` a uma string local sem fazer
a conversão de fuso; isso altera o instante real.

Toda escrita exige `Idempotency-Key` UUID. O retry da mesma intenção pelo mesmo
ator devolve o status e o resultado originais mesmo após alterações posteriores;
a chave usada com outra operação, alvo ou payload retorna
`409 IDEMPOTENCY_KEY_REUSED`. Uma chave nova sempre gera audit administrativo,
inclusive em no-op. O audit contém ator, ação, tipo/referência da entidade e
metadados mínimos, nunca nome, email, documento, telefone, mapa, descrição,
corpo, token ou segredo.

Não existe rota `DELETE` para Space ou Activity. Activity pode seguir para
`archived` apenas pelo PATCH validado; `start` e `pause` continuam nas operações
gerenciais. Rebaixar gestor com assignment retorna
`409 MANAGER_HAS_ASSIGNMENTS`; primeiro remova todos os assignments. IDs
malformados, ausentes ou cruzados são resolvidos contra as tabelas
correspondentes, nunca confiados a partir do cliente.

## Handoff incremental do frontend das Iterações 2–5

Nenhum arquivo do frontend foi alterado. O handoff deve ser integrado nesta
ordem para não misturar contratos antigos de homologação:

1. Iteração 2: trocar sessão/senha administrativa por Google OIDC, access token,
   refresh/CSRF, `GET /auth/session`, onboarding e perfil V2. A autorização é
   revalidada no banco; o JWT permanece somente de identidade.
2. Iteração 3: consumir `/users/me`, membership/grupo atual, paginação e convites
   administrativos; usar `PATCH /users/me/group` e remover o alias POST após a
   migração do consumidor.
3. Iteração 4: trocar `/api/v1/spaces` por `GET /v2/spaces`, substituir Route
   Handlers acoplados a `events`, `experiences`, senhas e escopos antigos pelas
   rotas `/v2/admin`, e gerar um UUID de idempotência por intenção de escrita,
   preservando-o somente durante retries. Tratar `startsAt`/`endsAt` como
   instantes UTC no transporte e formatá-los para o fuso local atual somente na
   apresentação; seleções locais devem ser convertidas para UTC antes do envio.
4. Iteração 5: trocar o consumidor atual de `/schedule` pela base explícita
   `/v2`, criar módulos públicos para `/v2/activities` e
   `/v2/users/me/favorites`, cancelar requests ao desmontar, deduplicar
   re-render/reconexão e preservar o mesmo UUID em retries de favorito. O mapa
   completo de arquivos, requests, orçamentos, backlog e carga futura está em
   `docs/agenda-content.md`.

Agenda, conteúdo público e favoritos foram implementados na Iteração 5 e estão
documentados em `docs/agenda-content.md`, incluindo o handoff incremental das
Iterações 2–5 e os grafos de requests. QR, participações, runs, jogos, Moments,
mídia, ranking, anúncios e notificações permanecem nas iterações posteriores e
não devem ser simulados por esses endpoints.
