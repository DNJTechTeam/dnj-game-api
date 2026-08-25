# DNJ V2 — Status executivo e matriz de atendimento

Atualizado em: 2026-08-24

Branch de execução: `develop`

Contrato canônico: `/v2`
Legado: `/v1` preservado até que exista evidência de migração do consumidor

Este arquivo é a fonte de verdade entre sessões. Nenhum endpoint é marcado
como concluído antes de possuir implementação, contrato OpenAPI, teste
automatizado e evidência no ambiente `develop`.

## Decisões fechadas

- O backend é o único repositório alterado; o frontend recebe um handoff pronto.
- O DNJ é uma instalação de edição única. Não existe tabela `events`, coluna
  `event_id`, seleção de evento ou suporte multi-evento na V2.
- Filas e comentários estão fora do escopo da V2 atual.
- Login Google cria perfil incompleto quando CPF, telefone ou grupo faltarem;
  recursos protegidos exigirão conclusão explícita do onboarding.
- Dados existentes em `develop` são preservados. É proibido resetar o banco
  como estratégia de rollout.
- Fotos permanecem por 90 dias após o término da edição do DNJ; limpeza será assíncrona
  e auditável.
- Capacidade alvo: 2–10 mil conexões simultâneas, com ensaio posterior a 2x da
  carga esperada e critérios objetivos de abortar o teste.
- Portas locais reservadas: frontend `3000`, API `8081`, PostgreSQL `55432`,
  S3/MinIO `59000`, console MinIO `59001`.

## Definition of Done por operação

1. Regra de negócio e autorização implementadas nas camadas corretas.
2. Migration `expand/backfill/contract` idempotente quando houver persistência.
3. Testes de unidade, integração e HTTP cobrindo sucesso e cada erro publicado.
4. `operationId` único no OpenAPI 3.0.3 e registro em
   `docs/openapi/dnj-v2.operations.yaml` apontando para o teste automatizado.
5. Sem segredo ou PII em log; erro no envelope `{code,message,details?,requestId}`.
6. Gates `make validate` verdes e smoke no `develop` registrado neste documento.
7. Guia de uso/manutenção e exemplos atualizados para consumo sem contexto do
   código do backend.

## Matriz de implementação

| Iteração | Capacidade | Operações V2 planejadas | Persistência principal | Estado |
|---|---|---|---|---|
| 1 | Enablers e trilho | `GET /healthcheck`, `GET /readiness` | `schema_migrations.checksum` | Implementada e publicada em develop |
| 2 | Identidade e Google | Google, refresh/logout, sessão atual, completar perfil | usuários, identidades, refresh sessions | Concluída e publicada em develop |
| 3 | Perfil e grupos | perfil atual, grupo atual, membros, convites/códigos | perfis, grupos, memberships, invites | Concluída e publicada em develop |
| 4 | Configuração da instalação | descoberta/operação de Activities e configuração administrativa de Spaces, Activities, staff e assignments | spaces, activities, assignments, auditoria, idempotência administrativa | Concluída; enabler administrativo publicado em develop |
| 5 | Agenda e conteúdo | agenda, atividades, detalhes, favoritos | activities, user_favorites, participant_operations | Concluída e publicada em develop |
| 6 | Jogos e ranking | catálogo, runs, QR, participações, pontuação e ranking | Activities competitivas, activity_runs, participations, point_entries | Concluída e publicada em develop |
| 7 | Mídia | upload assinado, confirmação, galeria, moderação, retenção | media_assets, moments, moment_likes, moderação, jobs de retenção | Concluída localmente; validação de publicação em develop em andamento |
| 8 | Notificações | preferências, listagem, leitura e envio administrativo | notificações, preferências, deliveries | Pendente |
| 9 | Operação e carga | observabilidade final, segurança e soak/spike/stress baseados nos grafos reais de requests do frontend | perfis de carga, métricas e relatórios, sem novo domínio | Pendente |
| 10 | Handoff final | OpenAPI publicado, página única de integração do frontend, manifesto, exemplos e checklist de release | documentação versionada e artefato publicado pelo CI | Pendente |

Os nomes e formatos exatos das operações das iterações 2–8 serão fechados a
partir das regras do frontend e dos handoffs antes de entrar no OpenAPI. Uma
lacuna encontrada vira um enabler documentado e testado; não vira suposição
silenciosa.

### Entrega obrigatória da Iteração 10 para o frontend

A última iteração deve consolidar os handoffs incrementais em um único pacote,
sem exigir que a equipe do frontend reconstrua decisões a partir do histórico:

- fonte canônica versionada em `docs/handoff/DNJ-V2-FRONTEND-INTEGRATION.md`;
- manifesto legível por máquina em
  `docs/handoff/dnj-v2-frontend-integration.json`, validado contra o OpenAPI e o
  manifesto operação→testes;
- página HTML publicada em `/develop/frontend-integration/`, com link visível na
  página principal da documentação V2;
- artefato do workflow contendo a página, o Markdown, o manifesto, o OpenAPI
  3.0.3 e exemplos executáveis.

