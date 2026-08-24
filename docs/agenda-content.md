# Agenda, conteúdo público e favoritos — Iteração 5

Este guia acompanha o contrato OpenAPI V2 `2.4.0`. O frontend foi consultado
somente para descobrir o consumidor atual; nenhum arquivo, commit ou push foi
feito no clone. O DNJ continua uma instalação de edição única: não existem
`events`, `event_id` ou seleção de edição.

## Contrato decidido

### Agenda

`GET /v2/schedule?view=home&sector=palco` é público e retorna exatamente
`{items,generatedAt}`. Cada item contém somente `id`, `title`, `description`,
`startsAt`, `endsAt`, `sector` e `state`. `sector` é o nome compatível da
projeção `{id,name,slug}` do `Space`; não há tabela, domínio ou coluna sector.
O filtro `sector` usa `spaces.slug`.

São consideradas somente Activities `kind=schedule`, não arquivadas e com
`startsAt < endsAt`. A visão completa ordena por `startsAt,name,id` e limita a
100 itens. `view=home` retorna todas as Activities simultaneamente `live` e,
depois, no máximo três futuras ainda não encerradas. Ausência e `home` são os
únicos valores de `view`.

O servidor captura `generatedAt` uma vez e deriva o estado nesta prioridade:

1. `ended` se `status=completed` ou `generatedAt >= endsAt`;
2. `live` se `status=active` e `startsAt <= generatedAt < endsAt`;
3. `upcoming` se `generatedAt < startsAt` e a diferença é menor ou igual a 15 minutos;
4. `scheduled` nos demais casos visíveis, inclusive `draft` e pausa.

A fronteira é fechada em 15 minutos: `15m + 1ns` é `scheduled` e exatamente
`15m` é `upcoming`. No instante exato do início, uma Activity `active` é
`live`; `draft` ou `paused` é `scheduled`. No instante exato do término é
`ended`.

### Activities públicas

`GET /v2/activities?kind=&spaceId=&page=` usa `{data,pagination}`, limite 10 e
ordem `startsAt NULLS LAST,name,id`. Os únicos filtros são `kind`, `spaceId` e
`page`; `kind` usa os cinco enums existentes e `spaceId` precisa ser UUID de um
Space persistido. A listagem e `GET /v2/activities/{activityId}` compartilham a
mesma visibilidade: schedule futuro em `draft`, ou qualquer Activity `active`,
`paused` ou `completed`; `archived` e draft não-schedule são invisíveis.
Ausência e invisibilidade retornam o mesmo `404 NOT_FOUND`.

O DTO contém somente `id`, `space`, `slug`, `name`, `description`, `kind`,
`startsAt`, `endsAt`, `checkInPoints`, `momentPoints`, `cooldownSeconds`,
`allowsMoment` e `state`. `space` é a projeção pública sem `mapReference`;
`state` é nulo quando não há janela válida. Assignments, status administrativo,
auditoria e timestamps administrativos nunca são expostos.

### Favoritos

`GET /v2/users/me/favorites?page=`, `PUT` e `DELETE
/v2/users/me/favorites/{activityId}` exigem JWT/cookie de identidade. Usuário,
papel atual e onboarding são sempre relidos do banco; todos os papéis existentes
podem favoritar depois do onboarding. A listagem retorna apenas Activities que
continuam visíveis, no mesmo DTO, paginação e ordem públicos.

PUT e DELETE retornam `204` sem corpo e exigem `Idempotency-Key` UUID. A tabela
`user_favorites` possui chave única `(user_id,activity_id)` e nenhuma
`event_id`. `participant_operations` guarda somente ator, chave, operação,
Activity, hash da intenção, status 204 e timestamp: não guarda PII, corpo,
token, segredo ou conteúdo. Retry igual retorna o resultado original sem novo
efeito; reutilização com operação ou Activity diferente retorna
`409 IDEMPOTENCY_KEY_REUSED`.

PUT aceita somente Activity publicamente visível. DELETE não consulta Activity
e retorna 204 para favorito presente, ausente ou UUID inexistente, impedindo
enumeração. Favoritar não é privilegiado e não grava `operation_audit`.
Locks no usuário serializam operações do mesmo ator; unicidade mantém segurança
com chaves diferentes.

## Datas e fuso

`generatedAt`, `startsAt` e `endsAt` saem em UTC RFC 3339 com `Z`. Escritas
administrativas podem receber offsets positivos ou negativos; o instante é
preservado e normalizado para UTC. O frontend deve formatar no fuso local atual
do dispositivo e converter uma escolha local para UTC antes de enviar. Nunca
deve apenas anexar `Z` a uma string local, nem fixar `America/Sao_Paulo` em
helpers finais de apresentação.

## Handoff incremental das Iterações 2–5

