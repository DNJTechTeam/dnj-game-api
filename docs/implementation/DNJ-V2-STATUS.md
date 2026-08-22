# DNJ V2 — Status executivo e matriz de atendimento

Atualizado em: 2026-08-22

Branch de execução: `develop`

Contrato canônico: `/v2`
Legado: `/v1` preservado até que exista evidência de migração do consumidor

Este arquivo é a fonte de verdade entre sessões. Nenhum endpoint é marcado
como concluído antes de possuir implementação, contrato OpenAPI, teste
automatizado e evidência no ambiente `develop`.

## Decisões fechadas

- O backend é o único repositório alterado; o frontend recebe um handoff pronto.
- Filas e comentários estão fora do escopo da V2 atual.
- Login Google cria perfil incompleto quando CPF, telefone ou grupo faltarem;
  recursos protegidos exigirão conclusão explícita do onboarding.
- Dados existentes em `develop` são preservados. É proibido resetar o banco
  como estratégia de rollout.
- Fotos permanecem por 90 dias após o término do evento; limpeza será assíncrona
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
| 2 | Identidade e Google | Google, refresh/logout, sessão atual, completar perfil | usuários, identidades, refresh sessions | Backend publicado; smoke Google bloqueado pela variável externa ausente |
| 3 | Perfil e grupos | perfil atual, grupo atual, membros, convites/códigos | perfis, grupos, memberships, invites | Pendente |
| 4 | Configuração do evento | leitura/edição do evento, regras, staff e permissões | eventos, configurações, roles | Pendente |
| 5 | Agenda e conteúdo | agenda, atividades, detalhes, favoritos | agenda, atividades, favoritos | Pendente |
| 6 | Jogos e ranking | catálogo, partidas/tentativas, placar e ranking | jogos, tentativas, resultados, leaderboard | Pendente |
| 7 | Mídia | upload assinado, confirmação, galeria, moderação, retenção | assets, uploads, moderação, jobs de retenção | Pendente |
| 8 | Notificações | preferências, listagem, leitura e envio administrativo | notificações, preferências, deliveries | Pendente |
| 9 | Operação e carga | observabilidade final, segurança, soak/spike/stress em develop | métricas e relatórios, sem novo domínio | Pendente |
| 10 | Handoff final | OpenAPI publicado, guia, exemplos e checklist de release | documentação versionada | Pendente |

Os nomes e formatos exatos das operações das iterações 2–8 serão fechados a
partir das regras do frontend e dos handoffs antes de entrar no OpenAPI. Uma
lacuna encontrada vira um enabler documentado e testado; não vira suposição
silenciosa.

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

Enablers explícitos fora deste backend:

- O ambiente precisa fornecer `GOOGLE_CLIENT_ID` e `DOCUMENT_HMAC_SECRET`.
- O frontend ainda precisa integrar o SDK Google e consumir este contrato; o
  repositório frontend não foi alterado.
- A remoção de `users.document` é a etapa contract posterior à migração total
  dos consumidores V1; até lá o campo legado é preservado sem perda de dados.

## Deploy e smokes remotos da Iteração 2

- Commit: `6cc61ac`.
- Run verde: <https://github.com/DNJTechTeam/dnj-game-api/actions/runs/32599410466>.
- API: <https://ttwkfudhvvhuhp5yvsoydxggum0ictpg.lambda-url.sa-east-1.on.aws/v2>.
- OpenAPI: <https://dnjtechteam.github.io/dnj-game-api/develop/v2/>.

| Smoke em 2026-08-22 | Resultado |
|---|---|
| Migrations/checksums CockroachDB | etapa do workflow verde |
| `GET /healthcheck` | 200 `ok` |
| `GET /readiness` | 200 `ready` |
| `X-Request-ID` gerado | 200 com id seguro retornado |
| `X-Request-ID` preservado | `smoke-develop-6cc61ac` retornado sem alteração |
| UI OpenAPI | 200 |
| JSON OpenAPI | 200, OpenAPI 3.0.3, versão 2.1.0 e 7 paths |
| `GET /auth/session` sem token | 401 `UNAUTHENTICATED` com `requestId` |
| `POST /auth/refresh` sem CSRF | 403 `CSRF_INVALID` com `requestId` |
| `POST /auth/google` com token inválido | 500 `INTERNAL_ERROR`: `GOOGLE_CLIENT_ID` vazio no runtime |

Bloqueio externo objetivo: adicionar `GOOGLE_CLIENT_ID` à configuração já
existente da Lambda `develop`, com o OAuth client id usado pelo frontend, sem
substituir as demais variáveis. Depois, repetir o último smoke; um token malformado
deve retornar 401 `INVALID_GOOGLE_TOKEN`. Confirmar também a presença de
`DOCUMENT_HMAC_SECRET` antes do smoke autenticado de conclusão de onboarding.
