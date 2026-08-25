# Checklist de release — contrato V2

Este checklist é específico do conteúdo da V2 (Iterações 1–10). Para o
processo mecânico de branch/tag/deploy, siga [`docs/RELEASE.md`](../RELEASE.md)
— este documento não o substitui, é um portão de conteúdo a passar **antes**
do passo 2 daquele guia ("criar a branch de release").

## 1. PRs pendentes

Confirme que os PRs abaixo foram revisados e mergeados em `develop`, **nesta
ordem** (cada um foi aberto empilhado sobre o anterior, então a ordem de
merge importa para o diff ficar legível — mergear fora de ordem funciona,
mas o GitHub vai reapresentar o diff acumulado até o merge do PR mais
antigo):

- [ ] #3 — Iteração 8 (notificações)
- [ ] #4 — Iteração 9 (gate de carga no CI)
- [ ] #5 — Iteração 10 (handoff final)

Depois de cada merge, confirme que o próximo PR na fila é rebaseado /
atualizado contra `develop` antes de revisar (evita revisar um diff que já
inclui mudanças já mergeadas).

## 2. Gates locais e de CI

Rode `make validate` a partir de um checkout limpo de `develop` pós-merge.
Deve terminar em verde, cobrindo:

- [ ] `wire`, `build`, `vet`
- [ ] `test-race` (`-race` sob `TZ=UTC`)
- [ ] `test-cover-check` (cobertura geral ≥ 90%; mantida ≥ 55%)
- [ ] `test-admin-cover-check` (≥ 91,4% serviço / 92,1% integrado)
- [ ] `test-iteration5-cover-check` (≥ 97,4% / 96,0%)
- [ ] `test-iteration6-cover-check` (≥ 90% / 90%)
- [ ] `test-iteration7-cover-check` (≥ 90% / 90%)
- [ ] `test-iteration8-cover-check` (≥ 90% / 90%)
- [ ] `test-iteration9-cover-check` (≥ 90% / 90%)
- [ ] `test-iteration10-cover-check` (≥ 90% / 90%)
- [ ] `test-migrations` (Postgres real; ver também o teste dedicado de
      CockroachDB — não faz parte de `test-migrations` mas deve rodar
      manualmente antes do corte: `go test ./internal/infrastructure/db/migrations/... -run TestMigrations_Cockroach`)
- [ ] `openapi` (spec 3.0.3, manifesto operação→teste e manifesto de handoff
      do frontend — `cmd/handoff-check` — todos consistentes com o código)
- [ ] `loadtest-smoke` (perfil CI smoke: 0 erros, 0 erros de servidor, p95
      dentro do orçamento — já roda como job `loadtest-smoke` em `pr.yml`,
      mas confirme que passou no PR mais recente)

Nenhum desses gates pode ser enfraquecido (threshold reduzido, teste pulado,
`--no-verify`) para fechar uma release — se algo está vermelho, é bloqueio,
não ajuste de número.

## 3. Migrations em produção

- [ ] Toda migration desde a última release é idempotente (`expand/backfill/
      contract`, ver [`docs/migrations.md`](../migrations.md)) e **nenhuma**
      migration já aplicada foi editada — `schema_migrations.checksum`
      detectaria a violação, mas confirme visualmente no diff também.
- [ ] `go run cmd/migrate/main.go` já roda automaticamente em `develop.yml`/
      `production.yml` antes do deploy — confirme no log do último deploy de
      `develop` que "All migrations completed successfully" apareceu sem
      erro, para o conjunto de migrations desta release.
- [ ] Nenhuma migration desta release faz cascata destrutiva ou apaga dados
      de usuário; migrations desta janela (Iterações 8–9) não tocam nenhuma
      tabela pré-existente além de adicionar colunas/tabelas novas.

## 4. Contrato e documentação

- [ ] `docs/openapi/dnj-v2.openapi.yaml` reflete exatamente as operações
      implementadas até esta release (`make openapi` já valida isso
      mecanicamente, mas revise manualmente qualquer breaking change de
      schema).
- [ ] `docs/handoff/DNJ-V2-FRONTEND-INTEGRATION.md` (handoff canônico),
      `docs/handoff/dnj-v2-frontend-integration.json` (manifesto — `make
      handoff-check` já valida contra o OpenAPI) e
      `docs/game-frontend-handoff.md` (grafos de request) estão atualizados
      para as operações desta release.
- [ ] Página `/develop/frontend-integration/` publicada e navegável após o
      deploy mais recente de `develop.yml` (confira visualmente — a
      publicação é um passo separado do teste automatizado).
- [ ] `docs/implementation/DNJ-V2-STATUS.md` reflete o estado real — nenhuma
      linha da matriz de iterações diz "Pendente" para algo que na verdade
      já foi mergeado, nem "Concluída" para algo que não passou pelos gates.
- [ ] Swagger UI de `develop` (<https://dnjtechteam.github.io/dnj-game-api/develop/v2/>)
      abre e reflete a spec após o deploy de `develop.yml` — confirme
      visualmente antes de promover para produção, já que a publicação do
      GitHub Pages é um passo separado do teste automatizado.

## 5. Perfis de carga não automatizados

Perfis de `develop` (soak/spike), o perfil `canary` de produção, e todo
perfil por fluxo autenticado da tabela em `docs/game-frontend-handoff.md`
("Perfis futuros de carga") **não rodam em CI** — só o smoke reproduzível
roda. Antes de uma release que aumenta significativamente carga ou muda
caminho quente (ex: nova iteração de mídia, jogos ou notificações em massa):

- [ ] Alguém com acesso autorizado a `develop` rodou pelo menos o perfil
      `develop soak autorizado` da tabela, dentro de uma janela combinada
      com o time (comando exato em
      [`docs/load-testing.md`](../load-testing.md)).
- [ ] Resultado (p50/p95/p99, taxa de erro, saturação de pool/conexões)
      registrado em `docs/implementation/DNJ-V2-STATUS.md` antes do corte.
- [ ] Se o resultado não coube no orçamento documentado, a release é
      bloqueada até a causa ser entendida — não até "parecer aceitável".

## 6. Segurança

- [ ] `/security-review` rodou sobre o diff acumulado de todas as Iterações
      desde a última release (não só o diff do PR mais recente) e nenhum
      achado de alta/média confiança ficou sem tratamento.
- [ ] Nenhum segredo, credencial, PII ou payload de mídia apareceu em log
      estruturado introduzido nesta janela — checagem pontual grep por
      `Authorization`, `password`, `token`, `checksum`, `Signature` nos
      arquivos de log/observabilidade tocados.

## 7. Rollback

- [ ] Para cada mudança de contrato desta release, existe um caminho de
      rollback sem perda de dados: migrations são `expand/backfill/contract`
      (nunca dropam coluna/tabela na mesma release que a introduz), e
      nenhuma migration desta janela é destrutiva o suficiente para impedir
      reverter o binário para a versão anterior enquanto o schema já
      migrado.

Só depois de todos os itens acima marcados, siga para o passo 2 de
[`docs/RELEASE.md`](../RELEASE.md) (criar `release/X.Y.Z`).
