# Carga e observabilidade final — Iteração 9

Esta iteração não adiciona domínio novo. Ela transforma os grafos de request
e a tabela de perfis já publicados em `docs/game-frontend-handoff.md` (seção
"Perfis futuros de carga") em uma ferramenta reproduzível
(`cmd/loadtest`) e em um gate real no CI, e usa a observabilidade já existente
(`RequestObservabilityMiddleware`, um evento estruturado `http_request_completed`
por requisição com `requestId`, `method`, `route`, `status` e `latencyMs`) como
fonte de verdade para os relatórios — nenhum endpoint de métricas novo foi
criado, pois o objetivo desta iteração é validar sob carga o que já existe,
não expandir a superfície da API.

## `cmd/loadtest`

Ferramenta Go standalone (`go run ./cmd/loadtest ...` ou o binário compilado)
que aplica **um** perfil de carga contra um servidor já em execução e falha
(`exit 1`) quando o `p95` observado ou a taxa de erro ultrapassam o
orçamento do perfil:

```bash
go run ./cmd/loadtest \
  -url http://localhost:8081/v1 -path /healthcheck \
  -concurrency 10 -rps 10 -duration 2m \
  -p95-budget-ms 500 -error-budget-percent 0.5 \
  -request-timeout-seconds 5 -max-server-errors 0
```

Características deliberadas:

- **Sem autenticação, sem mutação.** Só exercita endpoints públicos e
  somente-leitura (hoje: `/healthcheck`). Isso evita a necessidade de gerar
  identidades, JWTs ou qualquer dado sintético em qualquer ambiente — a regra
  de nunca criar dados artificiais em ambiente remoto vale também para
  ferramentas de carga, não só para testes de aplicação.
- **Requisições em voo não são canceladas no fim da janela.** O temporizador
  da duração do perfil só decide se uma **nova** requisição é disparada;
  uma requisição já em andamento sempre corre até completar. Cancelar no
  limite exato da janela mediria artificialmente uma taxa de erro maior do
  que a real.
- **Taxa determinada por um agendamento absoluto, não por um `time.Ticker`
  compartilhado nem pela resposta anterior.** Um único goroutine "pacer"
  calcula o instante-alvo de cada requisição (`início + i×intervalo`) e a
  dispara quando um worker estiver livre; se todos os workers ficarem
  ocupados por mais de um intervalo — o que aconteceria justamente sob a
  saturação que esta ferramenta existe para flagrar —, os "ticks" atrasados
  não são silenciosamente descartados (como aconteceria com o canal de um
  `time.Ticker`, que só bufferiza um tick pendente): assim que um worker
  libera, o atraso é recuperado imediatamente, preservando a contagem total
  de requisições pretendida ao longo de toda a janela.
- **Dois orçamentos de erro independentes.** `-error-budget-percent` é a
  taxa agregada (4xx + 5xx + falha de transporte); `-max-server-errors` é um
  teto adicional, e mais estrito, só sobre 5xx/falha de transporte
  (`-1` desabilita). O perfil **CI smoke reproduzível** usa os dois: até
  0,5% de erro agregado, mas **nenhum** 5xx tolerado — refletindo a coluna
  "0,5% e nenhum 5xx" da tabela de perfis.
- **Relatório em JSON** (`requests`, `errors`, `serverErrors`,
  `errorRatePercent`, `p50Ms`, `p95Ms`, `p99Ms`, `maxMs`) impresso em
  `stdout`; o resultado da avaliação (passou/falhou) vai para `stderr` e o
  código de saída.

Testado com `go test ./cmd/loadtest/... -race` (cobertura dedicada via
`make test-iteration9-cover-check`, ≥90% sobre a lógica de agregação e
execução do perfil — `main()` em si, como todo `cmd/*` deste repositório,
fica fora do gate de cobertura por ser só amarração de flags).

## Perfis versionados

A tabela completa de perfis (por ambiente e por fluxo do frontend) está em
`docs/game-frontend-handoff.md`, seção "Perfis futuros de carga". Esta
iteração implementa e automatiza apenas o perfil **CI smoke reproduzível**
(10 de concorrência, 10 RPS, burst 20, 2 min, orçamento de erro 0,5%) contra
`GET /healthcheck` — o único fluxo que não exige dados de conta nem
autenticação, portanto o único seguro para rodar sem supervisão em todo
`push`/PR.

### `make loadtest-smoke`

```bash
make loadtest-smoke
```

