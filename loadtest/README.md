# Testes de carga — DNJ Game API

Esta pasta reúne o ferramental de carga da API. Comece por aqui: este README é o
**guia de execução** — como rodar, o que cada perfil faz e como observar o
comportamento da API ao vivo, incluindo o que entregar quando outra pessoa for
executar.

> ## ⚠️ ANTES DE RODAR: confirme o banco por trás do alvo
>
> A carga é isolada só até a Lambda. Do pooler para baixo, ela bate no banco real
> que estiver configurado naquele ambiente. **A Lambda de develop e a produção
> compartilham o mesmo projeto Supabase**, então apontar os perfis `soak`, `spike`
> ou `full` para a `K6_BASE_URL` de develop **derruba a produção** e, nas mutações,
> suja dados reais.
>
> **Nunca** rode acima do `smoke` contra um banco que serve usuários reais. Para
> dimensionar tier/capacidade, suba um **projeto Supabase descartável** (populado
> até o nº de usuários esperado), aponte uma Lambda de teste para ele, ajuste a
> `K6_BASE_URL` e teste contra esse alvo. Confirme o `DB_HOST` do ambiente antes de
> escalar.

```
loadtest/
├── README.md      ← você está aqui (guia de execução)
└── k6/            suíte k6 que reproduz o tráfego real do front
    ├── main.js, config/, lib/, scenarios/, report/, scripts/
    ├── README.md  referência técnica da suíte (variáveis, estrutura)
    └── .env.k6.example
```

> A ferramenta Go legada de smoke (`cmd/loadtest`, um perfil só, sem auth) continua
> existindo para o gate de CI (`make loadtest-smoke`). Para o teste realista com
> ≥ 5000 VUs, autenticação e relatório HTML, use a suíte **k6** descrita aqui.

A análise do volume de requisições do front que embasa os cenários está em
[`../docs/load-testing-k6.md`](../docs/load-testing-k6.md).

---

## 1. Pré-requisitos

