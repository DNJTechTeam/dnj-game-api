# Processo de Release — Nucleus API

## Princípios

| Branch | Invariante |
|--------|-----------|
| `develop` | Sempre **releasable** — qualquer commit deve poder virar uma release |
| `release/X.Y.Z` | Apenas bug-fixes de estabilização; nenhuma feature nova |
| `main` | Sempre **deployable** — representa o que está em produção |

---

## Versionamento Semântico

Use [Semantic Versioning](https://semver.org/) (`MAJOR.MINOR.PATCH`):

| Tipo de mudança | O que incrementar |
|-----------------|------------------|
| Breaking change (incompatível com a API pública) | `MAJOR` |
| Nova funcionalidade compatível | `MINOR` |
| Bug fix | `PATCH` |

---

## Fluxo Completo

```mermaid
gitGraph
   commit id: "feat: A"
   branch develop
   checkout develop
   commit id: "feat: B"
   branch release-1.2.0
   checkout release-1.2.0
   commit id: "fix: bug de QA"
   checkout main
   merge release-1.2.0 tag: "v1.2.0"
   checkout develop
   commit id: "fix: cherry-pick"
   commit id: "feat: C"
```

---

## Esteira de CI/CD

```mermaid
flowchart TD
    PR[PR para develop] -->|pr.yml| V[vet + test-cover-check\nmigrations-idempotency]
    V -->|aprovado e mergeado| D

    D[push develop] -->|develop.yml| D1[vet + test-cover-check]
    D1 --> D2[go run cmd/migrate/main.go\nDB credentials via secrets]
    D2 --> D3[docker build → ECR develop]
    D3 --> D4[Lambda update develop]
    D4 --> D5[publica Swagger UI\nGitHub Pages /develop]

    REL[git push release/X.Y.Z] -->|release.yml| R1[valida semver do nome]
    R1 --> R2[vet + test-cover-check\nmigrations-idempotency]
    R2 --> R3[cria PR → main]

    MAIN[merge para main] -->|production.yml| P1[vet + test-cover-check]
    P1 --> P2[go run cmd/migrate/main.go\nDB credentials via secrets]
    P2 --> P3[docker build → ECR latest]
    P3 --> P4[Lambda update production]
    P4 --> P5[publica Swagger UI\nGitHub Pages /production]
    P5 --> P6[cria tag vX.Y.Z\nGitHub Release]
```

---

## Passo a Passo

### 1. Verificar que develop está pronta

```bash
git checkout develop
git pull origin develop
```

Confirme que todos os PRs previstos para a versão foram mergeados e o ambiente develop está passando nos checks.

### 2. Criar a branch de release

```bash
git checkout -b release/X.Y.Z develop
git push origin release/X.Y.Z
```

O workflow `release.yml` dispara automaticamente e:
1. Valida o formato `release/X.Y.Z` (semver)
2. Roda: vet + test-cover-check + migrations-idempotency
3. Cria um **PR** de `release/X.Y.Z` → `main` com changelog automático

### 3. Estabilização (opcional)

Bug fixes encontrados em QA vão **direto na release branch**:

```bash
git checkout release/X.Y.Z
# ... corrigir bug ...
git commit -m "fix: corrige bug encontrado em QA"
git push origin release/X.Y.Z
# cherry-pick para develop também
git checkout develop
git cherry-pick <commit-hash>
git push origin develop
```

Cada push em `release/**` re-executa o workflow de validação.

### 4. Fechar a release

1. Abra o PR no GitHub
2. Certifique que todos os checks passaram
3. Solicite aprovação de pelo menos 1 reviewer
4. Mergee para `main`

O `production.yml` dispara e:
- Roda migrations contra o banco de produção (via secrets)
- Builda e publica imagem no ECR com tag `latest`
- Atualiza a Lambda de produção
- Cria tag `vX.Y.Z` e GitHub Release com changelog

### 5. Confirmar o deploy

```bash
git fetch --tags
git tag --sort=-version:refname | head -5
```

Confirme também que o workflow `production.yml` terminou com sucesso e que o GitHub Release `vX.Y.Z` foi criado.

### 6. Excluir a branch de release

Depois que o PR foi mergeado, o deploy de produção passou e a tag/GitHub Release foram criados, a branch `release/X.Y.Z` pode ser excluída:

```bash
git push origin --delete release/X.Y.Z
git branch -d release/X.Y.Z
```

Não exclua a branch antes de confirmar a pipeline e a tag. Se o deploy ou a criação da release falhar, mantenha a branch para correções rápidas ou novo rerun controlado.

---

## Ordem de operações na pipeline

```mermaid
sequenceDiagram
    participant CI as GitHub Actions
    participant DB as PostgreSQL
    participant ECR as Amazon ECR
    participant LB as AWS Lambda

    CI->>CI: vet + test-cover-check
    CI->>DB: go run cmd/migrate/main.go
    DB-->>CI: migrations aplicadas (idempotente)
    CI->>ECR: docker build + push
    CI->>LB: update-function-code
    LB-->>CI: ok
    Note over CI,LB: Lambda serve requests com schema e código novos
    CI->>CI: publica Swagger UI (docs/openapi) no GitHub Pages
```

A migration roda **antes** do deploy do Lambda: schema atualizado primeiro, código novo depois.
A publicação do Swagger UI roda **depois** do Lambda: sempre reflete o código já em produção/develop.

---

## Rollback

### Opção 1: Re-deploy da imagem anterior via Lambda Console

1. AWS Lambda Console → função de produção
2. **Image** → **Deploy new image**
3. Selecione a imagem ECR com a tag anterior (ex: `v1.1.0`)

### Opção 2: Reverter via hotfix

```bash
git checkout -b hotfix/X.Y.Z main
# ...fix...
git push origin hotfix/X.Y.Z
# Abra PR direto para main, sem passar por release flow
```

> Hotfixes críticos podem ir diretamente para `main` via PR sem criar branch `release/`. Use com parcimônia.

---

## Migrations na Pipeline

As migrations rodam via `go run cmd/migrate/main.go` com credenciais de DB dos GitHub Secrets do environment correspondente.

**Por que é seguro rodar duas vezes:**
- O registry é idempotente: verifica `schema_migrations` antes de cada migration
- O guardrail `TestMigrations_AreIdempotent` em CI garante que toda migration é idempotente antes de chegar ao banco

**Localmente:**
```bash
make migrate   # roda migrations (carrega .env automaticamente)
```

---

## Ambientes

| Ambiente | Branch | Trigger | Migrations | Swagger UI |
|----------|--------|---------|-----------|-----------|
| Develop | `develop` | push automático | via pipeline (DB secrets) | GitHub Pages `/develop/` |
| Production | `main` | push automático | via pipeline (DB secrets) | GitHub Pages `/production/` |

### Documentação OpenAPI (Swagger)

A cada deploy (`develop.yml`/`production.yml`), depois do `Update Lambda`, a
pipeline publica a spec já commitada em `docs/openapi/swagger.json` como
Swagger UI navegável no GitHub Pages (branch `gh-pages`, uma subpasta por
ambiente — `keep_files: true` garante que publicar um ambiente não apaga o
outro). O `host`/`basePath` da spec são recalculados nesse passo a partir da
secret `API_BASE_URL` do Environment correspondente (`develop`/`production`,
cadastrada sem o basePath da aplicação — ex: sem o `/v1`, que já vem da spec
gerada). Veja `scripts/publish-openapi-docs.sh`.

---

## Antipadrões

- **Não** adicione features em `release/**` — somente bug-fixes
- **Não** faça push direto para `main`
- **Não** mergee sem aprovação de reviewer
- **Não** pule develop — valide antes de criar a release
- **Não** crie migrations não-idempotentes — o guardrail vai pegar em CI