O conteúdo deve reunir, por tela e fluxo do frontend: ordem de rollout das
Iterações 2–8, configuração de ambiente, autenticação/refresh/CSRF, permissões,
operações e DTOs, paginação, idempotência, códigos de erro, estados vazios e de
retry, UTC no transporte e fuso local na apresentação, substituições das rotas
V1/Supabase, remoção segura de aliases, exemplos de chamadas e respostas,
matriz tela→operação→status→teste e checklist de aceite/rollback. A geração deve
falhar no CI se a página ou o manifesto divergirem do OpenAPI publicado.

O material precisa ser pragmático e utilizável sem permissão de escrita no
repositório do frontend. Para cada fluxo, deve informar: arquivos/módulos atuais
afetados, rota antiga→rota V2, pré-requisitos, sequência de implementação,
snippet TypeScript copiável, estados de UI esperados, testes que o frontend deve
criar e critério objetivo de pronto. Deve incluir helpers de referência para
cliente autenticado, refresh/CSRF, UUID de idempotência e conversão
UTC↔fuso-local sem timezone fixo. A página deve abrir com um checklist ordenado
"faça isto" e separar claramente trabalho obrigatório, limpeza posterior e
itens fora de escopo.

O backlog final deve ser granular: cada item informa tela/fluxo, prioridade,
dependências, endpoints envolvidos, responsável sugerido, bloqueios, estado
`pending|ready|blocked|done`, teste de aceite e evidência esperada. Itens amplos
como "integrar agenda" devem ser decompostos até que cada passo possa ser
implementado e verificado isoladamente.

Para fluxos que disparam várias chamadas ao backend, o handoff deve incluir um
grafo de requests com gatilho, ordem, fan-out paralelo, dependências, payload,
autenticação, idempotência, cache/deduplicação, cancelamento, timeout, retry com
backoff, polling e comportamento offline. Também deve registrar o número máximo
esperado de requests por abertura de tela e por ação do usuário, evitando que
re-render, reconexão ou retry multipliquem carga silenciosamente.

A Iteração 9 deve transformar esses grafos em perfis de carga versionados e
reproduzíveis. Para cada ambiente (`local`, `CI`, `develop` e produção apenas
como modelo não destrutivo), documentar limites, massa sintética, concorrência,
RPS, burst, duração, pool de conexões, timeout, rate limit e orçamento de erro.
Os cenários devem cobrir cold start, login/refresh, abertura da home com fan-out,
agenda, favoritos, ranking, upload/galeria, polling e retries; executar smoke de
carga no CI e soak/spike/stress autorizados em `develop`, nunca carga mutante em
produção. Relatórios precisam correlacionar p50/p95/p99, erros, throttling,
conexões e saturação do banco por fluxo e por endpoint, com thresholds que
falham o gate.

Critério de aceite: uma pessoa com acesso somente ao artefato publicado deve
conseguir migrar o frontend, validar cada fluxo e executar o rollback sem ler o
histórico das iterações nem consultar o código interno da API. O clone local do
frontend permanece fonte de descoberta somente leitura; a Iteração 10 não
depende de commit ou push nele.

## Evidência da Iteração 1

Commit de implementação: `2314d06`

| Controle | Evidência executada em 2026-08-21 |
|---|---|
| Gate agregado | `make validate`: Wire, build, vet, race, cobertura, migrations e OpenAPI verdes |
| Migration limpa e replay real de cada `Up` | `make test-migrations`: verde em PostgreSQL 16 real |
| Upgrade de schema parcial legado | `TestMigrations_UpgradeLegacyPartialSchema` |
| Quatro runners concorrentes | `TestMigrations_ConcurrentRunnersSerialize` |
| Backfill e drift de checksum | `TestMigrations_ChecksumBackfillAndDriftDetection` |
| Request ID, log seguro e recovery | testes de `request_observability_middleware` |
| Health/readiness e falha do DB | testes de `healthcheck_handler` |
| Cobertura operação ↔ teste | `make openapi-check` |
| Cobertura de código mantido | 58,1% ≥ gate 55%; baseline a elevar até 90% |
| Cobertura crítica de mappers/repositories | 100% ≥ gate 90% |
| Infra local | PostgreSQL 55432 e MinIO 59000/59001 saudáveis via Compose |
| Smoke HTTP local | health 200, readiness 200, request ID gerado/preservado e logs JSON |
| Banco indisponível | cold start HTTP funciona; health 200 e readiness 503 correlacionado |
| Publicação documental | script validado com V1 em `/develop/` e V2 em `/develop/v2/` |

## Enabler de banco de develop resolvido

- Falha original: run
  <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32538460606>,
  `SQLSTATE 53300` por limite mensal de Request Units.
- O banco configurado foi substituído/restaurado como CockroachDB saudável,
  sem reset nem bypass de migrations.
- Compatibilidade direta aplicada: lock transacional por linha com
  `SELECT ... FOR UPDATE`; nenhuma função PostgreSQL específica ou detecção de
  engine foi adicionada.
- Commit do enabler: `0b229a2`.
- Deploy verde: run
  <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32542284797>.
- Migrations e checksums: concluídos pelo workflow; replay local real também
  verde em CockroachDB `v25.2.19`, com 10 migrations e 0 checksum ausente.
