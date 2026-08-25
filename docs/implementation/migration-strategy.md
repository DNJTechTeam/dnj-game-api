# Estratégia segura de evolução do banco

Toda mudança em banco usa `expand → backfill → contract`, preserva os dados de
`develop` e deve ser segura com versões antiga e nova da aplicação convivendo
durante o deploy.

## Expand

- Adicionar tabelas/colunas/índices de forma idempotente e compatível.
- Colunas novas começam opcionais ou com default seguro; não bloquear uma
  tabela grande enquanto o tráfego está ativo.
- Criar índice concorrentemente em migration própria quando o volume justificar.
- Subir código que leia o formato antigo e o novo antes do backfill.

## Backfill

- Processar em lotes limitados, com checkpoint e possibilidade de retomar.
- Tornar o comando reexecutável e publicar métricas de progresso/erro.
- Validar contagem, nulos, duplicidade e invariantes antes de avançar.
- Não fazer backfill longo no boot da Lambda ou dentro da migration de schema.

## Contract

- Só remover campo, índice ou comportamento legado após telemetria provar que
  nenhuma versão ativa depende dele.
- Executar backup/restauração ensaiada e documentar rollback antes da remoção.
- Contract destrutivo exige migration separada e janela aprovada.

## Garantias do runner

- `schema_migrations` guarda checksum SHA-256 de definição imutável.
- Registros legados sem checksum recebem backfill; alteração posterior falha
  fechada e exige uma migration nova.
- Lock transacional de linha com `SELECT FOR UPDATE`, compatível com PostgreSQL
  e CockroachDB, serializa deploys concorrentes.
- Cada migration e seu registro são confirmados na mesma transação.
- O gate executa cada função `Up` diretamente duas vezes, além do replay pelo
  registry; portanto não confunde “foi pulada” com “é idempotente”.

## Checklist por migration

- Nome, versão, descrição e `Definition` imutáveis.
- `Up` idempotente, `Down` consciente de perda e teste com PostgreSQL real.
- Teste de banco limpo, schema parcial relevante e concorrência quando aplicável.
- Plano de backup, validação, backfill, compatibilidade e rollback descritos no
  PR/commit que introduz a mudança.
- Nunca executar `db-reset`, `DROP SCHEMA` ou truncamento fora de container de
  teste descartável.