- [k6](https://k6.io/docs/get-started/installation) ≥ 0.50 (`k6 version`).
- Bash, `curl`, `python3` (usados só pelo preflight).
- Para autenticar VUs: o **`JWT_IDENTITY_SECRET` de develop** e uma lista de
  **IDs de usuários reais** de develop.

---

## 2. Configuração (URL e credenciais)

Tudo é por variável de ambiente. Copie o exemplo e preencha:

```bash
cp loadtest/k6/.env.k6.example loadtest/k6/.env.k6
# edite loadtest/k6/.env.k6 -> preencha JWT_IDENTITY_SECRET
```

A URL da develop já vem embutida como default; troque `K6_BASE_URL` para bater em
outro ambiente. Os tokens dos VUs são **assinados localmente** (HS256): o middleware
de auth só valida a assinatura e as claims, não consulta o banco.

### IDs de usuários reais

```bash
psql "$DEVELOP_DATABASE_URL" -Atc "$(cat loadtest/k6/scripts/k6-export-users.sql)" \
  > loadtest/k6/data/users.json
```

`data/users.json` e `.env.k6` são **gitignored** (contêm segredo). Inclua alguns
`EVENT_MANAGER` no arquivo para o cenário de gestor rodar.

---

## 3. Rodar

```bash
# 1. Preflight — valida ulimit, RAM, healthcheck e um token real. NÃO pule.
make k6-preflight

# 2. Smoke (50 VUs, ~2 min). Confirme veredito APROVADO no relatório antes de escalar.
PROFILE=smoke make k6-run

# 3. Carga alvo (≥ 5000 VUs). Eleve o limite de descritores antes:
ulimit -n 1048576
PROFILE=full  make k6-run
```

Cada execução gera, em `loadtest/k6/out/`:
- `report-<perfil>-<timestamp>.html` — relatório visual (abra no navegador).
- `summary-<perfil>-<timestamp>.json` — dados brutos.

---

## 4. O que cada perfil faz (comportamento esperado)

| Perfil | Participantes | Outros cenários | Duração | Para quê |
|---|---:|---|---|---|
| `smoke` | 50 | telão ×2 | ~2 min | validação segura; **zero 5xx** tolerado |
| `soak` | 1500 | telão, gestor, mutações | ~20 min | carga sustentada, procura vazamento/degradação lenta |
| `spike` | 1000 | pico coordenado ×800 | ~5 min | todos abrindo o Game ao mesmo tempo |
| `full` | **5000** | telão, gestor, mutações, pico ×1500 | ~25 min | teste alvo completo |

Os cenários rodam **em paralelo e coordenados por tempo**:
- **participant** — 1 VU = 1 participante, com os 4 pollings de 15 s desalinhados em
  fase (jitter por VU) e a tela game trocando para polling de 5 s. Reproduz ~16 req/min
  por VU.
- **display** — telão/TV: 3 GETs públicos (rankings + live-display) a cada 15 s.
- **manager** — gestor: `manager/game-overview` a cada 15 s.
- **spike** — dispara num `startTime` fixo (no `full`, aos 12 min): milhares abrindo o
  Game simultaneamente, batendo no endpoint mais pesado (`game/overview`).
- **mutations** — like (toggle) + mark-read idempotentes, com `Idempotency-Key` único.

O teste **aborta sozinho** se a taxa de erro passar do orçamento do perfil (1% no smoke,
até 5% no full) ou se houver qualquer 5xx no smoke — critérios de
`docs/game-frontend-handoff.md`.

---

## 5. Runbook de observabilidade

Carga sem observação não vale. Siga na ordem: descobrir os limites antes, abrir as
abas certas, vigiar limiares durante, e reconciliar os logs depois. Toda a
observabilidade da API já existe (um evento `http_request_completed` por request); o
rigor está em olhar as fontes certas na janela certa.

### 5.1 Antes de subir carga (descoberta obrigatória)

```bash
bash loadtest/k6/scripts/k6-aws-discover.sh   # requer credenciais AWS válidas
```

Anote e leve para o `.env.k6` e para o relatório:
- **Nome da função** e **log group** (`/aws/lambda/<nome>`) → `K6_LAMBDA_FUNCTION_NAME`.
- **Memória, timeout, arquitetura**.
- **Concorrência reservada/provisionada** da função e a **cota de concorrência da conta**
  em `sa-east-1` (padrão 1000). Sem isso você está cego para throttling — ver §5.4.

Confirme também, antes: retenção do log group ativa (senão você perde a evidência), e
a **porta do pooler** em develop (5432 sessão vs 6543 transação) no Supabase.

### 5.2 Abas para deixar abertas (abra ANTES do `PROFILE=full`)

| # | Onde | O que | Filtro / caminho exato |
|---|---|---|---|
| 1 | CloudWatch → **Live Tail** | erros em tempo real | log group da função · `{ $.msg = "http_request_completed" && $.status >= 500 }` |
| 2 | CloudWatch → **Live Tail** (2ª aba) | panics | mesmo log group · `{ $.msg = "http_request_panic" }` |
| 3 | CloudWatch → **Metrics → AWS/Lambda** (gráfico da função) | `ConcurrentExecutions` (Max), `Throttles` (Sum), `Errors` (Sum), `Duration` (p99) — período 1 min | seleção por `FunctionName` |
| 4 | CloudWatch → **Lambda Insights** (se a layer estiver ativa) | memória usada e cold starts | dashboard da função |
| 5 | **Supabase → Database → Roles/Connections** | conexões client vs server no pooler | comparar com o teto do plano |
| 6 | **Supabase → Reports → Database** | CPU, IO e "Database connections" na janela | — |
| 7 | terminal do **k6** | RPS, VUs, erros, thresholds cruzados | saída ao vivo |

### 5.3 Durante — limiares e o que cada número significa

| Sinal | Onde | Verde | Atenção | ABORTAR |
|---|---|---|---|---|
| p99 de `http_req_duration` | k6 / aba 3 | < 3 s | 3–8 s | encosta em **10 s** sustentado |
| taxa de erro | k6 | < 1% | 1–5% | **> 5%** sustentado (o k6 aborta sozinho) |
| `Throttles` | aba 3 | 0 | qualquer valor | crescendo request após request |
| `ConcurrentExecutions` | aba 3 | folga vs cota | ~70% da cota da conta | no teto da cota |
| conexões do pooler | aba 5 | folga | ~80% do plano | no teto do plano / erro de conexão |
| 5xx / panic | abas 1 e 2 | silêncio | 5xx esporádico | fluxo de 5xx ou **qualquer panic** |

Números concretos deste ambiente (cota de concorrência da conta = **1000** em
`sa-east-1`; latência observada no navegador = **50–80 ms**): trate
`ConcurrentExecutions` **< 700** como verde, **700–900** como atenção, **≥ 900** como
abortar. Ver a conta de capacidade em §5.4.

Regra prática: `ConcurrentExecutions × DB_MAX_OPEN_CONNS (4)` ≈ conexões no Supabase.
Se a aba 3 mostra 200 execuções concorrentes, espere ~800 conexões no pooler (aba 5).

Correlação 1:1: cada request leva `X-Request-ID = k6-<cenário>-<vu>-<iter>-<seq>`,
ecoado na resposta e no log. Viu um 5xx na Live Tail? O `requestId` diz qual VU,
iteração e passo o gerou.

### 5.4 Throttling — o cuidado central

**Conta de capacidade (Lei de Little: `concorrência = req/s × latência`).** Com cota de
**1000** execuções concorrentes e latência saudável de **~65 ms**, o teto teórico é
`1000 / 0,065 ≈ 15 000 req/s` — muito acima do que o teste gera. O baseline de 5000
participantes é ~1300 req/s, o que ocupa só `1300 × 0,065 ≈ 85` execuções concorrentes:
**~8,5% da cota**. Ou seja, em regime saudável **não há throttling**, e há folga enorme.

O perigo não é o volume; é a **espiral latência↔concorrência**. Se o pooler do Supabase
começar a saturar e a latência subir, a concorrência sobe junto para o mesmo RPS:

| Latência por request | Concorrência a 1300 req/s | Situação |
|---|---:|---|
| 65 ms (saudável) | ~85 | verde, 8,5% da cota |
| 200 ms | ~260 | atenção — algo já degradou |
| 500 ms | ~650 | perto do limite operacional |
| **≥ 770 ms** | **≥ 1000** | **estoura a cota → Throttles** |

Traduzindo: a latência teria que piorar **~11×** (de 65 ms para ~770 ms) antes de a cota
morder. Então **o gatilho de abort não é o RPS, é a latência**: se p99 e
`ConcurrentExecutions` sobem juntos, você está entrando na espiral — pare antes dos 900.
O `spike` (todos abrindo o Game de uma vez, no endpoint mais pesado) é onde essa espiral
pode aparecer primeiro; é por isso que ele é um cenário separado e coordenado.

Há duas camadas de throttling, e elas se retroalimentam:

- **A API não tem rate limit HTTP.** Ela não vai se proteger devolvendo 429; ela vai
  aceitar tudo até o pooler do Supabase ou a concorrência da Lambda saturarem. Ou seja,
  o freio tem que ser **operacional** (você vigiando as abas 3 e 5), não automático.
- **O front amplifica falha.** O cliente real retenta mutações **3× sem backoff** em
  429/5xx (`src/lib/api/client.ts`). Sob degradação, o tráfego de mutação **triplica na
  hora** — o cenário `mutations` do k6 não reproduz esse efeito de propósito (para não
  falsear a medição), então **some mentalmente esse fator 3** ao interpretar um incidente.
- **Throttle da Lambda derruba requests inteiros.** Se `ConcurrentExecutions` bater na
  cota da conta em `sa-east-1`, a AWS responde `Throttles` (429 antes de invocar sua
  função) e outras funções da conta sofrem junto.

Guarda-corpos, em ordem:

1. **Descubra a cota** (§5.1). Se o pico projetado do `full` (baseline ~1300 req/s +
   spike) puder passar da cota, **não rode o `full` cru**: comece por `spike` isolado
   e suba `K6_SPIKE_VUS` aos poucos.
2. **Proteja a conta**: considere setar **reserved concurrency** na função de develop
   antes do teste, isolando o blast radius das outras funções — e, se quiser matar o
   cold start no spike, **provisioned concurrency**. Ambos via console/CLI (fora do repo).
3. **Suba gradualmente** — os perfis já fazem ramp (`K6_RAMP`); não comece no pico.
4. **Vigie `Throttles` (aba 3) e o pooler (aba 5)** como gatilho de abort, não só a
   taxa de erro do k6: o throttle pode aparecer na infra antes de virar erro no cliente.

### 5.5 Depois — reconciliação (o rigor do log)

Feche o loop cruzando o relatório do k6 com o log da API. No **CloudWatch Logs
Insights**, na janela exata do teste (UTC):

```
filter msg = "http_request_completed"
| stats count() as reqs, pct(latencyMs,95) as p95, pct(latencyMs,99) as p99,
        avg(dbMs) as avgDbMs, avg(dbQueries) as avgQ by route, status
| sort reqs desc
```

```
filter msg = "http_request_completed"
| stats avg(latencyMs) as appMs, avg(dbMs) as dbMs,
        avg(dbMs)/avg(latencyMs)*100 as pctDb by route
| sort pctDb desc
```

O relatório HTML traz essas queries prontas na seção "Onde observar". Se o p95 do log
(lado servidor) divergir muito do p95 do k6 (lado cliente), a diferença é rede/fila
fora do processo. Se `pctDb` estiver alto numa rota, o gargalo é banco, não CPU.

> Nota de precisão: `latencyMs` e `dbMs` são inteiros truncados — requests sub-ms
> aparecem como `0`. Para percentis finos, a fonte é o k6; o log serve para **atribuir**
> o tempo (app vs DB) e para achar 5xx/panics por `requestId`.

Guarde os dois artefatos por execução: o `out/report-*.html` e o `out/summary-*.json`.

---

## 6. Passando o teste para outra pessoa executar

### O que o dono do teste entrega

Por canal seguro (nunca commitar nem colar em chat aberto):

| Item | O que é | Como obter |
|---|---|---|
| **`JWT_IDENTITY_SECRET`** de develop | assina os tokens dos VUs | env da Lambda develop (console AWS) ou cofre do time |
| **`data/users.json`** | IDs reais de develop (com alguns `EVENT_MANAGER`) | `scripts/k6-export-users.sql` contra o Postgres de develop |
| **`K6_BASE_URL`** | URL da API V2 alvo | já embutida (develop); entregue só se for outro ambiente |
| _(opcional)_ acesso read-only a CloudWatch e ao dashboard Supabase | para observar durante o teste | IAM read-only / convite no Supabase |
| _(opcional)_ **`K6_LAMBDA_FUNCTION_NAME`** | nome da Lambda develop | console AWS ou `scripts/k6-aws-discover.sh` |

Modelo de `.env.k6` a entregar:

```bash
K6_BASE_URL=https://ttwkfudhvvhuhp5yvsoydxggum0ictpg.lambda-url.sa-east-1.on.aws/v2
JWT_IDENTITY_SECRET=<<< segredo de develop >>>
K6_USERS_FILE=./loadtest/k6/data/users.json
K6_TOKEN_TTL_SECONDS=10800
PROFILE=smoke
K6_OUT_DIR=./loadtest/k6/out
# opcional (rotula as queries do relatório):
K6_LAMBDA_FUNCTION_NAME=<<< nome da lambda develop >>>
```

### O que o executor informa de volta

1. **Máquina** (CPU/RAM), rede, e se rodou de dentro da AWS (`sa-east-1`) ou fora.
2. **Janela em UTC** de início e fim de cada perfil (amarra o relatório às métricas).
3. **Arquivos** `out/report-*.html` e `out/summary-*.json`.
4. **Prints de observabilidade** da janela: pooler Supabase, `ConcurrentExecutions`/
   `Throttles`/`Duration` p99 da Lambda, memória e cold starts (Lambda Insights).
5. **Anomalias** com horário aproximado (5xx, timeouts, abort).
6. **Perfil e overrides** usados.

Template para colar na devolução:

```
### Execução de carga k6 — DNJ Game API
- Executor / máquina: ______ (CPU __ / RAM __ GB), rede: ______
- Rodou de: [ ] fora da AWS  [ ] EC2 sa-east-1
- Alvo (K6_BASE_URL): ______
- Perfis e janelas (UTC):
    - smoke: início __:__  fim __:__  -> APROVADO / REPROVADO
    - full : início __:__  fim __:__  -> APROVADO / REPROVADO   overrides: ______
- Anexos: report-*.html, summary-*.json
- Observabilidade:
    - Supabase pooler (conexões máx): ____   CPU máx: __%
    - Lambda ConcurrentExecutions máx: ____  Throttles: ____  Duration p99: ____ ms
    - Lambda Insights: mem máx __%  cold starts: ____
- Anomalias (UTC + descrição): ______
```

### Higiene

- `JWT_IDENTITY_SECRET` e `data/users.json` nunca vão para o git (já no `.gitignore`).
  Apague-os da máquina do executor ao fim, se for temporária.
- Tokens forjados expiram em 3 h; não precisam ser revogados.
- O teste roda contra **develop** (isolado de produção). Confirme `K6_BASE_URL` antes
  do `full`.

---

Referência técnica da suíte (todas as variáveis e a estrutura de arquivos):
[`k6/README.md`](./k6/README.md).