- URLs validadas:
  - API: <https://ttwkfudhvvhuhp5yvsoydxggum0ictpg.lambda-url.sa-east-1.on.aws/v2>
  - OpenAPI: <https://dnjtechteam.github.io/dnj-game-api/develop/v2/>
- Smokes do commit `0b229a2`: `GET /healthcheck` 200, `X-Request-ID` gerado e
  preservado, UI e JSON OpenAPI 200/3.0.3. O readiness retornou 503 naquela
  medição; a configuração de runtime da Lambda é gerenciada externamente e não
  foi alterada pelo workflow.

## Evidência local da Iteração 2

| Controle | Evidência em 2026-08-22 |
|---|---|
| Gate agregado | `make validate`: build, vet, race, cobertura, migrations e OpenAPI verdes |
| Banco primário de teste | PostgreSQL 16 real via Testcontainers |
| Compatibilidade CockroachDB | migrations aplicadas e repetidas em `v25.2.19`; 10 checksums presentes |
| Cobertura crítica | mappers/repositories 96,8% ≥ 90% |
| Google sem rede em teste | verifier isolado; issuer e `email_verified` testados; assinatura/audience/expiração delegados ao verificador oficial |
| Linking seguro | subject primeiro, email verificado exato, conflitos 409 e índices únicos concorrentes |
| Sessão | access 15 min; refresh 30 dias com hash, rotação, reuse familiar e logout testados |
| HTTP e cookies | handlers reais, `HttpOnly`/`Secure`/`SameSite`, CSRF double-submit e erros publicados testados |
| Contrato | OpenAPI 3.0.3 v2.1.0 e manifesto operação→testes consistentes |

Dependências externas e etapas posteriores:

- `GOOGLE_CLIENT_ID` e `DOCUMENT_HMAC_SECRET` foram configurados no runtime de
  `develop` e validados pelo smoke final.
- O frontend ainda precisa integrar o SDK Google e consumir este contrato; o
  repositório frontend não foi alterado.
- A remoção de `users.document` é a etapa contract posterior à migração total
  dos consumidores V1; até lá o campo legado é preservado sem perda de dados.

## Deploy e smokes remotos da Iteração 2

- Commit: `6cc61ac`.
- Run verde: <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32599410466>.
- Revalidação do deploy após versionar o status: run
  <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32599712886>, verde.
- API: <https://ttwkfudhvvhuhp5yvsoydxggum0ictpg.lambda-url.sa-east-1.on.aws/v2>.
- OpenAPI: <https://dnjtechteam.github.io/dnj-game-api/develop/v2/>.

| Smoke em 2026-08-22 | Resultado |
|---|---|
| Migrations/checksums CockroachDB | etapa do workflow verde |
| `GET /healthcheck` | 200 `ok` |
| `GET /readiness` | 200 `ready` |
| `X-Request-ID` gerado | 200 com id seguro retornado |
| `X-Request-ID` preservado | `smoke-env-iteration-2` retornado sem alteração |
| UI OpenAPI | 200 |
| JSON OpenAPI | 200, OpenAPI 3.0.3, versão 2.1.0 e 7 paths |
| `GET /auth/session` sem token | 401 `UNAUTHENTICATED` com `requestId` |
| `POST /auth/refresh` sem CSRF | 403 `CSRF_INVALID` com `requestId` |
| `POST /auth/google` com token inválido | 401 `INVALID_GOOGLE_TOKEN` com `requestId` |

O enabler externo de ambiente foi resolvido em 2026-08-22. Não há bloqueio de
backend pendente para iniciar a Iteração 3.

## Evidência da Iteração 3

| Controle | Evidência em 2026-08-22 |
|---|---|
| Perfil seguro | leitura/edição restrita a nome e telefone; email, identidade, papel, pontos e grupo preservados; JSON sem CPF integral ou hashes |
| Membership atual | unicidade por usuário; `group_memberships` e `users.group_id` alterados na mesma transação |
| Isolamento | membros derivados exclusivamente do grupo do JWT; nenhuma rota aceita outro `groupId` para listar membros |
| Convites | 128 bits aleatórios, SHA-256 em repouso, sete dias, revogação, renovação e consumo único/idempotente |
| Autorização | somente `ADMIN` cria, lista, renova e revoga; qualquer identidade autenticada consome; `EVENT_MANAGER` não herda gestão |
| Concorrência | teste PostgreSQL real confirma um vencedor no consumo simultâneo e uma única membership |
| Paginação | grupos `name,id`; membros `name,user_id`; convites `created_at DESC,id DESC` |
| Migrations PostgreSQL | expand/backfill/contract, aplicação limpa, replay direto duplo, schema parcial, quatro runners e checksum drift verdes |
| CockroachDB | `v25.2.19` descartável: aplicação e replay sem reset/bypass; 13 migrations, 0 checksum ausente |
| Cobertura | mantida 70,6% ≥ 55%; mappers/repositories 93,9% ≥ 90% |
| Contrato | OpenAPI 3.0.3 `2.2.0`, manifesto operação→testes e guia `docs/profile-and-groups.md` |

