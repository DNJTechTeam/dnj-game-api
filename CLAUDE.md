# DNJ Game API — Guia de Arquitetura e Geração de Código

`dnj-game-api` é uma API REST em Go, construída a partir do template
`nucleus-api`. Este documento orienta a geração de código mantendo
consistência com a arquitetura existente. Vale tanto para o Claude quanto
para o Codex (ver também `AGENTS.md`, que aponta para os mesmos guias).

---

## Visão geral

API REST em Go (Gin) com:

- **Banco**: PostgreSQL via GORM.
- **DI**: Google Wire.
- **Auth**: JWT **somente de identidade** (sem tenant/customer). O token
  identifica o usuário e nada mais — um modelo de permissão mais rico (papéis,
  escopos) deve ser adicionado depois, sobre essa base.
- **Runner**: roda como servidor HTTP local **ou** como AWS Lambda, conforme a
  env `SERVER_RUNNER`.
- **Recurso de exemplo**: um CRUD de `Task` (`internal/.../task*`) — copie esse
  padrão ao criar novos recursos.

---

## Estrutura de diretórios

```
cmd/api/
├── main.go
└── di/
    ├── wire.go             # Configuração Wire (adicionar novos providers aqui)
    └── wire_gen.go         # Gerado: go generate ./cmd/api/di
cmd/migrate/
└── main.go                 # Runner de migrations standalone

internal/
├── app/                    # Lógica de aplicação (casos de uso)
│   ├── errors/             # Error, FieldError, SimpleError, sentinelas
│   ├── interfaces/         # Interfaces dos services
│   ├── mappers/            # Entity → DTO
│   ├── messages/           # DTOs de request/response
│   ├── services/           # Implementação dos services + testes
│   └── validators/         # Validadores reutilizáveis
│
├── domain/                 # Entidades e interfaces de domínio
│   ├── common/interfaces/  # TransactionManager, ModelInterface
│   ├── task/               # Recurso de exemplo (entities + interfaces)
│   └── user/               # Identidade (entities + interfaces)
│
├── infrastructure/
│   ├── api/                # runner.go (local/lambda), cookies.go, auth/, middlewares/
│   ├── common/             # helpers de env e contexto
│   ├── db/                 # GORM, models, migrations, repositories, mappers
│   ├── di/                 # Providers Wire (api, db, repositories, services, test)
│   └── email/              # Cliente de e-mail transacional
│
├── mocks/                  # Mocks gerados (mockery) — não editar à mão
└── presentation/api/
    ├── handlers/           # Handlers Gin (um por recurso) + functions.go
    └── routers/            # Registro de rotas
```

---

## Camadas e responsabilidades

1. **Domain** (`internal/domain/`): entidades puras + interfaces de repositório.
2. **Infrastructure/DB**: models GORM, mappers Model↔Entity, repositories
   (estendem `BaseRepository[T]`).
3. **Application/Services**: regras de negócio. Recebem DTOs, retornam DTOs.
   Usam `BaseService.WithTransaction` para operações atômicas.
4. **Application/Messages e Mappers**: DTOs (JSON camelCase; IDs como
   `Uint64String`) e conversão Entity→DTO.
5. **Presentation/Handlers**: parse de request (`ParseRequest`), chamada ao
   service, resposta via helpers (`ResponseSuccess`, `ResponseBadRequest`,
   `ResponseNotFound`, ...).
6. **Presentation/Routers**: registro de rotas. Rotas protegidas usam
   `r.authProtected()`.

---

## Autenticação (resumo)

- `AuthenticationMiddleware` valida o JWT de identidade e coloca o `userId` no
  contexto. O token é lido do header `Authorization` **e**, como fallback, do
  cookie `identity_token` — ambos funcionam.
- Nos services, use `common.ExtractUserIdFromContext(ctx)` para obter o usuário.
- Detalhes completos em `docs/auth-and-tokens.md`.

---

## Tratamento de erros

- **Erro de validação**: `errors.NewError("mensagem", []*errors.FieldError{...})`.
- **Erro simples**: `errors.NewSimpleError("mensagem")`.
- **Não encontrado**: retornar `errors.ErrNotFound` (sentinela); nos handlers,
  testar com `errors.Is(err, appErrors.ErrNotFound)`.
