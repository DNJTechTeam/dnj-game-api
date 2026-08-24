## Resumo

- Descrição do que foi feito, motivações e impacto das mudanças:

- Link da tarefa:

## Escopo e Incrementalidade

- [ ] O PR está apontando entre as branches corretas
- [ ] O PR é pequeno, incremental e revisável
- [ ] Não mistura refactors ou mudanças fora do escopo
- [ ] Comportamentos alterados estão descritos no resumo
- [ ] Se ficou grande, há justificativa clara:

## Segurança e Autorização

- [ ] Middleware de autenticação aplicado nas rotas que exigem login
- [ ] Service valida regras de negócio e acesso ao recurso (não apenas o JWT)
- [ ] Recurso inexistente retorna `ErrNotFound`; acesso negado retorna erro adequado
- [ ] A mudança não expõe dados de outros usuários por leitura/listagem

## Validação em Camadas

- [ ] DTOs validam entrada HTTP básica
- [ ] Services validam payload, estado final e referências
- [ ] Services usam transação quando a persistência abrange mais de uma tabela
- [ ] Erros de entrada retornam validação, não 500

## Banco e Migrations

- [ ] Migrations novas são idempotentes (ver `docs/migrations.md`)
- [ ] Rodam em banco limpo e em banco já existente sem erro
- [ ] Campos relacionais têm índices explícitos
- [ ] Não houve seed fora do fluxo de migrations

## Testes Automatizados

- [ ] Cobrem o fluxo principal (happy path)
- [ ] Cobrem ao menos um caso de erro por método público
- [ ] Cobrem owner-scoping (usuário não lê dados de outro) quando aplicável
- [ ] Seguem o padrão de `docs/TESTING.md`

Print da cobertura de testes (se aplicável):

Comandos executados:

```bash
make build
make vet
make test-services
make test-repos
make test
make test-cover-check test-admin-cover-check
```

Resultados:

## Checklist Final

- [ ] Marquei os checklists com atenção e garanto o funcionamento dos fluxos testados
- [ ] O PR está pronto para revisão, não introduz bugs conhecidos e está limitado ao escopo da tarefa
