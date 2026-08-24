# Instalação única, espaços, atividades e operação V2

Este guia acompanha o contrato `2.3.0` em
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

As migrations `expand_iteration4_installation_activities`,
`backfill_iteration4_installation_activities` e
`contract_iteration4_installation_activities` preservam as tabelas e colunas
das Iterações 1–3. O backfill não apaga registros, e a contract adiciona checks,
FKs e índices determinísticos. Nenhuma migration cria ou consulta `events`.

## Papéis e permissões

| Handoff | Papel V2 existente | Permissões nesta iteração |
|---|---|---|
| `admin` | `ADMIN` | Escopo global para iniciar/pausar qualquer Activity. É o único papel autorizado a ler/alterar configuração não pública, gerir papéis e atribuir/remover gestores quando o contrato administrativo for aprovado. |
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
`active → paused`. `completed` e `archived` não podem ser reabertos. Cada
transição bloqueia a Activity, atualiza seu estado e grava auditoria na mesma
transação. Retry com a mesma chave, ator, ação e Activity retorna o resultado
original; reutilizar a chave para outra intenção retorna
`409 IDEMPOTENCY_KEY_REUSED`. Uma nova chave sobre estado incompatível retorna
`409 ACTIVITY_STATE_CONFLICT`.

## Enabler administrativo ainda não publicado

Os handoffs publicam descoberta de espaços e início/pausa, mas não definem
payloads, responses, paginação ou semântica dos endpoints administrativos de
configuração e staff. O frontend atual usa Route Handlers de homologação
acoplados a `events`, `experiences`, senhas administrativas e escopos antigos;
esses contratos não podem ser copiados para a V2 de instalação única.

A menor proposta para decisão é:

- `GET|POST /admin/spaces` e `PATCH /admin/spaces/{spaceId}`;
- `GET|POST /admin/activities` e `PATCH /admin/activities/{activityId}`;
- `GET /admin/staff?role=EVENT_MANAGER` e
  `PATCH /admin/users/{userId}/role` limitado a `DEFAULT ↔ EVENT_MANAGER`;
- `GET|PUT /admin/activities/{activityId}/managers` e
  `DELETE /admin/activities/{activityId}/managers/{userId}`.

Todas seriam exclusivas de `ADMIN`, paginadas e auditadas, com UUID de
idempotência nas escritas. Não haverá delete físico de Space/Activity nesta
fase: Activity usa `archived`; Space referenciado só poderá ser desvinculado ou
renomeado. Nenhuma dessas operações aparece no OpenAPI antes da aprovação e da
respectiva implementação/teste.

## Enablers do frontend para as etapas finais

Nenhum arquivo do frontend foi alterado. O handoff final deve integrar, em
ordem, os contratos de sessão/perfil/grupos das Iterações 2–3, trocar
`/api/v1/spaces` por `GET /v2/spaces`, substituir os Route Handlers
administrativos antigos depois que o enabler acima for decidido e consumir as
operações de Activity com JWT de identidade e `Idempotency-Key`. Agenda, QR,
participações, runs, jogos, Moments e anúncios permanecem nas iterações
posteriores.