- **Erro interno**: `errors.InternalError`.

---

## Como adicionar um novo recurso

O passo a passo completo está em **`docs/new-resource-guide.md`** — use o
recurso `Task` como referência viva. Resumo do fluxo:

1. Domain: entidade + interface de repositório.
2. Infra/DB: model GORM, mapper, repository.
3. Migration **idempotente** (`docs/migrations.md`).
4. App: DTOs, interface do service, service, mapper Entity→DTO.
5. Presentation: handler + router.
6. Wire: providers + `wire.Struct` do handler; rodar `go generate ./cmd/api/di`.
7. Mocks: `mockery` (após atualizar `.mockery.yml`).
8. Testes no pacote do service seguindo `docs/TESTING.md`.

Outros guias: `docs/architecture.md`, `docs/auth-and-tokens.md`,
`docs/migrations.md`. Para endpoints de lista com paginação, ver step 17 em
`docs/new-resource-guide.md`.

---

## Testes

- **Framework**: `testing` + `testify` + Testcontainers (PostgreSQL real, sem mocks de DB).
- **Padrão**: Given/When/Then com comentários obrigatórios.
- **Setup**: `setupXxxTest(t)` por serviço — chama `DefaultSetup`, trunca tabelas, registra `t.Cleanup`.
- **Contexto autenticado**: `TestSuite.ContextWithUser(userID)` (espelha o que `AuthenticationMiddleware` injeta).
- **Seed**: funções `seedXxx(t, ...)` em `test_helpers_test.go`.
- **Gate de cobertura**: ≥ 90% em `db/mappers` e `db/repositories`, verificado em CI.

Guia completo: `docs/TESTING.md`.

---

## Release

Adota GitFlow com Semantic Versioning (`vMAJOR.MINOR.PATCH`):

| Branch | Invariante |
|--------|-----------|
| `develop` | Sempre **releasable** |
| `release/X.Y.Z` | Apenas bug-fixes; sem features novas |
| `main` | Sempre **deployable** — está em produção |

Quatro workflows de CI/CD:
- `pr.yml` — vet + test-cover-check + migrations-idempotency em PRs
- `develop.yml` — deploy para develop em push
- `release.yml` — valida semver + cria PR → main automaticamente
- `production.yml` — deploy produção + cria tag `vX.Y.Z` + GitHub Release

Guia completo: `docs/RELEASE.md`.

---

## Convenções

- Pacotes: singular, minúsculo. Handlers/services: `{Recurso}Handler` /
  `{Recurso}Service`. DTOs: sufixos `RequestDTO`, `ResponseDTO`.
- Sempre passar `ctx` como primeiro parâmetro em chamadas que acessem o banco.
- Repositórios usam `r.getDB(ctx)` (suporte a transação).
- Transações: `s.WithTransaction(ctx, func(tCtx) error { ... })` no service.
- IDs externos: `messages.Uint64String` (serializa como string no JSON).

---

## Comandos

```bash
docker compose up -d db          # Postgres local
make run                         # sobe a API (porta SERVER_PORT)
make migrate                     # roda migrations standalone (carrega .env)
make vet                         # go vet ./...
make test                        # todos os testes (Testcontainers)
make test-cover-check            # testes + gate de cobertura ≥ 90%
make test-services               # apenas services
make test-migrations             # idempotência de migrations
go generate ./cmd/api/di         # regenera o Wire
mockery                          # regenera os mocks
```

---

## Checklist para nova feature

- [ ] Entidade de domínio + interface de repositório
- [ ] Model GORM + mapper + repository
- [ ] Migration idempotente registrada
- [ ] DTOs + interface do service + service + mapper Entity→DTO
- [ ] Handler + rota registrada (`authProtected` quando exigir login)
- [ ] Wire atualizado e `go generate ./cmd/api/di` executado
- [ ] Mocks regenerados (se criou interface nova)
- [ ] Testes seguindo `docs/TESTING.md` passando (`make test-cover-check`)
