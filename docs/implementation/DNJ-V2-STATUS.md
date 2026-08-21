# DNJ V2 — Status executivo e matriz de atendimento

Atualizado em: 2026-08-21

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
| 1 | Enablers e trilho | `GET /healthcheck`, `GET /readiness` | `schema_migrations.checksum` | Concluída localmente; aguardando deploy |
| 2 | Identidade e Google | troca Google, refresh/logout, sessão atual, completar perfil | usuários, identidades, refresh sessions, challenges | Próxima |
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
| Publicação documental | script validado com V1 em `/develop/` e V2 em `/develop/v2/` |

Após o push, registrar aqui o commit, URLs publicadas, run do GitHub Actions e
resultado dos smokes. Se um segredo/ambiente impedir o smoke, registrar a
evidência do bloqueio e transformar a primeira ação da próxima sessão em
enabler operacional.