Enabler preservado para as etapas finais: o frontend deve migrar sessão/perfil,
trocar o alias deprecated `POST /users/me/group` pelo `PATCH` canônico, consumir
paginação e implementar as telas administrativas de convite. Nenhum arquivo do
frontend foi alterado nesta iteração.

## Deploy e smokes remotos da Iteração 3

- Commit de implementação: `7ea0cfd`.
- Run verde: <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32602976830>.
- API: <https://ttwkfudhvvhuhp5yvsoydxggum0ictpg.lambda-url.sa-east-1.on.aws/v2>.
- OpenAPI: <https://dnjtechteam.github.io/dnj-game-api/develop/v2/>.

| Smoke em 2026-08-22 | Resultado |
|---|---|
| Migrations/checksums CockroachDB | etapa do workflow verde, sem reset ou bypass |
| `GET /healthcheck` | 200 `ok` |
| `GET /readiness` | 200 `ready` |
| UI OpenAPI | 200 |
| JSON OpenAPI | 200, OpenAPI 3.0.3, versão 2.2.0 e 16 paths |
| `GET /users/me` sem token | 401 `UNAUTHENTICATED` com `requestId` |
| `GET /groups` sem token | 401 `UNAUTHENTICATED` com `requestId` |
| `GET /groups/me` sem token | 401 `UNAUTHENTICATED` com `requestId` |
| `GET /groups/me/members` sem token | 401 `UNAUTHENTICATED` com `requestId` |
| `POST /groups/invites/consume` sem token | 401 `UNAUTHENTICATED` com `requestId` |
| `GET /admin/groups/1/invites` sem token | 401 `UNAUTHENTICATED` com `requestId` |

Não há bloqueio de backend pendente para iniciar a Iteração 4. A integração do
frontend com os contratos das Iterações 2 e 3 permanece deliberadamente como
enabler das etapas finais, conforme os handoffs, sem alteração antecipada do
repositório frontend.

## Evidência local da Iteração 4

| Controle | Evidência em 2026-08-24 |
|---|---|
| Instalação única | migrations e teste de upgrade confirmam `events=0` e nenhuma coluna `event_id` |
| Persistência | `spaces`, `activities`, `activity_manager_assignments` e `operation_audit`; UUID para recursos V2 e BIGINT somente na FK compatível com `users.id` legado |
| Configuração | kinds/status publicados, horários UTC, janela válida, pontos e cooldown não negativos, Space opcional e elegibilidade estrutural de Moment protegidos por constraints |
| Autorização | `ADMIN` global; `EVENT_MANAGER` somente com assignment persistido; `DEFAULT` sem operação; papel e assignment lidos sob lock no banco |
| Isolamento e segurança | ausência e fora de assignment usam o mesmo 404; UUIDs validados; assignment duplicado rejeitado; cliente não fornece papel, escopo ou pontos |
| Concorrência e idempotência | locks transacionais, update condicional e testes provam um vencedor com chaves distintas e um único efeito/audit em retries concorrentes da mesma chave |
| Auditoria | `activity.start` e `activity.pause` gravam ator, ação, entidade e estados anterior/final, sem PII, token, segredo ou corpo |
| Paginação | `GET /spaces` ordena por `name,id`, limita 20 e expõe headers mantendo o array exigido pelo frontend |
| PostgreSQL real | `make validate` verde: race, cobertura 71,9% ≥ 55%, mappers/repositories 94,1% ≥ 90%, HTTP real e migrations |
| CockroachDB | `v25.2.19` descartável: instalação limpa e replay pelo tracker; upgrade exato de `ba6a5dc` com dados preservados; 16 migrations, 0 checksum ausente, 4 tabelas da iteração e nenhum reset/bypass |
| Contrato | OpenAPI 3.0.3 `2.3.0`, três operações implementadas e manifesto operação→testes consistente |

O contrato administrativo mínimo foi aprovado e implementado em 2026-08-24.
Ele publica sob `/v2/admin` CRUD sem exclusão física para configuração de
Spaces/Activities, listagem de `EVENT_MANAGER`, transição restrita de papel e
assignments idempotentes. `ADMIN` continua impossível de conceder/remover pela
API, Activity nasce `draft`, somente `archived` é aceito como transição no PATCH
administrativo e start/pause permanecem nas operações gerenciais.

Enabler preservado para as etapas finais: o frontend deve integrar em conjunto
os contratos das Iterações 2–4, migrar `/api/v1/spaces` para `/v2/spaces` e
substituir os handlers antigos de staff/configuração pelas rotas publicadas.
Nenhum arquivo do frontend foi alterado.

## Evidência local do enabler administrativo da Iteração 4