| Iteração | Arquivos/módulos atuais descobertos | Substituição necessária | Estado |
|---|---|---|---|
| 2 — identidade | `src/lib/api/client.ts`, `src/lib/api/auth.ts`, fluxo em `src/components/dnj-app.tsx` | sessão/senha e aliases V1 → Google OIDC, `/v2/auth/google`, refresh/CSRF, `/v2/auth/session`, onboarding | `ready` |
| 3 — perfil/grupos | `src/lib/api/groups.ts`, telas de grupo/perfil e `src/components/dnj-app.tsx` | `/api/v1` e payloads legados → `/v2/users/me`, `/v2/users/me/group`, `/v2/groups*` | `ready` |
| 4 — instalação | mapa, `src/components/admin/admin-dashboard.tsx`, `src/lib/manager-api.ts` e Route Handlers em `src/app/api/admin/**`/`manager/**` | Supabase/`experiences`/`/api/v1/spaces` → `/v2/spaces`, `/v2/admin/**`, `/v2/manager/activities/**` | `ready`; migração ampla do painel fica para o handoff final |
| 5 — agenda | `src/lib/api/schedule.ts`, `src/features/home/home-screen.tsx`, `src/features/schedule/schedule-screen.tsx` e testes correspondentes | cliente atual `/schedule` sob base antiga → `GET /v2/schedule`; remover timezone fixo de apresentação | `ready` |
| 5 — catálogo/detalhe | ainda não existe módulo público dedicado; há tipos parciais em `src/types/experience.ts` | criar `src/lib/api/activities.ts` e tela/rota de catálogo/detalhe usando `/v2/activities` | `blocked` pela decisão de navegação/tela, não pelo backend |
| 5 — favoritos | nenhuma ação ou store pública encontrada | criar `src/lib/api/favorites.ts`, estado otimista reversível e UI nas telas definidas pelo produto | `blocked` pela localização da UI, não pelo backend |

## Grafos de requests e orçamento

Convenção comum: timeout atual do cliente é 10 s. GET pode tentar novamente no
máximo duas vezes com backoff exponencial e jitter (250 ms, 750 ms); escritas
podem repetir somente com a mesma chave. Não há polling nesta iteração. Cada
efeito deve receber `AbortSignal` da tela; o booleano `active` atual evita
setState, mas não cancela a chamada. Cache de GET deve deduplicar por URL por 30
s; reconexão invalida no máximo uma vez. Offline usa cache somente leitura se
existir e nunca enfileira silenciosamente favorito.

### Home

```text
abrir app autenticado
  ├─ sessão/perfil (Iterações 2–3, pré-requisito para UserData)
  └─ em paralelo GET /v2/schedule?view=home [público, sem payload]
       └─ render live[] e próximas[] a partir da resposta única
```

- Autenticação: agenda pública; sessão segue seu fluxo próprio.
- Idempotência: não se aplica. Cache/dedupe: chave URL, 30 s.
- Cancelamento: abortar ao desmontar/trocar de tela. Polling: nenhum.
- Offline: último snapshot pode ser mostrado como desatualizado; sem snapshot,
  estado offline explícito.
- Orçamento: 1 request de agenda por montagem; cold app até 2 requests em
  fan-out contando sessão. Com retries: máximo 3 tentativas por request.
- Risco: re-render/reconexão montar `HomeScreen` repetidamente. A query precisa
  viver em cache compartilhado e não no ciclo de render.

### Agenda completa

```text
toque “Ver cronograma completo”
  └─ GET /v2/schedule [público]
       └─ render máximo de 100 itens na ordem recebida
```

- Orçamento: 1 request por abertura; nenhum fan-out e nenhum polling.
- Cache: chave `/schedule`, 30 s; pode aproveitar resposta completa anterior,
  nunca a resposta reduzida `view=home` como se fosse completa.
- Cancelamento/offline/retry: política comum. Risco principal é dupla chamada
  por remount/Strict Mode sem deduplicação.

### Catálogo de Activities

```text
abrir catálogo ou mudar filtro/página
  └─ cancelar request anterior
  └─ GET /v2/activities?kind={enum}&spaceId={uuid}&page={n} [público]
       └─ resposta alimenta lista e paginação
```

- Orçamento: 1 request por abertura, mudança estabilizada de filtro ou página.
- Dependência: `spaceId` vem de `/v2/spaces` já carregado ou seleção persistida;
  se a tela também buscar Spaces, são 2 chamadas paralelas e a Activity depende
  somente da seleção, não da resposta textual do Space.
- Cache/dedupe por conjunto ordenado de filtros; debounce de 250 ms para UI de
  filtro. Sem polling. Offline permite apenas páginas já cacheadas.

### Detalhe