Sobe uma stack Postgres/MinIO local e descartável via `docker compose`
(nenhuma credencial de produção envolvida), compila e inicia o binário real
da API, aguarda `/healthcheck` responder, roda o perfil CI smoke contra ele e
sempre encerra o servidor ao final — mesmo se o perfil falhar. É o mesmo
alvo usado pelo job `loadtest-smoke` no workflow `pr.yml`.

Variáveis (`LOADTEST_URL`, `LOADTEST_PATH`, `LOADTEST_CONCURRENCY`,
`LOADTEST_RPS`, `LOADTEST_DURATION`, `LOADTEST_P95_BUDGET_MS`,
`LOADTEST_ERROR_BUDGET_PERCENT`, `LOADTEST_REQUEST_TIMEOUT_SECONDS`,
`LOADTEST_MAX_SERVER_ERRORS`) sobrescrevem o perfil sem editar o Makefile.

### Perfis de `develop` (soak/spike) e o modelo de produção — bloqueado

Os demais perfis da tabela (`develop soak autorizado`, `develop spike
autorizado`, `produção canary read-only`, e todos os perfis por fluxo que
exigem conta autenticada — abertura de Game, scan, upload/galeria,
finalização de run, etc.) **não foram executados nesta sessão** e ficam
registrados como bloqueio:

- Rodá-los contra `develop` exige acesso à infraestrutura real (URL/host
  autorizado, possivelmente uma janela de manutenção combinada com o time) e
  uma decisão de produto sobre quando é seguro gerar tráfego sintético
  sustentado contra um ambiente compartilhado — nenhuma das duas coisas é
  algo que esta sessão tem ou deveria decidir sozinha.
- Muitos desses fluxos exigem uma conta de teste autenticada com JWT válido;
  gerar isso em `develop` seria criar dados artificiais em ambiente remoto,
  o que as regras desta execução proíbem explicitamente.
- `cmd/loadtest` já suporta apontar para qualquer `-url`, incluindo
  `develop`; o que falta é a autorização e a execução manual, não a
  ferramenta. Quando alguém com acesso autorizar, o comando é:
  ```bash
  go run ./cmd/loadtest -url https://<host-de-develop>/v2 -path /healthcheck \
    -concurrency 100 -rps 50 -duration 30m \
    -p95-budget-ms 8000 -error-budget-percent 1
  ```
  (ajustando concorrência/RPS/duração conforme a linha do perfil desejado na
  tabela de `docs/game-frontend-handoff.md`).

Nenhuma carga mutante contra produção é permitida por design — a tabela já
existente restringe produção ao perfil `canary read-only`, e esta iteração
não altera essa restrição.

## Observabilidade final

A observabilidade estruturada por requisição já existia antes desta
iteração (`internal/infrastructure/api/middlewares/request_observability_middleware.go`):
cada requisição emite um evento `http_request_completed` com `requestId`
(correlação com `X-Request-ID`), `method`, `route`, `status`, `latencyMs` e
`responseBytes` — sem header, query string, corpo ou IP do cliente, para
manter credenciais e PII fora do log. `RecoveryMiddleware` garante que um
panic vira um `500` padronizado, com o mesmo `requestId`, em vez de
derrubar o processo.

Esta iteração não substitui nem expande esse mecanismo — o valida sob carga
sustentada via `cmd/loadtest`/`make loadtest-smoke`, e usa `p95`/`p99`
derivados diretamente da resposta HTTP (o mesmo sinal que o log estruturado
já registra por requisição) como o critério objetivo de gate. Adicionar um
endpoint de métricas agregadas (Prometheus ou similar) ficaria fora do
escopo desta iteração, que é operar e validar o que já existe, não introduzir
domínio novo.

## Correlacionando um relatório de carga com os logs

1. Rode o perfil e capture o `requestId` de qualquer requisição da janela
   (o cliente HTTP do `cmd/loadtest` não imprime `requestId` individualmente
   hoje — para investigar uma requisição específica, capture os logs do
   servidor durante a janela do perfil, filtrando por `route` e `status`).
2. Cruze `latencyMs` do log estruturado com o `p95Ms`/`p99Ms` do relatório
   do `cmd/loadtest` para confirmar que a cauda observada pelo cliente bate
   com a cauda observada pelo servidor (divergência grande sugere latência
   de rede/proxy fora do processo, não do handler).
3. Para uma reconciliação de saldo/ledger durante um perfil autorizado em
   `develop` (fluxos de jogos/pontos), use a mesma auditoria
   `SUM(point_entries.delta) = users.points` já publicada na Iteração 6 — a
   carga em si nunca deve introduzir divergência; se introduzir, é motivo de
   abortar o teste imediatamente, conforme já documentado em
   `docs/game-frontend-handoff.md`.
