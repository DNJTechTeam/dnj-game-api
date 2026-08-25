# Jogos, runs, QR, pontuação e ranking — Iteração 6

Este guia descreve o contrato implementado em `/v2`. `Game` não é um domínio
persistido: é somente a projeção pública de uma `Activity` com
`kind=competitive`, usando exatamente a visibilidade pública da Iteração 5.

## Catálogo e ranking

- `GET /games?page=` usa `{data,pagination}`, limite 10 e ordem
  `startsAt NULLS LAST,name,id`.
- `GET /games/{gameId}` usa o mesmo `404 NOT_FOUND` para UUID inválido,
  inexistente, invisível, `draft` ou `archived`.
- O DTO de jogo contém somente `id`, `space`, `slug`, `name`, `description`,
  `startsAt`, `endsAt`, `allowsMoment` e `state`.
- `GET /rankings?scope=individual|groups&page=` é público e responde
  `{data,pagination,generatedAt}`. Usuários elegíveis são existentes, onboarding
  completo e papel atual `DEFAULT`.
- Individual ordena `points DESC,name,id`; grupos somam os saldos atuais dos
  membros elegíveis, incluem grupos vazios e usam a mesma regra ordinal.
- `GET /game/overview` exige participante elegível e agrega top 30 individual,
  top 10 grupos, 50 lançamentos próprios e a posição/saldo atuais.

Nenhum ranking lê cache em memória ou deriva pontos de payload. Email, CPF,
telefone e o `reason` interno do ledger nunca são publicados.

## Persistência e invariantes

As migrations `expand/backfill/contract` da versão `2.5.0` adicionam:

| Tabela | Responsabilidade |
|---|---|
| `activity_runs` | tentativa/partida de uma Activity competitive e snapshot 50/30/20/10 |
| `activity_run_participants` | participante e resultado na run; único por `(activity_run_id,user_id)` |
| `participations` | entrada do usuário, QR de origem e elegibilidade futura para Moment |
| `activity_run_qr_codes` | hash/HMAC do token, estado e expiração; nunca QR bruto |
| `point_entries` | ledger imutável com usuário, origem, motivo, delta e instante |
| `manager_operations` | idempotência e resultado seguro das operações gerenciais |

Não existem `games`, `events`, `event_id` ou cascatas destrutivas. FKs de
histórico usam `RESTRICT`. Um trigger `BEFORE UPDATE OR DELETE` torna
`point_entries` append-only no PostgreSQL e CockroachDB. Saldos anteriores à
Iteração 6 são materializados uma única vez como `legacy_balance`, sem criar ou
alterar pontos; premiações novas usam `activity_run_results`.

`users.points` é o saldo materializado. Finalização ordena e bloqueia run e
usuários, insere um lançamento por participante e incrementa o saldo na mesma
transação. `users_points_nonnegative_check` e update condicional impedem saldo
negativo. `ListPointBalanceMismatches` audita, de forma testável, usuários para
os quais `COALESCE(SUM(point_entries.delta),0) <> users.points`.

## Máquina de estados

| Origem | Destino permitido | Operação |
|---|---|---|
| criação | `draft` | `POST /manager/runs` |
| `draft` | `active` | `POST /manager/runs/{runId}/start` |
| `active` | `paused` | `POST /manager/runs/{runId}/pause` |
| `paused` | `active` | `POST /manager/runs/{runId}/resume` |
| `active` ou `paused` | `results` | etapa atômica da finalização |
| `results` | `completed` | somente após ledger e saldos completos |
| `draft`, `active`, `paused`, `results` | `cancelled` | `POST /manager/runs/{runId}/cancel` |

`completed` e `cancelled` são terminais. Transição incompatível retorna
`409 RUN_STATE_CONFLICT`. `startedAt` nasce na primeira entrada em `active` e
não muda no resume; `endedAt` nasce no término ou cancelamento. Todos os
instantes são produzidos por relógio injetável e serializados em UTC com `Z`.

## Autorização e não enumeração

Catálogo e ranking são públicos. Overview, run atual, participação atual e scan
exigem usuário existente, onboarding completo e papel atual `DEFAULT`, sempre
revalidado no banco. Operações gerenciais exigem `ADMIN` ou `EVENT_MANAGER`
com assignment persistido para a Activity. O papel do token não concede acesso.

Run inexistente e run fora do assignment retornam o mesmo 404. Run alheia no
fluxo do participante retorna 204. Campos desconhecidos, parâmetros repetidos,
corpos extras e mass assignment são rejeitados com 400.

## QR e idempotência

`POST /manager/runs/{runId}/qr` funciona somente em `draft`, invalida o QR
anterior e expira em 45 minutos ou ao sair de `draft`. O token é derivado por
HMAC a partir de um UUID aleatório; somente o hash de validação fica no banco.
O token bruto existe apenas na resposta de criação/retry e não deve aparecer em
logs, auditoria ou erros.

`POST /qr/validate` recebe estritamente `{qrToken}` e a chave no header
`Idempotency-Key`. O mesmo usuário entra uma vez na run, scan repetido não dá
pontos e não cria `operation_audit`. QR indisponível usa
`409 QR_UNAVAILABLE`; o instante exato de expiração usa `410 QR_EXPIRED`.

Todas as mutações gerenciais também exigem UUID de idempotência. Retry da mesma
chave e intenção devolve o resultado original, mesmo após mudança posterior de
estado. Reutilização para outro alvo, operação ou intenção retorna
`409 IDEMPOTENCY_KEY_REUSED`. Somente o primeiro efeito gerencial bem-sucedido
gera `operation_audit` mínimo.

## Resultados

`POST /manager/runs/{runId}/results` aceita somente:

```json
{
  "results": [
    {"participantId": "11111111-1111-4111-8111-111111111111", "result": "first"},
    {"participantId": "22222222-2222-4222-8222-222222222222", "result": "participation"}
  ]
}
```

O conjunto deve ser exatamente o conjunto persistido, sem duplicados,
omissões ou adicionais. `first`, `second` e `third` aceitam no máximo um
participante cada. Falha em qualquer lançamento, saldo, estado, operação
idempotente ou audit reverte toda a transação. Chave nova após `completed`
retorna o resultado terminal seguro e nunca premia novamente.

## Matriz HTTP publicada

| Família | Status publicados |
|---|---|
| catálogo/detalhe | 200, 400, 404 quando aplicável, 500 |
| ranking público | 200, 400, 500 |
| overview/run/participação | 200, 204 quando aplicável, 400, 401, 403, 409, 500 |
| scan | 200, 201, 400, 401, 403, 409, 410, 500 |
| gestão | 200/201, 400, 401, 403, 404 quando aplicável, 409, 500 |

`429` não é publicado nesta iteração; limites e perfis de carga permanecem
insumo explícito da Iteração 9.

## Operação e diagnóstico

- Nunca logar `Authorization`, cookies, QR bruto, payload integral de resultados,
  PII ou segredos.
- Para divergência de pontos, execute a auditoria de reconciliação em modo
  somente leitura antes de qualquer correção; não edite o ledger.
- Para retry duvidoso, consulte `manager_operations`/`participant_operations`
  pela identidade e chave, sem registrar o corpo original.
- Para concorrência, preserve a ordem de locks: run, participantes, usuários em
ordem de ID. Não introduza estado de coordenação somente em memória.
