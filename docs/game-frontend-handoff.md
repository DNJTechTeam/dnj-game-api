# Handoff incremental do frontend — jogos e ranking (Iteração 6)

O clone do frontend foi consultado somente como fonte de descoberta. Nenhum
arquivo dele deve ser alterado, commitado ou enviado por este trabalho.

## Consumidores atuais e rota de migração

| Tela/módulo atual | Rota atual | Rota V2 | Ajuste obrigatório |
|---|---|---|---|
| `src/features/game/game-screen.tsx` | `/api/v1/game/overview` | `GET /v2/game/overview` | remover `isUser`/`group`; usar `current` e `groupName` |
| mesma tela | `/api/v1/activity-runs/current` | `GET /v2/activity-runs/current` | tratar 204 e polling terminal |
| mesma tela e galeria | `/api/v1/participations/current` | `GET /v2/participations/current` | manter 204; não iniciar mídia na Iteração 6 |
| `src/features/scanner/qr-scanner-modal.tsx` | `/api/v1/qr/validate` com chave no corpo | `POST /v2/qr/validate` | enviar chave no header e corpo somente `{qrToken}` |
| `src/components/manager/manager-dashboard.tsx` | `/api/manager/overview` | `GET /v2/manager/game-overview` | preservar shape `scope:actions` |
| painel/Route Handlers `src/app/api/manager/actions/**` | ações legadas com `runId` no corpo | `/v2/manager/runs*` | mover runId para path e usar uma chave por intenção |
| ranking/display e leituras diretas | payloads agregados/Supabase | `GET /v2/rankings` | separar `individual` e `groups`; usar `position` do servidor |

Criação/edição de Activity competitive continua no contrato administrativo da
Iteração 4. A Iteração 6 não oferece CRUD de `Game` porque Game é projeção.

## Grafo de requests — abertura do Game

```text
abrir tela Game
  ├─ GET /v2/game/overview              [auth, obrigatório]
  ├─ GET /v2/participations/current     [auth, 200|204]
  └─ GET /v2/activity-runs/current      [auth, 200|204]
       └─ se run aberta: iniciar polling por runId
```

Fan-out inicial: três requests paralelos, uma vez por abertura. Dependências:
sessão válida e onboarding completo. Deduplicar por chave de cache durante a
montagem; cancelar as três com `AbortController` ao desmontar; timeout sugerido
8 s. Um 401 dispara no máximo um refresh compartilhado e uma repetição. 409
`ONBOARDING_REQUIRED` navega para onboarding; não aplicar retry automático.

Orçamento: 3 requests por abertura; mais 1 refresh apenas quando o access token
expirar. Não buscar `/rankings` adicional quando `/game/overview` já atende a
tela autenticada.

## Grafo de requests — scan

```text
leitura estável da câmera
  └─ POST /v2/qr/validate
       header Idempotency-Key: UUID estável desta leitura/intenção
       body {qrToken}
       ├─ 201|200 → renderizar participação; GET current run opcional
       ├─ 409 QR_UNAVAILABLE → mensagem neutra e permitir nova leitura
       └─ 410 QR_EXPIRED → mensagem de expiração e fechar tentativa
```

Orçamento: 1 POST por leitura lógica e, quando necessário para atualizar UI,
1 GET de run. `busyRef` já impede leituras simultâneas; a chave deve nascer
antes do POST e sobreviver a timeout/reconexão. Retry somente para timeout,
rede, 429 futuro ou 5xx, com a mesma chave, no máximo 3 tentativas em
0,5 s/1 s/2 s com jitter de ±20%. Nunca colocar a chave ou QR em URL/log.

```ts
async function validateQr(qrToken: string, signal: AbortSignal) {
  const key = crypto.randomUUID();
  return apiV2("/qr/validate", {
    method: "POST",
    signal,
    headers: { "Idempotency-Key": key },
    body: JSON.stringify({ qrToken }),
  });
}
```

## Grafos de requests — gestor