| Controle | Evidência em 2026-08-24 |
|---|---|
| Autorização e banco como fonte | Todos os reads/writes administrativos revalidam ator e papel no banco; IDs de Space, Activity e User, onboarding, papel e assignment são consultados nas tabelas próprias. |
| Mass assignment | `ParseStrictRequest` e DTOs distintos rejeitam `eventId`, `status` no POST, `ADMIN`, start/pause simulados e qualquer campo fora do contrato. |
| Papel e assignments | Somente `DEFAULT ↔ EVENT_MANAGER`; `MANAGER_HAS_ASSIGNMENTS` bloqueia rebaixamento; PUT/DELETE são idempotentes e assignment exige onboarding completo. |
| Idempotência | `admin_operations` guarda fingerprint e resultado seguro original; retry não repete efeito/audit e reutilização cruzada retorna `IDEMPOTENCY_KEY_REUSED`. |
| Auditoria | Toda escrita bem-sucedida, inclusive no-op com chave nova, grava ator, ação, entidade e metadados mínimos sem PII, corpo, mapa, descrição, token ou segredo. |
| HTTP real entre camadas | Uma suíte atravessa middleware, handlers, services, repositories e PostgreSQL real nas 11 rotas, incluindo 401, 403, strict JSON e resultado original após alteração posterior. |
| Concorrência | Oito retries simultâneos do mesmo assignment produzem um único row, um único resultado idempotente e um único audit. |
| Cobertura do service | 91,4% (403/441 statements), gate permanente `make test-admin-cover-check` com mínimo 91,4%. |
| Cobertura da fatia entre camadas | 92,1% (608/660 statements) em service, handlers, mappers e métodos de repositories da fatia, bloqueada em 92,1%. |
| Migrations PostgreSQL | Clean install, replay direto duplo, upgrade preservando rows da Iteração 4, quatro runners, checksum e constraints verdes; três migrations adicionais expand/backfill/contract. |
| Migrations CockroachDB | `v25.2.19`: clean install e replay com 19 migrations/0 checksum ausente; upgrade exato desde `9e33526` preservou User, Space, Activity, assignment e audit, realizou backfill de `entity_reference` e repetiu sem reset ou bypass; `events=0` e `event_id=0`. |
| Enforcement no CI | Os workflows de PR, develop, release e production executam `make test-admin-cover-check`; tanto o service quanto a fatia integrada precisam manter no mínimo 90%. |
| Contrato | OpenAPI 3.0.3 `2.3.1`, 11 novas operações, exemplos e manifesto operação→testes consistentes. |

Enablers finais do frontend, sem alteração deste repositório: Iteração 2 exige
Google/sessão/refresh/onboarding/perfil V2; Iteração 3 exige membership, grupos,
paginação e convites; Iteração 4 exige `/v2/spaces`, `/v2/admin`, JWT de
identidade e um UUID de idempotência estável por intenção. Datas trafegam e são
persistidas como instantes UTC/RFC 3339 `Z`; o frontend deve convertê-las para o
fuso local atual somente na exibição e converter escolhas locais de volta para
UTC antes de enviar. Agenda, QR,
participações, runs, jogos, Moments e anúncios permanecem fora deste enabler.

## Deploy e smokes remotos da Iteração 4

- Commit de implementação: `80f5ca7`.
- Run verde: <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32727711442>.
- Revalidação do commit documental: run
  <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32728720693>, verde.
- Commit do enabler administrativo: `362ea1f`.
- Commit de normalização UTC e handoff do frontend: `0458c8b`.
- Run final do enabler: <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32745253801>,
  verde, incluindo race, gates de cobertura, migrations, deploy Lambda e Pages.
- API: <https://ttwkfudhvvhuhp5yvsoydxggum0ictpg.lambda-url.sa-east-1.on.aws/v2>.
- OpenAPI: <https://dnjtechteam.github.io/dnj-game-api/develop/v2/>.

| Smoke em 2026-08-24 | Resultado |
|---|---|
| Migrations/checksums CockroachDB | etapa do workflow verde, sem reset, edição de migration aplicada ou bypass de checksum |
| `GET /healthcheck` | 200 `ok`, com `X-Request-ID` |
| `GET /readiness` | 200 `ready`, com `X-Request-ID` |
| `GET /spaces?page=1` | 200 `[]`; `X-Page: 1`, `X-Limit: 20`, `X-Has-Next-Page: false` e `X-Request-ID` |
| `POST /manager/activities/{id}/start` sem token | 401 `UNAUTHENTICATED` com `requestId`, confirmando registro e proteção da rota sem enumerar Activity |
| `GET /admin/spaces?page=1` sem token | 401 `UNAUTHENTICATED` com `requestId`, sem expor dados administrativos |
| `GET /admin/staff?role=EVENT_MANAGER&page=1` sem token | 401 `UNAUTHENTICATED` com `requestId` |
| `POST /admin/spaces` sem token, JSON e chave válidos | 401 `UNAUTHENTICATED`; middleware bloqueou antes de qualquer mutação |
| `PUT /admin/activities/{id}/managers/{userId}` sem token | 401 `UNAUTHENTICATED`, sem enumerar Activity, User ou assignment |
| UI OpenAPI | 200 |
| JSON OpenAPI | 200, OpenAPI 3.0.3, versão 2.3.1 e 27 paths; inclui as 11 operações administrativas e a regra UTC/RFC 3339 `Z` com exibição local no cliente |

O ambiente de develop não possui Space/Activity administrativa publicada nem
credencial de gestor atribuída para um smoke mutante seguro. Sucesso,
idempotência, concorrência, isolamento e auditoria das transições são cobertos
por HTTP real e PostgreSQL real no gate/deploy; nenhuma configuração remota foi
inventada ou inserida fora de contrato para fabricar o smoke.

## Evidência local da Iteração 5