```text
abrir Activity {id}
  └─ GET /v2/activities/{id} [público]
       ├─ 200 → render DTO público
       └─ 404 → mesma UI para ausente/invisível
```

- Orçamento: 1 request por abertura. Cache por ID por 30 s, invalidado após
  mudança conhecida de favorito somente para o indicador, não para o conteúdo.
- Sem fan-out obrigatório, payload ou idempotência. Cancelar ao desmontar.

### Lista de favoritos

```text
abrir favoritos após sessão válida
  └─ GET /v2/users/me/favorites?page={n} [JWT/cookie]
       └─ 401 → fluxo único de refresh e um retry deduplicado
            └─ falha → logout controlado
```

- Orçamento: 1 request por abertura/página; cold app pode somar sessão/refresh.
- Cache privado por usuário e página; limpar em logout. Nunca compartilhar
  cache entre identidades. Sem polling. Offline mostra snapshot privado marcado.
- Risco: cada componente de card buscar favoritos. A tela deve fazer uma única
  query paginada e distribuir estado.

### Favoritar e desfavoritar

```text
toque único
  ├─ gerar UUID uma vez para a intenção
  ├─ PUT ou DELETE /v2/users/me/favorites/{activityId}
  │    headers: Authorization/cookie + Idempotency-Key
  │    payload: nenhum
  ├─ 204 → atualizar/invalidate cache privado
  └─ timeout/rede → retry com a mesma chave (máximo 2)
```

- Orçamento: 1 request por ação; máximo 3 tentativas da mesma intenção.
- Deduplicar cliques enquanto a intenção estiver em voo. Uma nova ação inversa
  recebe chave nova e só começa após resolver/cancelar visualmente a anterior.
- Offline: não assumir sucesso e não criar fila mutante automática; oferecer
  botão de tentar novamente com a mesma chave enquanto a intenção existir.
- Risco: handler recreado em re-render, retry genérico gerar nova chave ou
  reconexão repetir ação. A chave pertence ao objeto de intenção, não ao fetch.

## Backlog granular do frontend

| ID | Pri. | Item | Dependências/endpoints | Bloqueio | Estado | Teste de aceite | Evidência esperada |
|---|---:|---|---|---|---|---|---|
| FE5-01 | P0 | Fazer `apiRequest` aceitar `AbortSignal` externo sem perder timeout | cliente V2 | nenhum | `ready` | desmontar cancela fetch e não atualiza estado | teste com AbortError controlado |
| FE5-02 | P0 | Remover `America/Sao_Paulo` dos dois formatadores de agenda | `/v2/schedule` | nenhum | `ready` | teste muda TZ e mostra fuso do dispositivo | teste unitário sob dois TZs |
| FE5-03 | P0 | Apontar `scheduleApi` explicitamente para base `/v2` | GET schedule | FE5-01 | `ready` | home e agenda não usam proxy `/api/v1` | captura de request em teste |
| FE5-04 | P0 | Deduplicar agenda por URL e separar cache home/completo | GET schedule | FE5-03 | nenhum | `ready` | remount simultâneo gera uma chamada | teste de duas montagens |
| FE5-05 | P0 | Criar tipos exatos de PublicActivity/Pagination | GET activities/favorites | nenhum | `ready` | fixture rejeita campos administrativos | typecheck + contract fixture |
| FE5-06 | P0 | Criar `activitiesApi.list/get` com filtros publicados | GET activities | FE5-01/05 | nenhum | `ready` | URL canônica, cancelamento e 404 uniforme | testes do módulo API |
| FE5-07 | P1 | Definir navegação e tela do catálogo | GET activities | decisão de produto | `blocked` | rota abre lista paginada | vídeo/screenshot + e2e |
| FE5-08 | P1 | Definir navegação e tela de detalhe | GET activity | FE5-07 e decisão de produto | `blocked` | 200/vazio/404/retry cobertos | e2e |
| FE5-09 | P0 | Criar `favoritesApi.list/put/delete` sem corpo | favoritos | FE5-01/05 e auth I2 | nenhum | `ready` | PUT/DELETE enviam chave estável e tratam 204 | teste de retry com mesma chave |
| FE5-10 | P1 | Definir locais dos botões/aba de favoritos | favoritos | decisão de produto | `blocked` | acessibilidade e estados definidos | design aprovado + e2e |
| FE5-11 | P0 | Implementar estado otimista reversível e dedupe de clique | PUT/DELETE | FE5-09/10 | FE5-10 | `blocked` | duplo clique gera uma intenção; falha reverte | teste concorrente de UI |
| FE5-12 | P0 | Cache privado por identidade e limpeza em logout | GET favorites | auth I2, FE5-09 | nenhum | `ready` | troca de usuário não reaproveita dados | teste multi-identidade |
| FE5-13 | P0 | Integrar refresh único para fan-out com 401 | todas protegidas | Iteração 2 | nenhum | `ready` | N respostas 401 geram um refresh | teste de fan-out |
| FE5-14 | P1 | Instrumentar contagem de requests/retry/reconexão | todos | FE5-03/06/09 | nenhum | `ready` | orçamento excedido falha teste | relatório do teste |