```text
abrir dashboard
  ├─ obter/renovar sessão V2
  └─ GET /v2/manager/game-overview
       └─ se actions.run != null: polling do mesmo overview

abrir partida(gameId)
  └─ POST /v2/manager/runs {gameId}
       └─ POST /v2/manager/runs/{runId}/qr [corpo vazio]

rotacionar QR
  └─ POST /v2/manager/runs/{runId}/qr [corpo vazio]

iniciar | pausar | retomar
  └─ POST /v2/manager/runs/{runId}/{start|pause|resume} [corpo vazio]

apurar/finalizar
  └─ POST /v2/manager/runs/{runId}/results {results:[...]}
       └─ resposta completed → parar polling e invalidar overview/rankings

cancelar
  └─ POST /v2/manager/runs/{runId}/cancel [corpo vazio]
       └─ resposta cancelled → parar polling
```

Cada POST usa um UUID novo por clique/intenção e reutiliza a mesma chave em
retry. Desabilitar o botão enquanto a intenção estiver em voo. 409 não deve ser
repetido automaticamente: atualizar overview uma vez e reconciliar a UI.
Orçamento por ação: um POST e no máximo um GET de reconciliação; criação com QR
consome dois POSTs porque são intenções distintas.

O backend retorna `qrToken`, não imagem/URL. O frontend deve gerar o bitmap
localmente (biblioteca QR já existente) sem enviar o token a terceiros e
descartá-lo ao sair de `draft` ou atingir `expiresAt`.

## Polling seguro

O polling atual de 2 s pode multiplicar chamadas em re-render, remontagem,
reconexão e várias abas. A referência `overviewPollInFlight` evita sobreposição,
mas ainda faltam jitter, backoff e deduplicação compartilhada.

Política proposta:

1. Uma única chave de polling por `runId` no cliente, compartilhada por
   consumidores montados.
2. `AbortController` no unmount, troca de run, logout e aba oculta.
3. Intervalo base 2 s em `active/results`, 3 s em `draft`, 5 s em `paused`.
4. Jitter ±20%; em erro de rede/5xx, backoff 2/4/8/15 s, máximo 30 s.
5. Reconexão dispara uma leitura imediata deduplicada, não um novo timer.
6. Parar definitivamente em `completed` ou `cancelled` após uma última
   atualização de overview/ranking.
7. `visibilitychange`: pausar oculto; ao voltar, uma leitura imediata e retomada.

Orçamento: em uma run ativa, máximo nominal 30 GET/min por dashboard ou tela de
participante; pausado, 12 GET/min. Com deduplicação, re-render/remount não muda
esse número.

## Tipos e datas

```ts
type RankingPerson = {
  id: string; name: string; groupName: string | null;
  points: number; position: number;
};

type CurrentRun = {
  id: string;
  status: "draft" | "active" | "paused" | "results" | "completed" | "cancelled";
  gameName: string;
  startedAt: string | null;
  endedAt: string | null;
  result?: "first" | "second" | "third" | "participation";
  points?: number;
};

const localDateTime = (instant: string) =>
  new Intl.DateTimeFormat(undefined, { dateStyle: "short", timeStyle: "short" })
    .format(new Date(instant));
```

Não anexar `Z` a string local. O backend já envia UTC; `Date`/`Intl` devem usar
o fuso atual do dispositivo.

## Backlog incremental 2–6