| Controle | Evidência em 2026-08-24 |
|---|---|
| Agenda | Contrato público `{items,generatedAt}`, DTO estrito, filtro por slug de Space, ordem determinística, limite completo 100 e home com todos os simultaneamente live + três próximas. |
| Relógio/estados | Clock injetável e testes para `15m+1ns`, 15m exatos, início exato draft/active, durante, pausa, término exato, depois e completed. |
| Conteúdo público | Envelope paginado, ordem `startsAt NULLS LAST,name,id`, filtros estritos, mesma visibilidade em lista/detalhe e 404 uniforme. |
| Favoritos | `user_favorites(user_id,activity_id)` único; `participant_operations` sem PII/corpos; PUT/DELETE 204, retry original, reutilização cruzada 409 e nenhuma auditoria privilegiada. |
| Banco como fonte | Usuário, onboarding, papel atual, Activity, visibilidade e favorito são relidos no PostgreSQL; role alterado não depende do JWT e usuário removido recebe 401. |
| Concorrência | Mesma chave produz um efeito/operação; chaves distintas não duplicam favorito; DELETE ausente e Activity arquivada depois do favorito permanecem seguros. |
| UTC | Respostas serializam `Z`; offsets administrativos preservam instante; testes e race são executados sob `TZ=UTC`. |
| HTTP real | Suíte atravessa middleware, handler, service, repository e PostgreSQL nas seis rotas, incluindo queries estritas e autorização. |
| Migrations | Três migrations expand/backfill/contract, clean install/replay e upgrade preservando dados da Iteração 4, sem `events`/`event_id`. |
| CockroachDB | `v25.2.19`: clean install e upgrade exato desde `ef20d63`, seguidos de replay sem reset/bypass; 22 migrations, 0 checksum ausente, User/Space/Activity preservados, 2 tabelas novas e 0 coluna `event_id`. |
| Cobertura | Services da Iteração 5: 97,4% (150/154); fatia integrada: 96,0% (316/329). Ambos possuem gate permanente de 90% em `make validate` e nos quatro workflows. |
| Contrato/handoff | OpenAPI 3.0.3 `2.4.0`, manifesto operação→testes e `docs/agenda-content.md` com arquivos atuais, rotas, grafos, orçamento, backlog e perfis futuros de carga. |
| Validação | `make validate` verde: Wire, build, vet, race sob `TZ=UTC`, cobertura agregada 93,1%, código mantido 78,9%, gates especializados, PostgreSQL real, migrations e OpenAPI. |

O frontend permaneceu somente leitura. O roadmap e todos os artefatos obrigatórios
da Iteração 10 continuam preservados na seção “Entrega obrigatória da Iteração
10 para o frontend”.

## Deploy e smokes remotos da Iteração 5

- Commit de implementação: `ddcf50a`.
- Run verde: <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32754142688>,
  incluindo race sob `TZ=UTC`, gates de cobertura, migrations, deploy Lambda e Pages.
- Commit documental: `dcd33d8`; revalidação final verde em
  <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32754694548>.
- API: <https://ttwkfudhvvhuhp5yvsoydxggum0ictpg.lambda-url.sa-east-1.on.aws/v2>.
- OpenAPI: <https://dnjtechteam.github.io/dnj-game-api/develop/v2/>.

| Smoke em 2026-08-24 | Resultado |
|---|---|
| `GET /schedule?view=home` | 200, `{items:[],generatedAt}` e `generatedAt` UTC serializado com `Z` |
| `GET /activities?page=1` | 200, `{data:[],pagination}` com página, limite e `hasNextPage` determinísticos |
| `GET /schedule?view=full` | 400 `INVALID_REQUEST`, confirmando allowlist de `view` |
| `GET /activities/{UUID inexistente}` | 404 `NOT_FOUND`, sem distinguir inexistente de invisível |
| `GET /users/me/favorites?page=1` sem autenticação | 401 `UNAUTHENTICATED` |
| `PUT /users/me/favorites/{activityId}` sem autenticação, chave UUID válida | 401 `UNAUTHENTICATED`, sem efeito nem enumeração |
| `DELETE /users/me/favorites/{activityId}` sem autenticação, chave UUID válida | 401 `UNAUTHENTICATED`, sem efeito nem enumeração |
| UI OpenAPI | 200 |
| JSON OpenAPI | 200, OpenAPI 3.0.3, versão 2.4.0, servidor de develop e os cinco paths que expõem as seis operações da Iteração 5 |

O ambiente de develop continua sem Activities publicadas nem credencial de
participante criada artificialmente. Os resultados autenticados, mutações,
idempotência, concorrência, isolamento e ausência de auditoria indevida são
cobertos pelo HTTP/PostgreSQL real e pelos gates do deploy, sem adicionar seed
de produção para fabricar smoke mutante.

## Evidência local da Iteração 6

