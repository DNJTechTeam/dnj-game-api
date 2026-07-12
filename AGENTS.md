# AGENTS.md — Nucleus API

Guia para agentes de IA (Codex, Claude e afins) que trabalham neste repositório.
O conteúdo arquitetural completo está em `CLAUDE.md` e na pasta `docs/` — este
arquivo é o ponto de entrada e aponta para eles.

## O que é este projeto

`nucleus-api` é um **template base** de API REST em Go (Gin + GORM + Wire),
com autenticação JWT de identidade (sem multi-tenant) e um runner que roda como
servidor HTTP local ou como AWS Lambda. Traz um CRUD de `Task` como exemplo de
referência.

## Antes de começar

- **Projeto recém-criado a partir do template?** Rode a renomeação de
  placeholders descrita em `README.md` (seção "Inicializando o projeto").
- **Codex**: use a skill de projeto em
  `.codex/skills/nucleus-api-project/SKILL.md`.
- **Claude Code**: use as skills em `.claude/skills/` (`init-project`,
  `new-resource`).
- Leia `CLAUDE.md` para arquitetura e convenções.

## Tarefas comuns

| Tarefa | Onde olhar |
|--------|-----------|
| Adicionar um novo recurso/CRUD | `docs/new-resource-guide.md` (passo a passo; step 17 para paginação) |
| Escrever testes de service | `docs/TESTING.md` |
| Entender as camadas e o fluxo | `docs/architecture.md` |
| Autenticação, JWT, cookies/header | `docs/auth-and-tokens.md` |
| Escrever migrations idempotentes | `docs/migrations.md` |
| Processo de release (versioning, CI/CD) | `docs/RELEASE.md` |

Ao criar um recurso novo, **copie o padrão do recurso `Task`** — ele existe em
todas as camadas (`internal/domain/task`, `internal/infrastructure/db/.../task*`,
`internal/app/.../task*`, `internal/presentation/api/handlers/task_handler.go`,
`internal/presentation/api/routers/task_router.go`).

## Regras

- Após mexer em providers Wire: rode `go generate ./cmd/api/di`.
- Após criar/alterar uma interface mockada: rode `mockery`.
- Migrations: toda função `Up` deve ser idempotente (ver `docs/migrations.md`).
- Todo novo recurso deve ter testes seguindo `docs/TESTING.md`.
- Sempre rode `make test-cover-check` antes de concluir — exige Docker disponível (Testcontainers).

## Validação

```bash
go build ./...
go generate ./cmd/api/di
make vet
make test-cover-check
make test-migrations
```
