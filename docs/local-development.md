# Desenvolvimento local isolado

O ambiente local foi desenhado para rodar ao lado do frontend sem disputar
portas ou depender de serviços compartilhados.

| Serviço | Porta host | Endereço |
|---|---:|---|
| Frontend (externo a este repo) | 3000 | `http://localhost:3000` |
| DNJ Game API | 8081 | `http://localhost:8081` |
| PostgreSQL | 55432 | `localhost:55432` |
| MinIO S3 | 59000 | `http://localhost:59000` |
| Console MinIO | 59001 | `http://localhost:59001` |

## Subir

```bash
cp .env.example .env
make local-up
make run-api
```

Em outro terminal:

```bash
curl -i http://localhost:8081/v2/healthcheck
curl -i http://localhost:8081/v2/readiness
```

O Compose usa o projeto `dnj-game-api-local`, volumes próprios e nomes gerados
pelo Compose. Não depende nem interfere em um PostgreSQL já exposto em `5432`.

## Parar e diagnosticar

```bash
make local-down       # preserva volumes
docker compose ps
docker compose logs db s3
```

`make db-reset` apaga apenas os volumes do projeto local e é destrutivo. Ele
nunca deve ser usado com credenciais ou hosts de `develop`/produção. A suíte de
testes cria PostgreSQL descartável via Testcontainers e não usa este banco.

## Contrato para o frontend

- Base V2 local: `http://localhost:8081/v2`.
- Enviar `X-Request-ID` opcionalmente; a API preserva IDs seguros ou cria um.
- Ler `X-Request-ID` em sucesso e erro para suporte.
- O contrato gerado fica em `docs/openapi/dnj-v2.openapi.json` e a documentação
  publicada fica sob `/<ambiente>/v2/` no GitHub Pages.