| Controle | Evidência em 2026-08-24 |
|---|---|
| Catálogo | Game é apenas projeção de Activity competitive; mesma visibilidade da Iteração 5, exclusão de draft/archived/invisível, envelope paginado e ordem `startsAt NULLS LAST,name,id`. |
| Runs | Estados draft/active/paused/results/completed/cancelled, transições condicionais, uma run aberta por Activity, relógio injetável e timestamps UTC. |
| QR/participação | Um QR ativo por run draft, 45 minutos, HMAC/hash em repouso, rotação imediata, scan único sem pontos/audit e 409/410 sem enumeração. |
| Autorização | Usuário/papel/onboarding e assignment relidos no banco; `ADMIN` global, `EVENT_MANAGER` atribuído e `DEFAULT` participante; recursos fora do escopo não enumeráveis. |
| Resultados/pontos | Snapshot 50/30/20/10; conjunto completo; locks estáveis; ledger e saldo na mesma transação; unicidade impede prêmio duplicado; cancelamento não pontua. |
| Ledger | `point_entries` append-only por trigger; FKs `RESTRICT`; backfill `legacy_balance` preserva saldos anteriores; auditoria testável compara `SUM(delta)` com `users.points`. |
| Ranking | Usuários DEFAULT com onboarding, grupos inclusive vazios, ordem determinística, posição ordinal, overview 30/10/50 e nenhuma PII/reason interno. |
| HTTP real | `TestIteration6HTTP_MiddlewareHandlerServiceRepositoryAndDatabase` atravessa middleware→handler→service→repository→PostgreSQL nas 16 rotas e nos fluxos completo/cancelado. |
| Concorrência/rollback | Testes cobrem mesma chave, scans simultâneos, criação concorrente, operações incompatíveis, finalização única e rollback integral. |
| Cobertura | Services: 90,1% (549/609); fatia integrada: 90,4% (923/1021), gates permanentes mínimos de 90% em `make validate` e quatro workflows. |
| Gate agregado | `make validate` verde: Wire, build, vet, race sob `TZ=UTC`, cobertura, migrations PostgreSQL reais, testes HTTP reais, OpenAPI e todos os gates permanentes das Iterações 1–6. |
| Migrations PostgreSQL | PostgreSQL 16: instalação limpa, upgrade exato desde `f6b1cb4` e replay direto verdes, com 25 checksums íntegros. User, Space, Activity, assignment, favorite e audit anteriores permaneceram intactos; o saldo legado de 37 pontos foi conciliado no ledger e o trigger append-only rejeitou UPDATE. |
| Migrations CockroachDB | CockroachDB v25.2.19: instalação limpa, upgrade exato desde `f6b1cb4` e replay sem reset verdes, também com 25 checksums íntegros, os mesmos dados preservados e as mesmas verificações de ledger/append-only. |
| Esquema | Seis tabelas da Iteração 6 presentes; nenhuma tabela `games`/`events`, coluna `event_id` ou cascata destrutiva introduzida. |
| Contrato/handoff | OpenAPI 3.0.3 `2.5.0`, 16 operações, manifesto operação→testes, `docs/games-runs-scoring.md` e `docs/game-frontend-handoff.md`. |
| Frontend | Clone consultado somente em leitura; nenhum arquivo, commit ou push produzido. Handoff registra rotas, grafos, orçamento, polling e perfis futuros de carga. |

## Evidência local da Iteração 7

