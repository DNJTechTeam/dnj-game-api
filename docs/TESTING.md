# Testing Guidelines — Nucleus API

Documentação sobre padrões de testes adotados no projeto. Todo novo recurso API ou manutenção em existentes **deve incluir testes** seguindo as convenções descritas.

---

## Sumário executivo

- **Framework**: Go `testing` + `testify/assert` e `testify/require`
- **Padrão**: Subtest com nomenclatura `TestXxx_MethodName` → `t.Run("should do X", func(t *testing.T) { ... })`
- **Estrutura**: Given/When/Then com comentários
- **Setup**: Função `setupXxxTest(t *testing.T)` por serviço, com `t.Helper()` e `t.Cleanup()`
- **DB**: Testcontainers em `internal/infrastructure/di/test` (PostgreSQL real em cada suite)
- **Contexto**: `TestSuite.ContextWithUser(userID)` para simular usuário autenticado
- **Limpeza**: `TestSuite.TruncateTable(t, model)` antes/depois de cada teste
- **Gates**: código mantido nunca pode cair abaixo de 55%; mappers/repositories
  permanecem em 90% no mínimo (100% no baseline atual). A meta é elevar o gate
  mantido progressivamente até 90%, sem contar código gerado, DI ou runtime glue.

---

## 1. Estrutura de pacotes e nomenclatura

### Arquivo de teste

Crie um arquivo `*_test.go` no mesmo pacote do código sendo testado:

```
internal/app/services/
├── task_service.go
├── task_service_test.go        # Testes do TaskService
├── user_auth_service.go
├── user_auth_service_test.go   # Testes do UserAuthService
└── main_test.go                # Setup global, fixtures, context helpers
```

### Nomenclatura de testes

Use `TestXxx_MethodName` onde `Xxx` é o struct e `MethodName` é o método:

```go
func TestTaskService_Create(t *testing.T) { ... }
func TestTaskService_ListIsOwnerScoped(t *testing.T) { ... }
func TestUserAuthService_Login(t *testing.T) { ... }
```

---

## 2. Setup global (main_test.go)

O arquivo `internal/app/services/main_test.go` já existe e define a infraestrutura compartilhada:

```go
type TestSuiteType struct {
    *test.Containers
    UserRepository uInterfaces.UserRepositoryInterface
    TaskRepository taskInterfaces.TaskRepositoryInterface
    BaseService    *BaseService
}

var TestSuite *TestSuiteType

func TestMain(m *testing.M) {
    initializeTestSuite()
    code := m.Run()
    test.HandleShutdown(TestSuite.Containers)
    os.Exit(code)
}
```

**Ao criar um novo recurso**, adicione o repositório correspondente ao `TestSuiteType` e inicialize em `initializeTestSuite()`:

```go
type TestSuiteType struct {
    *test.Containers
    // ...
    WidgetRepository widgetInterfaces.WidgetRepositoryInterface  // <-- adicionar
}

func initializeTestSuite() {
    TestSuite = &TestSuiteType{...}
    // ...
    TestSuite.WidgetRepository = repositories.NewWidgetRepository(TestSuite.DbConn)
}
```

### DefaultSetup e TruncateTable

```go
// Configura ENV vars para o teste
TestSuite.DefaultSetup(t)

// Limpa uma tabela (CASCADE)
TestSuite.TruncateTable(t, &models.Task{})
```

### ContextWithUser

Retorna um contexto com o `userID` injetado, exatamente como o `AuthenticationMiddleware` faz em produção:

```go
ctx := TestSuite.ContextWithUser(user.ID)
```

---

## 3. Setup por serviço

Cada arquivo de teste deve ter uma função `setupXxxTest(t *testing.T)` que:

1. Marca como helper via `t.Helper()`
2. Chama `TestSuite.DefaultSetup(t)`
3. Trunca as tabelas relevantes
4. Registra cleanup automático via `t.Cleanup()`
5. Retorna o serviço instanciado

```go
func setupTaskServiceTest(t *testing.T) interfaces.TaskServiceInterface {
    t.Helper()
    TestSuite.DefaultSetup(t)
    TestSuite.TruncateTable(t, &models.Task{})
    TestSuite.TruncateTable(t, &models.User{})
    t.Cleanup(func() {
        TestSuite.TruncateTable(t, &models.Task{})
        TestSuite.TruncateTable(t, &models.User{})
    })
    return NewTaskService(TestSuite.BaseService, TestSuite.TaskRepository)
}
```

---

## 4. Padrão Given/When/Then

Toda função `t.Run()` deve seguir a estrutura:

```go
t.Run("should create task for authenticated user", func(t *testing.T) {
    // given
    svc := setupTaskServiceTest(t)
    user, ctx := seedTaskUser(t, "owner@example.com")
    req := &messages.CreateTaskRequestDTO{Title: "Write docs"}

    // when
    result, err := svc.Create(ctx, req)

    // then
    require.NoError(t, err)
    assert.Equal(t, "Write docs", result.Title)
    assert.NotZero(t, result.ID.Uint64())
})
```

