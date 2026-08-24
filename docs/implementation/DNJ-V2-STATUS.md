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
| 5 | Agenda e conteúdo | agenda, atividades, detalhes, favoritos | agenda, atividades, favoritos | Pendente |
| 6 | Jogos e ranking | catálogo, partidas/tentativas, placar e ranking | jogos, tentativas, resultados, leaderboard | Pendente |
| 7 | Mídia | upload assinado, confirmação, galeria, moderação, retenção | assets, uploads, moderação, jobs de retenção | Pendente |
| 8 | Notificações | preferências, listagem, leitura e envio administrativo | notificações, preferências, deliveries | Pendente |
| 9 | Operação e carga | observabilidade final, segurança, soak/spike/stress em develop | métricas e relatórios, sem novo domínio | Pendente |
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
| Cobertura do service | 91,4% (403/441 statements), gate permanente `make test-admin-cover-check` com mínimo 90%. |
| Cobertura da fatia entre camadas | 92,1% (608/660 statements) em service, handlers, mappers e métodos de repositories da fatia, também bloqueada em 90%. |
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