| Controle | Evidência em 2026-08-24 |
|---|---|
| Domínio | Moment é a entidade persistida; galeria é somente consulta. Sem tabelas `gallery`/`events`, sem `event_id`. `media_assets` guarda metadados; conteúdo permanece em bucket privado S3/MinIO. |
| Upload | `POST /media/upload-intents` aceita só `contentType/bytes/checksumSha256`; JPEG/PNG até 10 MiB; staging key aleatória sem PII; intenção e URL assinada expiram em 10 minutos; URL nunca persistida; retry reconstrói a resposta a partir de metadados seguros. |
| Confirmação/sanitização | `POST /media/{id}/complete` sem corpo; valida ownership/estado/expiração; HEAD real no provider; download limitado; valida magic bytes e configuração real (até 20 megapixels); reencoda removendo EXIF/GPS; grava versão final distinta da staging key; claim/lease durável serializa confirmações concorrentes sem transação de banco aberta durante chamadas ao S3; 409/410/503 conforme especificado; asset alheio/inexistente é sempre o mesmo 404. |
| Moments | `POST /moments` deriva origem, pontos, ownership e timestamps no servidor; free nunca pontua; challenge público concede o snapshot de `Activity.momentPoints`; unicidade de asset e de Participation impede duplicidade mesmo sob concorrência. |
| Galeria | `GET /moments` aceita só `scope` (`feed|mine|group`) e `cursor` opaco assinado (HMAC, sem PII); paginação de 20 por `capturedAt DESC,id DESC`; `mine` preserva histórico completo; `feed`/`group` só público+aprovado+disponível de autores elegíveis; mudança de grupo reflete imediatamente. |
| Likes | `POST /moments/{id}/likes` alterna sem corpo, serializado por `(momentId,userId)`, idempotente, sem pontos nem `operation_audit`. |
| Moderação corretiva | `GET/POST /admin/moments/...` exigem ADMIN confirmado no banco; `deny_points`/`delete_photo` revertem pontos atomicamente, preservam a foto/histórico, geram `operation_audit` mínimo sem imagem/URL/PII; decisão já aplicada com chave nova retorna o estado terminal sem novo efeito. |
| Retenção/worker | `DNJ_MEDIA_RETENTION_ANCHOR_AT` obrigatório (RFC 3339 com offset, normalizado para UTC); `retentionDueAt = anchor + 90 dias`, nunca menor que 90 dias desde a criação; ausência/config inválida bloqueia novas intenções com 503 sem afetar healthcheck/catálogo/auth; worker interno (`cmd/media-worker`) expira uploads pendentes, reclama jobs de limpeza com lease persistido e retry com backoff+jitter, sem endpoint administrativo de disparo. |
| Idempotência | Toda escrita HTTP exige `Idempotency-Key` UUID; chave+intenção iguais repetem o resultado original; reuso para operação/recurso/payload diferente retorna 409 `IDEMPOTENCY_KEY_REUSED`; unicidade unificada (`idempotency_operations`) atravessa também as escritas de participante, gestor e admin das Iterações 1–6 (não é só backfill de dados — `admin_operation_repository.go`, `favorite_repository.go` e `game_repository.go` gravam nela). |
| Bug corrigido | `MediaRepository.Metrics` quebrava sempre que a fila de limpeza estava vazia (`MIN(due_at)` retornando `NULL` escaneado para `*time.Time`) e sempre que chamava as duas primeiras agregações (parâmetro extra passado a uma query sem placeholder); ambos corrigidos e cobertos por teste de integração real com Postgres antes da publicação. |
| HTTP real | `TestMediaMomentsHTTP_MiddlewareHandlerServiceRepositoryAndDatabase` atravessa middleware→handler→service→repository→PostgreSQL. |
| Concorrência | Testes cobrem confirmações concorrentes com a mesma chave (espera e replay), duas confirmações do mesmo asset, dois Moments para o mesmo asset/Participation, likes simultâneos, moderações concorrentes, claim/lease abandonado e retomado por outro worker, e falha entre S3 e banco. |
| Cobertura | Services da Iteração 7: 93,1% (650/698); fatia integrada (handlers/services/mappers/repositories): 92,0% (968/1052). Gate permanente `db/mappers`+`db/repositories`: 90,6% (1046/1155); cobertura mantida: 83,0% (4575/5515). Gates administrativos (91,4%/92,2%) e da Iteração 5 (97,4%/97,3%) preservados após a unificação de idempotência. |
| Gate agregado | `make validate` verde: Wire, build, vet, race sob `TZ=UTC`, cobertura (geral + todos os gates permanentes 1–7), migrations PostgreSQL e CockroachDB reais, testes HTTP reais, OpenAPI. |
| Migrations PostgreSQL/CockroachDB | Três migrations novas (`expand_media_moments_v2`, `backfill_global_idempotency_registry`, `contract_media_moments_v2`), sem editar as migrations já aplicadas; instalação limpa e upgrade exato verdes em ambos os bancos; dados das Iterações 1–6 preservados. |
| MinIO/S3 | `docker-compose.yml` provisiona bucket privado com versionamento habilitado e lifecycle de defesa em profundidade (`staging/` expira em 2 dias); version ID confirmado persistido internamente para impedir que um PUT tardio substitua o conteúdo do Moment. |
| Contrato/handoff | OpenAPI 3.0.3 `2.6.0` com as 7 operações novas; manifesto operação→testes atualizado; `docs/game-frontend-handoff.md` atualizado. |
| Frontend | Clone consultado somente em leitura para descoberta de contrato; nenhum arquivo, commit ou push produzido no repositório do frontend. |

## Publicação da Iteração 6 em develop

- Commit de implementação: `b0775c5` (`feat: implement iteration 6 games and rankings`).
- Workflow `develop.yml`: run `32778480015` verde em 5m17s, incluindo race,
  coberturas, contrato, migrations no banco, imagem, Lambda e GitHub Pages.
- Migrations aplicadas sem reset: `expand_iteration6_games_runs_scoring`,
  `backfill_iteration6_games_runs_scoring` e
  `contract_iteration6_games_runs_scoring`, todas com checksum registrado.
- Smokes somente leitura após o deploy:

| Operação publicada | Resultado |
|---|---|
| `GET /healthcheck` | 200, serviço `ok` |
| `GET /readiness` | 200, banco `ready` |
| `GET /games?page=1` | 200, `data=[]`, paginação estável |
| `GET /rankings?scope=individual&page=1` | 200, `data=[]`, `generatedAt` UTC com `Z` |
| `GET /rankings?scope=groups&page=1` | 200, `data=[]`, `generatedAt` UTC com `Z` |
| `GET /games/00000000-0000-4000-8000-000000000000` | 404 `NOT_FOUND` sem enumeração |
| Overview, run/participação atual e overview gerencial sem autenticação | 401 `UNAUTHENTICATED` |
| UI OpenAPI | 200 |
| JSON OpenAPI | 200, OpenAPI 3.0.3, versão 2.5.0, servidor de develop e 16 paths da Iteração 6 |

O ambiente permaneceu sem Activities, usuários, pontos, QR ou runs artificiais;
nenhum smoke mutante foi executado. As Iterações 7–10 e os artefatos
obrigatórios do handoff final permanecem preservados no roadmap.