| Prioridade | Item | Dependência/endpoints | Estado | Aceite e evidência esperada |
|---|---|---|---|---|
| P0 | cliente V2, refresh compartilhado, CSRF e erros | Iteração 2 | ready | teste prova um refresh para fan-out e abort no logout |
| P0 | sessão/onboarding/perfil sem campos legados | `/v2/auth*`, `/v2/users/me` | ready | testes 401/409 e atualização segura |
| P0 | grupos/membership/convites | Iteração 3 | ready | paginação e 204/404 conforme contrato |
| P0 | administração e assignments | Iteração 4 | ready | UUID de idempotência e UTC testados |
| P0 | agenda/Activities/favoritos | Iteração 5 | ready | rotas legadas removidas dos consumidores migrados |
| P0 | adaptar tipos do Game overview | `/v2/game/overview` | ready | usa `groupName`, `position`, `current.points`; snapshot visual/teste |
| P0 | migrar scan para header idempotente | `/v2/qr/validate` | ready | 200/201/409/410 e retry com mesma chave testados |
| P0 | migrar run/participação atual | dois GETs current | ready | 204, terminal por runId, abort e parada testados |
| P0 | migrar dashboard actions | `/v2/manager/game-overview`, `/runs*` | ready | ciclo completo e 409 reconciliado em teste |
| P1 | gerar QR local a partir de `qrToken` | resposta de rotate QR | ready | nenhum token em log/analytics; expiração visível |
| P1 | migrar display público de ranking | `/v2/rankings` | ready | alternância de scopes usa `position` do servidor |
| P1 | polling compartilhado/backoff/jitter | current run e manager overview | ready | fake timers provam orçamento/parada terminal |
| P2 | remover Route Handlers e mocks legados após rollout | todos acima | blocked | depende de telemetria e rollback aprovado |
| P2 | Moment/upload/galeria | Iteração 7 | pending | fora da Iteração 6 |

Rollback: manter o adaptador antigo atrás de flag durante o rollout, mas nunca
executar V1 e V2 mutantes em paralelo. Para recuar, parar novos POSTs, aguardar
intenções em voo, alternar a flag e preservar as chaves já emitidas até o fim
dos retries.

## Perfis futuros de carga — entrada da Iteração 9

Não executar stress amplo nesta iteração. Todos os perfis abaixo são
não destrutivos por padrão; cenários mutantes usam tenants/dados efêmeros
somente em ambiente autorizado.

| Ambiente/perfil | Concorrência | RPS/burst/duração | Timeout/limite/pool | Orçamento de erro |
|---|---:|---|---|---:|
| local smoke | 1–5 | 1–5 RPS, burst 10, 60 s | 8 s; sem rate limit; pool 10 | <1% |
| CI smoke reproduzível | 10 | 10 RPS, burst 20, 2 min | 5 s; pool 20 | 0,5% e nenhum 5xx |
| develop soak autorizado | 100 | 50 RPS, burst 200, 30 min | 8 s; limite candidato 120/min/identidade; pool 40 | <1% |
| develop spike autorizado | 250 | 200 RPS, burst 500, 5 min | 8 s; pool 60 | <2%, sem perda de efeito |
| produção canary read-only | 25 | 10 RPS, burst 25, 10 min | 5 s; respeitar limite publicado; pool existente | <0,5% |

Perfis por fluxo:

| Fluxo | Concorrência/RPS/burst | Duração | Medidas obrigatórias |
|---|---|---|---|
| cold start + health/readiness | 20 / 5 / 20 | 5 min | p50/p95/p99, init, erros |
| login/refresh fan-out | 50 / 20 / 100 | 10 min | refresh único, 401/403, conexões |
| abertura Game (3 GETs) | 100 / 100 / 300 | 15 min | p95, cache, pool, saturação |
| catálogo/rankings/overview | 100 / 75 / 200 | 30 min | query time, payload, CPU, p99 |
| polling de run | 250 clientes / 125 RPS / 250 | 30 min | dedupe, throttling, conexões |
| burst de scans | 200 / 100 / 400 | 5 min | unicidade, locks, 409/410, latência |
| finalização concorrente | 20 intenções / 10 RPS / 40 | 5 min | um ledger, rollback, deadlocks |
| atualização de ranking pós-finalização | 100 / 50 / 150 | 10 min | consistência saldo/ledger, p99 |
| retries e reconexão | 100 / 50 / 300 | 10 min | mesmos efeitos, backoff, 5xx |

Para cada execução registrar commit, dataset não sensível, parâmetros, p50,
p95, p99, taxa de erro, throttling, conexões, pool, locks, retries transacionais,
CPU/memória e saturação do banco. Abort imediato se houver saldo divergente,
premiação duplicada, deadlock persistente, p99 acima do timeout ou erro >5%.
