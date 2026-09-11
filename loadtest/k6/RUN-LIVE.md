# Execução ao vivo — rodada de baseline no Micro

Roteiro para metralhar a API e medir o joelho do Micro com o código atual (sem
cache). É a rodada 1, o "antes". A rodada 2 (com cache) repete os mesmos passos
depois do deploy.

Seed é feito UMA vez (SEED_ONLY), rodamos vários perfis observando, e limpamos no
fim. Assim não seedamos/limpamos a cada perfil.

## Passo 0 — preencher credenciais (você)

Em `loadtest/k6/.env.k6`:
- `JWT_IDENTITY_SECRET=` → o segredo HS256 de develop (env da Lambda).

E exporte a conexão do banco para o seed (o MESMO banco que a Lambda usa):
```bash
export SEED_DATABASE_URL='postgres://USER:PASS@aws-0-sa-east-1.pooler.supabase.com:5432/postgres'
```
Backup do banco antes, como combinado.

## Passo 1 — abrir os painéis (5 abas)

Ordem de importância para o Micro:

1. **Supabase → Reports → Database**: CPU, e principalmente o **saldo de créditos de
   CPU / burst balance** (o Micro é burstável; é ESTE gráfico que despenca no soak).
2. **Supabase → Database → Connections/Roles**: conexões client vs server no pooler.
3. **CloudWatch → Live Tail** no log group da Lambda, filtro
   `{ $.msg = "http_request_completed" && $.status >= 500 }`.
4. **CloudWatch → Metrics → AWS/Lambda** (da função): `ConcurrentExecutions` (Max),
   `Throttles` (Sum), `Duration` (p99), `Errors` — período 1 min.
5. O **terminal do k6**.

## Passo 2 — seedar 15k (uma vez) + exportar identidades

```bash
SEED_ONLY=1 N=15000 G=200 P=5 M=20 \
  bash loadtest/k6/scripts/k6-run-managed.sh
```
Confere: deve dizer "~15020 identidades exportadas" e escrever
`loadtest/k6/data/users.json`.

## Passo 3 — preflight

```bash
make k6-preflight
```
Tudo verde (inclui GET /auth/session com token forjado = 200). Vermelho no token =
segredo errado.

## Passo 4 — escada de carga (observando os painéis a cada degrau)

```bash
# 4a. smoke: valida a cadeia inteira (auth + seed + alvo). ~2 min.
cd loadtest/k6 && k6 run -e PROFILE=smoke main.js

# 4b. soak moderado: sustenta e observa os CRÉDITOS DE CPU do Micro caírem. ~20 min.
#     comece modesto num Micro; suba se aguentar.
cd loadtest/k6 && k6 run -e PROFILE=soak -e K6_PARTICIPANTS=1500 -e K6_HOLD=20m main.js

# 4c. spike: pico coordenado, observa cold-start herd e recuperação.
cd loadtest/k6 && k6 run -e PROFILE=spike -e K6_SPIKE_VUS=800 main.js

# 4d. full no volume alvo — só se os anteriores sobreviveram.
ulimit -n 1048576
cd loadtest/k6 && k6 run -e PROFILE=full -e K6_PARTICIPANTS=15000 main.js
```

Entre cada degrau, olhe: CPU e créditos no Supabase, `ConcurrentExecutions`/
`Throttles` na Lambda, 5xx na Live Tail. **Pare a escada** no primeiro degrau que
mostrar CPU no teto sustentada, créditos zerando, `Throttles` > 0 crescendo, ou
erro > 5%. Num Micro, é provável parar cedo — e isso É o resultado (prova que sem
cache não cabe).

## Passo 5 — limpar o seed

```bash
psql "$SEED_DATABASE_URL" -f loadtest/k6/scripts/k6-seed-cleanup.sql
```
Confere: `users_restantes`, `grupos_restantes`, `entradas_restantes` = 0.

## Saída

Relatórios em `loadtest/k6/out/report-*.html` e `summary-*.json`, um por perfil.
Guarde para comparar com a rodada 2 (com cache).
