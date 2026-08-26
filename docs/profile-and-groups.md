# Perfil, grupos, memberships e convites V2

Este guia acompanha o contrato `2.2.0` em `docs/openapi/dnj-v2.openapi.yaml`.
As rotas são relativas a `/v2`, exigem JWT de identidade por Bearer ou cookie
`identity_token` e retornam o envelope de erro `{code,message,details?,requestId}`.

## Decisões de contrato

- `GET /users/me` nunca retorna CPF integral, `document_hash`, hashes de
  identidade, tokens ou segredos. `documentMasked` contém apenas os quatro
  últimos dígitos mascarados.
- `PATCH /users/me` aceita estritamente `name` e `mobilePhone`. Email, CPF,
  identidade, papel, pontos e grupo são somente leitura.
- `PATCH /users/me/group` é canônico. `POST /users/me/group` executa o mesmo
  caso de uso como alias deprecated para o frontend atual.
- `GET /groups` mantém o array esperado pelo consumidor e publica paginação
  nos headers `X-Page`, `X-Limit` e `X-Has-Next-Page`. A ordem é `name,id`.
- `POST /groups` cria um grupo — self-service, qualquer usuário autenticado,
  sem exigir papel `ADMIN`. Nome vazio retorna `400`; nome duplicado
  (case-insensitive, mesma regra de `FindByNameExact`) retorna
  `409 GROUP_NAME_TAKEN`. Cobre o cenário de quem busca em `GET /groups`, não
  encontra o próprio grupo, e cria na hora — inclusive antes de completar o
  onboarding, já que `groupId` é opcional em `PATCH /auth/onboarding`.
- `GET /groups/me/members` não recebe `groupId`: o grupo vem exclusivamente
  da identidade autenticada. Retorna apenas id, nome, papel e data de entrada,
  ordenados por `name,user_id`.
- A tabela `group_memberships` guarda somente a relação atual, com unicidade
  por usuário. `users.group_id` é mantido como espelho V1 na mesma transação.

## Permissões de convite

| Ação | Autorização |
|---|---|
| Criar, listar, renovar e revogar | Somente `ADMIN`; `EVENT_MANAGER` não herda permissão |
| Consumir | Qualquer identidade autenticada, inclusive durante onboarding |
| Ver membros | Somente os membros do próprio grupo |

Os códigos têm 128 bits aleatórios, são retornados apenas ao criar/renovar e o
banco armazena somente SHA-256. A validade é de sete dias. Renovar revoga o
anterior e cria outro atomicamente. Revogação é idempotente. O primeiro
consumidor vence atomicamente; retry do mesmo consumidor devolve a membership
já criada e qualquer outro consumidor recebe o mesmo
`404 INVITE_NOT_FOUND_OR_UNAVAILABLE` usado para código inexistente, expirado
ou revogado.

## Exemplos de handoff

```http
GET /v2/users/me
Authorization: Bearer <identity-token>
```

```json
{
  "id": "42",
  "email": "ana@example.com",
  "name": "Ana Souza",
  "mobilePhone": "5541999990000",
  "documentMasked": "***.***.*47-25",
  "role": "DEFAULT",
  "group": {"id": "12", "groupName": "Jovens da Luz"},
  "points": 0,
  "rankPosition": 1,
  "onboardingComplete": true,
  "createdAt": "2026-08-22T15:00:00Z",
  "updatedAt": "2026-08-22T15:00:00Z"
}
```

```http
PATCH /v2/users/me
Authorization: Bearer <identity-token>
Content-Type: application/json

{"name":"Ana Atualizada","mobilePhone":"5541988887777"}
```

```http
PATCH /v2/users/me/group
Authorization: Bearer <identity-token>
Content-Type: application/json

{"groupId":"12"}
```

Para sair do grupo, envie `{"groupId":null}`.

```http
POST /v2/admin/groups/12/invites
Authorization: Bearer <admin-identity-token>
```

O campo `code` dessa resposta precisa ser entregue ao participante naquele
momento; ele não aparece na listagem administrativa.

```http
POST /v2/groups/invites/consume
Authorization: Bearer <identity-token>
Content-Type: application/json

{"code":"<codigo-opaco>"}
```

## Migrations e manutenção

As migrations `expand_iteration3_profiles_groups`,
`backfill_iteration3_group_memberships` e
`contract_iteration3_group_indexes` são aditivas e reexecutáveis. O backfill
copia cada `users.group_id` não nulo somente quando ainda não existe membership
e não altera a coluna legada. Não remover `users.group_id` enquanto houver V1.

Nunca registrar corpo de consumo, código em claro, hash, CPF ou resposta de
criação/renovação de convite. Para investigar abuso, use apenas request id,
invite id, group id, ator e transição de estado. Uma rotação futura de formato
de código deve aceitar o formato anterior até todos expirarem; nunca recalcular
hash a partir de dados persistidos.

## Enabler do frontend para as etapas finais

O frontend não é alterado nesta iteração. Antes do handoff final ele deve:

1. reconstruir o perfil com `GET /v2/users/me` usando cookie HttpOnly;
2. migrar edição de nome/telefone para `PATCH /v2/users/me`;
3. migrar seleção de grupo do alias `POST` para o `PATCH` canônico;
4. consumir paginação pelos headers em `/groups` e pelo envelope nas demais listas;
5. implementar telas administrativas de convite somente para `ADMIN`;
6. manter códigos apenas em memória durante criação, renovação e consumo;
7. tratar todos os convites indisponíveis pelo mesmo estado visual, sem revelar
   se o código existiu, expirou, foi revogado ou pertenceu a outra pessoa.