Trabalho obrigatório: FE5-01–06, 09, 12–14 e as decisões FE5-07/08/10.
Limpeza posterior: remover aliases V1 e Route Handlers antigos somente após
telemetria. Fora de escopo: QR, participações, runs, jogos, Moments, mídia,
ranking, anúncios e notificações.

## Entradas não destrutivas para a Iteração 9

Nenhum stress amplo foi executado. Os perfis abaixo são modelos para massa
sintética sem PII. Produção é somente cálculo/capacidade, nunca alvo mutante.

| Ambiente | Concorrência | RPS/burst | Duração | Timeout | Rate limit esperado | Pool DB | Orçamento de erro |
|---|---:|---:|---:|---:|---:|---:|---:|
| local | 1–2 | 2 / 4 | 30 s | 10 s | desligado | 5 | 0% |
| CI smoke | 5 | 5 / 10 | 60 s | 10 s | acima de 10 RPS | 10 | <1%, zero 5xx sustentado |
| develop smoke | 20 | 20 / 40 | 5 min | 10 s | documentar 429 real | 20 | <1% |
| develop soak autorizado | 200 | 100 / 200 | 30 min | 10 s | respeitar Retry-After | 40 | <1%, p99 <2 s |
| develop spike autorizado | 1.000 | 500 / 1.000 | 5 min | 10 s | 429 esperado e medido | 60 | <2%, sem exaustão |
| produção (modelo, não executar) | 2.000–10.000 conexões | 500 / 1.000 | janela de 30 min | 10 s | a definir por telemetria | dimensionar por p95 e saturação | <1% |

| Cenário reproduzível futuro | Grafo lógico | Concorrência / RPS / burst | Métricas obrigatórias |
|---|---|---|---|
| cold start | health/readiness → sessão + home em paralelo | CI 5/5/10; develop 20/20/40 | cold latency, p50/p95/p99, 5xx, conexões |
| login/refresh | Google → sessão; N 401 → um refresh → retries | CI 5/2/5; develop 50/20/50 | refresh deduplicado, 401/403, throttling |
| home fan-out | sessão/perfil ∥ schedule home | CI 5/5/10; develop 100/50/100 | requests por abertura, p95 agregado |
| agenda completa | schedule sem view, até 100 itens | CI 5/5/10; develop 100/100/200 | payload, CPU, p99, cache hit |
| catálogo/detalhe | list filtros → detalhe | CI 5/5/10; develop 200/100/200 | query latency, pool, scans |
| favoritos | list ∥ PUT/DELETE com 10% retries | CI 5/3/6; develop 100/50/100 | conflitos, locks, duplicatas, 5xx |
| retry/reconexão | queda curta → backoff e dedupe | CI 5/5/10; develop 100/50/100 | amplificação de requests, Retry-After |

Os relatórios da Iteração 9 devem correlacionar p50/p95/p99, erros, 429,
conexões abertas/espera de pool, CPU e saturação do banco por endpoint e fluxo.
Smoke de carga entra no CI; soak/spike/stress só em develop com autorização e
abort automático. O manifesto final da Iteração 10 deve consumir estes grafos.

## Testes e evidência

- `iteration5_service_test.go`: relógio fixo, fronteiras, visibilidade, estados,
  home, limite, filtros, paginação, UTC, favoritos, retries e concorrência.
- `iteration5_http_integration_test.go`: middleware → handler → service →
  repository → PostgreSQL real.
- `iteration5_handler_test.go` e `iteration5_router_test.go`: queries estritas,
  matriz 400/401/404/409/500 e proteção de favoritos.
- `migrations_integration_test.go`: clean install, replay, upgrade preservando
  Iteração 4, unicidade e ausência de `event_id`.
- `docs/openapi/dnj-v2.operations.yaml`: cada uma das seis operações aponta para
  a evidência automatizada correspondente.

## Entrega final preservada

A Iteração 10 continua obrigada a gerar
`docs/handoff/DNJ-V2-FRONTEND-INTEGRATION.md`,
`docs/handoff/dnj-v2-frontend-integration.json`, a página publicada em
`/develop/frontend-integration/` iniciada por checklist ordenado “faça isto” e
o artefato de workflow com página, Markdown, manifesto, OpenAPI e exemplos
executáveis. CI deve falhar se página/manifesto divergirem do OpenAPI ou do
manifesto operação→testes. Uma pessoa com somente esse artefato deverá conseguir
implementar, validar e reverter a integração sem escrita no frontend.
