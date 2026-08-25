# Notificações — Iteração 8

Este guia descreve o contrato implementado em `/v2`. `Notification` é a única
entidade persistida; não há push nativo, e-mail transacional novo, websocket
ou polling em tempo real nesta iteração — é somente REST síncrono.

## Modelo de eventos

Uma `Notification` é sempre derivada de um evento do servidor. Não existe via
de escrita que aceite tipo, destinatário, conteúdo arbitrário ou timestamp do
cliente para uma notificação derivada do sistema — `userId`, `read`, `sentAt`,
`createdAt` e estado são sempre calculados no servidor. Categorias mínimas:
`moment_moderation`, `points`, `announcement`. Estados mínimos: `unread`,
`read`.

- `moment_moderation`: emitida quando `POST /admin/moments/{momentId}/moderation`
  (Iteração 7) efetivamente transiciona o Moment para `rejected`
  (`deny_points` ou `delete_photo`). Nunca pode ser desabilitada pela conta —
  é tratada como notificação de moderação/segurança da própria conta.
- `points`: emitida quando pontos são concedidos (prêmio de challenge via
  Moment, resultado de `activity_run`) ou revertidos (reversão de prêmio de
  Moment por moderação, resultado de `activity_run`). Respeita a preferência
  `pointsEnabled` do destinatário.
- `announcement`: a única categoria de conteúdo livre, emitida exclusivamente
  por `POST /admin/notifications`, sempre atribuída ao ator ADMIN autenticado
  — nunca a um usuário arbitrário. Respeita a preferência
  `announcementEnabled` do destinatário.

A escrita da notificação é atômica com o evento que a originou: ela acontece
na mesma transação de banco que grava a decisão de moderação ou o lançamento
de pontos (`internal/infrastructure/db/repositories/notification_event_writer.go`,
chamado a partir de `moment_repository.go` e `game_repository.go`) — se o
evento não se confirma, a notificação também não se confirma.

## Preferências

`GET /notifications/preferences` retorna `{momentModerationEnabled,
pointsEnabled, announcementEnabled, updatedAt}`. Um usuário que nunca
personalizou preferências recebe o padrão: todas habilitadas.
`momentModerationEnabled` é sempre `true` na resposta — não existe caminho de
escrita para desabilitá-la.

`PUT /notifications/preferences` recebe estritamente `{pointsEnabled?,
announcementEnabled?}`; qualquer outro campo (incluindo
`momentModerationEnabled`) é rejeitado por decodificação estrita com
`400 INVALID_REQUEST`. Campos omitidos preservam o valor atual (ou o padrão,
se ainda não existir preferência persistida). Exige `Idempotency-Key` UUID;
mesma chave e mesma intenção repetem exatamente o resultado original.

## Listagem e leitura

`GET /notifications` lista as notificações do ator atual, mais recentes
primeiro (`createdAt DESC, id DESC`), paginação de 10 itens por página
(`?page=N`, 1-indexado na borda HTTP). A resposta inclui `unreadCount`, a
contagem agregada de não lidas do ator — usada pelo badge do app. Nunca expõe
notificações de outro usuário.

`POST /notifications/{notificationId}/read` não aceita corpo e marca a
notificação como lida. É idempotente: reenviar a mesma chave, ou marcar uma
notificação já lida, retorna o mesmo estado terminal — nunca reverte `read`
para `unread`. Notificação alheia ou inexistente retorna o mesmo
`404 NOT_FOUND`, sem permitir enumeração. Exige `Idempotency-Key` UUID.

## Envio administrativo

`POST /admin/notifications` recebe estritamente `{title, body,
targetUserIds?}`. É a única via de conteúdo livre desta iteração e exige ADMIN
confirmado no banco — `EVENT_MANAGER` não recebe essa permissão. `title` e
`body` em branco retornam `400 INVALID_REQUEST`.

Sem `targetUserIds`, o envio é um broadcast para todo usuário `DEFAULT`,
existente, com onboarding completo e que não desabilitou
`announcementEnabled`. Com `targetUserIds`, o broadcast é restrito a esse
conjunto — ainda assim filtrado pelos mesmos critérios de elegibilidade e
preferência: informar um ID de usuário alheio à elegibilidade simplesmente o
exclui silenciosamente, sem erro. A resposta é `{recipientCount}` — apenas a
contagem agregada; a API nunca expõe a lista de destinatários de um envio em
massa.

Exige `Idempotency-Key` UUID, seguindo o mesmo padrão unificado
`idempotency_operations` das Iterações 1–7. Um retry com a mesma chave replica
o `recipientCount` original sem duplicar entrega — o broadcast inteiro
(inserção de todas as notificações do lote e o registro da operação
idempotente) acontece em uma única transação.

## Autenticação e autorização

Listagem, leitura e preferências exigem usuário autenticado, existente, com
onboarding completo e papel `DEFAULT`, revalidado no banco a cada request
(inclusive uma segunda vez dentro da transação de escrita, para fechar a
janela entre a leitura inicial e o commit). Envio administrativo exige `ADMIN`
confirmado no banco, também revalidado dentro da transação.

## Idempotência e concorrência

Toda escrita HTTP desta iteração (`PUT /notifications/preferences`,
`POST /notifications/{id}/read`, `POST /admin/notifications`) exige
`Idempotency-Key` UUID no header, seguindo exatamente o mesmo padrão unificado
(`idempotency_operations`) das Iterações 1–7: a intenção idempotente inclui
ator, operação e um fingerprint canônico do payload; mesma chave e mesma
intenção repetem exatamente o resultado original; reuso da mesma chave para
outra intenção retorna `409 IDEMPOTENCY_KEY_REUSED`. Sob concorrência com a
mesma chave, um único efeito é aplicado — o perdedor da corrida de escrita da
`idempotency_operations` sempre observa e retorna o resultado do vencedor
(nunca um erro genérico de conflito ao cliente).

## Matriz HTTP publicada

| Família | Status publicados |
|---|---|
| ler preferências | 200, 401, 403, 409, 500 |
| atualizar preferências | 200, 400, 401, 403, 409, 500 |
| listar notificações | 200, 401, 403, 409, 500 |
| marcar como lida | 200, 400, 401, 403, 404, 409, 500 |
| envio administrativo | 201, 400, 401, 403, 409, 500 |

## Operação e diagnóstico

- Nunca logar `title`/`body` de notificações administrativas, e-mail ou
  demais PII do destinatário.
- Para investigar um retry duvidoso, consulte `idempotency_operations` pela
  identidade e chave, assim como nas Iterações 1–7.
- Notificações nunca são excluídas fisicamente via API; não há cascata
  destrutiva associada a este recurso.
- Não há seed de dados de notificação na imagem de produção; toda notificação
  nasce de um evento real do servidor ou de um envio administrativo
  explícito.