**Regras**:
- Comentários `// given`, `// when`, `// then` são **obrigatórios**
- `when` executa a ação; pode ter múltiplas chamadas quando testamos uma sequência
- `then` agrupa todas as asserções sem intercalar com setup

---

## 5. Assertions: require vs assert

- **`require`**: Para erros críticos que invalidam o restante do teste. O teste para aqui se falhar.
- **`assert`**: Para verificações de valor. O teste continua mesmo se falhar.

```go
// Erros: use require
require.NoError(t, err)
require.NotNil(t, result)

// Valores: use assert
assert.Equal(t, "Write docs", result.Title)
assert.NotZero(t, result.ID.Uint64())
```

---

## 6. Seed helpers

Defina funções seed em `test_helpers_test.go` para criar dados de teste sem repetição:

```go
func seedTaskUser(t *testing.T, email string) (*uEntities.User, context.Context) {
    t.Helper()
    user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &uEntities.User{
        Email:    email,
        Name:     "Test User",
        Password: common.GenerateRandomPassword(),
    })
    require.NoError(t, err)
    return user, TestSuite.ContextWithUser(user.ID)
}
```

---

## 7. Padrões por tipo de teste

### 7.1 Teste CRUD

```go
t.Run("creates reads updates and deletes", func(t *testing.T) {
    // given
    svc := setupTaskServiceTest(t)
    _, ctx := seedTaskUser(t, "owner@example.com")

    // when
    created, createErr := svc.Create(ctx, &messages.CreateTaskRequestDTO{Title: "My Task"})
    got, getErr := svc.Get(ctx, created.ID.Uint64())
    deleteErr := svc.Delete(ctx, created.ID.Uint64())
    _, findErr := svc.Get(ctx, created.ID.Uint64())

    // then
    require.NoError(t, createErr)
    require.NoError(t, getErr)
    assert.Equal(t, created.ID, got.ID)
    require.NoError(t, deleteErr)
    assert.ErrorIs(t, findErr, appErrors.ErrNotFound)
})
```

### 7.2 Teste de validação

```go
t.Run("returns validation error for short title", func(t *testing.T) {
    // given
    svc := setupTaskServiceTest(t)
    _, ctx := seedTaskUser(t, "user@example.com")

    // when
    _, err := svc.Create(ctx, &messages.CreateTaskRequestDTO{Title: "no"})

    // then
    require.Error(t, err)
    assert.IsType(t, &appErrors.Error{}, err)
})
```

### 7.3 Teste de owner-scoping

Sempre teste que um usuário não consegue acessar dados de outro:

```go
t.Run("returns not found when user reads another user's resource", func(t *testing.T) {
    // given
    svc := setupTaskServiceTest(t)
    _, ctxA := seedTaskUser(t, "a@example.com")
    _, ctxB := seedTaskUser(t, "b@example.com")
    created, _ := svc.Create(ctxA, &messages.CreateTaskRequestDTO{Title: "Private"})

    // when
    _, err := svc.Get(ctxB, created.ID.Uint64())

    // then
    assert.ErrorIs(t, err, appErrors.ErrNotFound)
})
```

---

## 8. Teste de idempotência de migrations

Existe o teste `TestMigrations_AreIdempotent` em `internal/app/services/` que re-executa todas as migrations no banco já migrado e garante que nenhuma falha.

```bash
make test-migrations
```

---

## 9. Cobertura esperada

- **Services**: 100% dos métodos públicos cobertos, incluindo caminhos de erro
- **Repositórios**: cobertura indireta via testes de service (testcontainers)
- **Lógica de domínio**: 100% (entidades, enums, validações)

O gate de cobertura mínima de **90%** é aplicado sobre `db/mappers` e `db/repositories` e verificado em CI via `make test-cover-check`.

---

## 10. Executando testes

```bash
# Todos os testes
make test

# Apenas services
make test-services

# Apenas repositórios
make test-repos

# Com coverage e gate
make test-cover-check

# Idempotência de migrations
make test-migrations

# Teste específico
go test -run TestTaskService_Create ./internal/app/services/...
```

---

## 11. Troubleshooting

**"TRUNCATE TABLE failed: FK constraint"**
A ordem de truncate deve respeitar constraints. Truncate a tabela dependente antes da referenciada.

**"container failed to start"**
Testcontainers exige Docker. Verifique se o daemon está rodando com `docker ps`.

**Testes intermitentes (flaky)**
- Use sempre `t.Cleanup()` para limpar estado compartilhado
- Evite `time.Sleep()`; prefira polling ou asserções com timeout
- Use `require.NoError()` em setup para falhar rápido antes de asserções

---

## Referências

- `internal/app/services/main_test.go` — Setup global (TestSuiteType, TestMain)
- `internal/app/services/test_helpers_test.go` — Context e seed helpers
- `internal/app/services/task_service_test.go` — Exemplo completo (CRUD + owner-scoping)
- `docs/new-resource-guide.md` — Passo a passo para adicionar um novo recurso com testes
