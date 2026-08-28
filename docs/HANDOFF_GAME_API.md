# Handoff Game API — DNJ V2

**Este documento é a especificação de implementação do `dnj-game-api`.**
**Stack já existente a preservar:** Go, Gin, GORM, Wire, PostgreSQL, JWT, Testcontainers e runner HTTP/AWS Lambda.
**O que será removido:** Supabase e Route Handlers do `dnj-game-front`; recursos de exemplo/legados do Go (`Task`, subscriptions/webhooks e o fluxo de auth que não servir ao DNJ).
**O que não será removido:** a arquitetura em camadas, o registry de migrations Go, DI Wire, testes, PostgreSQL e o prefixo `/v1`.

## 1. Regra de transição

O Go API é o backend canônico. Não haverá um período em que o Go replica Supabase, nem endpoints `/v2` paralelos no Go.

```text
Antes: Next Route Handlers → Supabase Storage/PostgREST
Depois: Next Frontend → dnj-game-api /v1 → PostgreSQL + AWS S3
```

O front deve trocar sua base URL/adapters para o Go API. O contrato OpenAPI `1.1.0-draft` do front é referência funcional, mas não é a fonte de verdade do novo banco.

## 2. Decisões fechadas de domínio

| Decisão | Implementação no Go API |
| --- | --- |
| DNJ é edição única | Não criar `events` ou `event_id`. |
| Local ≠ ação | `Space` é local físico; `Activity` é tudo que acontece. |
| QR é ação | `QRCode` pertence a `Activity`, nunca a `Space`. |
| Foto livre existe | `Moment.ParticipationID` é nullable. |
| Foto de desafio pontua | `Moment.Origin = challenge`, vinculada a Participation válida. |
| Uma foto por desafio | Um usuário só envia um Moment de desafio por `Activity`, mesmo que tente criar outra Participation. |
| Galeria não é tabela | `GET /moments?scope=feed` consulta Moments públicos. |
| Sem fila | Não criar Queue/QueueEntry. |
| IDs | Usar `uint64` interno e `messages.Uint64String` nas respostas JSON, seguindo o template. |
| Mídia | S3 privado; banco armazena `MediaAsset`, nunca bytes. Feed é visível só no app autenticado. |
| Moderação | Moment nasce aprovado; Admin pode remover foto ou reverter pontos como ações independentes. |

## 3. O que muda no projeto existente

### Remover/substituir

| Atual | Ação |
| --- | --- |
| `Task` em todas as camadas | Remover após usar como referência para os recursos DNJ. |
| `SubscriptionWebhook` e `SubscriptionWebhookVerificationCode` | Remover, salvo se a integração de inscrições externas for confirmada como requisito. |
| `User.Document` em texto puro | Substituir por `DocumentHash` + `DocumentLast4`. |
| Auth de onboarding/verificação por e-mail do template | Reescrever para o provedor real de OTP escolhido; manter JWT e middleware. |
| `RoleEventManager`/nomes de template | Normalizar para `participant`, `manager`, `admin`. |

### Preservar e estender

| Estrutura existente | Uso V2 |
| --- | --- |
| `internal/domain/<resource>` | Entidade e interface por recurso DNJ. |
| `internal/infrastructure/db/models` | Models GORM. |
| `internal/infrastructure/db/repositories` | Persistência e locks SQL necessários. |
| `internal/app/services/BaseService.WithTransaction` | QR, Moment pontuado, resultado de run e reversão. |
| `internal/presentation/api/handlers` e `routers` | Contrato `/v1`. |
| `cmd/api/di/wire.go` | Providers dos novos recursos. |
| `model_migrations.go` | Migrations Go idempotentes registradas em ordem. |

## 4. Migrations reais deste projeto

As migrations **não** serão arquivos SQL numerados. Devem ser funções Go registradas em `internal/infrastructure/db/migrations/model_migrations.go`, usando o `MigrationRegistry` existente. Cada `Up` é idempotente.

Para tabelas simples, usar `createModelMigration`. Para enum PostgreSQL, índice parcial, trigger e function, registrar `Migration{ Up: func(db *gorm.DB) error { ... } }` com `CREATE ... IF NOT EXISTS` ou guardas de catálogo.

### Ordem de registro

```text
create_dnj_groups_table
create_dnj_users_table
create_dnj_spaces_table
create_dnj_activities_table
create_dnj_activity_manager_assignments_table
create_dnj_qr_codes_table
create_dnj_participations_table
create_dnj_media_assets_table
create_dnj_moments_table
create_dnj_moment_likes_table
create_dnj_activity_runs_table
create_dnj_activity_run_participants_table
create_dnj_point_entries_table
create_dnj_live_announcements_table
create_dnj_operation_audit_table
create_dnj_constraints_indexes_and_functions
```

Antes do primeiro deploy DNJ, remover do registry as migrations de `tasks` e subscriptions. Como o projeto ainda é scaffold pré-produção, a baseline pode ser reorganizada uma vez; depois disso, nomes de migrations são imutáveis.

## 5. Models GORM obrigatórios

### User e Group

```go
type User struct {
    ID            uint64 `gorm:"primaryKey;autoIncrement"`
    Email         string `gorm:"uniqueIndex;not null"`
    DisplayName   string `gorm:"not null"`
    DocumentHash  []byte `gorm:"uniqueIndex;not null"`
    DocumentLast4 string `gorm:"not null"`
    MobilePhone   *string
    GroupID       *uint64 `gorm:"index"`
    Role          string `gorm:"not null;default:participant"`
    Points        int `gorm:"not null;default:0"`
    LastSeenAt    *time.Time
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type Group struct { ID uint64; Name string }
```

O documento recebido é normalizado e transformado em HMAC-SHA-256 com segredo da API. Não guardar CPF puro, nem retorná-lo. O DTO responde `documentMasked`.

### Operação

```text
Space(id, slug UNIQUE, name, mapReference)
Activity(id, spaceID NULL, slug UNIQUE, name, description NULL,
         kind, status, startsAt NULL, endsAt NULL,
         checkInPoints, momentPoints, cooldownSeconds, allowsMoment)
ActivityManagerAssignment(activityID, userID) PK composto
QRCode(id, activityID, tokenHash UNIQUE, scanExpiresAt, momentExpiresAt NULL,
       maxUses NULL, usedCount, status)
Participation(id, userID, activityID, qrCodeID NULL, checkedInAt,
              cooldownEndsAt NULL, status, canShareMoment, checkInPoints,
              idempotencyKey UNIQUE por usuário/activity)
ActivityRun(id, activityID, startedBy NULL, status, pointRules JSONB,
            startedAt NULL, endedAt NULL)
ActivityRunParticipant(activityRunID, userID, placement NULL, pointsAwarded, status)
PointEntry(id, userID, participationID NULL, activityRunID NULL, reason, delta,
           idempotencyKey UNIQUE)
LiveAnnouncement(id, activityID NULL, title, startsAt, endsAt, teaserSeconds,
                 points, deliveryTargets, status, createdBy NULL)
OperationAudit(id, actorUserID NULL, action, entityType, entityID NULL, metadata JSONB)
```

### S3 e Moments

```text
MediaAsset(id, ownerUserID, provider='s3', bucket, objectKey UNIQUE,
           contentType, bytes, checksumSHA256 NULL, uploadStatus,
           deletedAt NULL)
Moment(id, userID, participationID NULL, mediaAssetID UNIQUE,
       origin='free|challenge', publicationStatus='private|public',
       moderationStatus='approved|rejected', rewardStatus,
       pointsAwarded, challengeActivityID NULL, idempotencyKey UNIQUE, capturedAt)
MomentLike(momentID, userID) PK composto
```

## 6. Constraints e funções que exigem SQL explícito

GORM não é suficiente para estas regras; implementar migration SQL idempotente:

1. `moments`: `origin = free` exige `participation_id IS NULL`, `challenge_activity_id IS NULL`, pontos zero e `reward_status = not_applicable`.
2. Moment `challenge` preenche `challenge_activity_id` com a Activity da Participation. Índice parcial: `UNIQUE (user_id, challenge_activity_id) WHERE challenge_activity_id IS NOT NULL`. Isso garante uma foto por pessoa por desafio, mesmo sob tentativa de criar outra Participation.
3. Índice público: Moments públicos/aprovados por `captured_at DESC`.
4. Trigger de Moment desafio: Participation e MediaAsset pertencem ao mesmo usuário; `challenge_activity_id` é a Activity da Participation; a Activity permite foto; janela de foto está válida.
5. Procedure/serviço transacional `ValidateQR`: lock em QR e usuário, limita usos, cria/recupera Participation, cria PointEntry e atualiza saldo.
6. Procedure/serviço transacional `AwardPoints`: insere PointEntry com idempotência e atualiza `users.points` uma vez.
7. Like valida Moment público, aprovado e mídia disponível antes de inserir/remover.

`users.points` é saldo materializado para ranking. `point_entries` é o livro-razão imutável. Reversão de pontos cria entrada negativa; não edita histórico.

## 7. S3, entrega rápida e cache de imagens

Criar interface `MediaStorageInterface` em `internal/app/interfaces` e adapter AWS SDK v2 em `internal/infrastructure/storage/s3`.

```text
CreateUploadIntent(owner, contentType, bytes) -> MediaAsset + presigned PUT
ConfirmUpload(owner, mediaAssetID) -> HeadObject + MediaAsset available
CreateReadURL(requester, mediaAssetID) -> presigned GET curto, após autorização
DeleteObject(mediaAssetID) -> marca deleted e remove/agenda S3
```

Regras:

- bucket privado, Block Public Access ligado;
- chave: `private/<user-id>/<random>.<ext>`;
- máximo inicial: 10 MB; JPEG, PNG e WebP;
- presigned PUT/GET de curta duração;
- objeto pendente expira via S3 Lifecycle em 24h;
- Moment só referencia asset `available` do próprio usuário.

`public` em `Moment.PublicationStatus` **não** torna o objeto S3 público. Significa somente que participantes autenticados podem vê-lo no feed. A API decide a leitura: dono vê seus Moments privados; participante vê Moments de feed aprovados; Admin acessa mídia necessária para operação e moderação.

### Cache sem expor fotos

Após captura, o front mostra o `Blob` local imediatamente; upload não bloqueia a percepção de velocidade. Para leituras seguintes, retornar uma URL estável por `mediaAssetId` através do backend/Next e enviar `Cache-Control: private, max-age=300, stale-while-revalidate=60`. O navegador pode reutilizar a imagem, mas caches compartilhados não podem servi-la a terceiros.

Não colocar mídia autenticada em cache genérico do Service Worker. O cache HTTP privado é suficiente e preserva a política atual do PWA de não armazenar respostas de API autenticada. Ao remover uma foto, o backend deixa de autorizá-la, o feed deixa de listá-la e o TTL curto limita qualquer visualização local antiga.

## 8. Contrato HTTP canônico: `/v1`

O prefixo vem de `API_PREFIX` e deve permanecer `/v1`. Não criar `/v2` enquanto o Go API é o primeiro backend real.

### Auth e perfil

| Método | Rota | Autorização | Corpo/resultado |
| --- | --- | --- | --- |
| POST | `/v1/auth/register` | pública | `name`, `email`, `document`, `mobilePhone`, `groupId?`; inicia OTP. |
| POST | `/v1/auth/verification-code` | pública | identifica canal + código; retorna JWT e `ApiUser`. |
| GET | `/v1/users/me` | participante | perfil atual. |
| PATCH | `/v1/users/me/group` | participante | `{ groupId: string|null }`. |
| GET | `/v1/groups?search=` | pública ou participante | grupos pesquisáveis. |

### Descoberta, game e anúncio

| Método | Rota | Resultado |
| --- | --- | --- |
| GET | `/v1/spaces` | locais do mapa. |
| GET | `/v1/schedule?view=home&sector=` | Activities de programação e estado derivado. |
| GET | `/v1/game/overview` | ranking, grupo, histórico próprio de pontos. |
| GET | `/v1/special-events/active` | alias público para `LiveAnnouncement` ativo; manter nome enquanto o front usa esse conceito. |

### QR e Participation

| Método | Rota | Corpo | Regra |
| --- | --- | --- | --- |
| POST | `/v1/qr/validate` | `qrToken`, `idempotencyKey` | cria ou recupera Participation atômica. |
| GET | `/v1/participations/current` | — | Participation atual elegível para foto ou `204`. |
| POST | `/v1/moment-challenges/{activityId}/participations` | `idempotencyKey` | cria/recupera entrada de desafio sem QR público. |

`Participation` retorna `activity`, `place`, horários, status e pontos; **não retorna `event`**.

### Mídia, Moments e likes

| Método | Rota | Regra |
| --- | --- | --- |
| POST | `/v1/media/upload-intents` | cria MediaAsset pendente e presigned PUT. |
| POST | `/v1/media/{mediaAssetId}/complete` | confirma objeto S3 do dono. |
| GET | `/v1/media/{mediaAssetId}` | leitura autorizada/assinada. |
| GET | `/v1/moments?scope=feed|mine|group&cursor=&limit=` | participante autenticado; feed/coleções, sem tabela Gallery. |
| POST | `/v1/moments` | cria Moment livre ou de desafio. |
| POST | `/v1/moments/{momentId}/likes` | toggle de like. |

`POST /v1/moments`:

```json
{
  "mediaAssetId": "123",
  "publishConsent": true,
  "participationId": "456 opcional"
}
```

Sem `participationId`, o servidor cria `origin=free`, `pointsAwarded=0`. Com ele, o servidor tenta `origin=challenge` e valida todas as regras. O cliente nunca envia origem ou pontos.

### Gestão e admin

| Método | Rota | Papel |
| --- | --- | --- |
| POST | `/v1/manager/activities/{id}/start` | Manager atribuído. |
| POST | `/v1/manager/activities/{id}/pause` | Manager atribuído. |
| POST | `/v1/manager/runs` | Manager de Activity competitiva. |
| POST | `/v1/manager/runs/{id}/qr` | Gera QR dinâmico do run. |
| POST | `/v1/manager/runs/{id}/results` | Fecha run e premia atomicamente. |
| POST | `/v1/admin/special-events` | Admin cria LiveAnnouncement. |
| PATCH | `/v1/admin/moments/{id}/moderation` | Admin executa `removePhoto` ou `removePoints`. |

As ações administrativas são independentes:

```text
removePhoto
  → Moment sai do feed; publicationStatus=private; moderationStatus=rejected;
    MediaAsset é removido/agendado para remoção no S3;
    se houve prêmio, cria PointEntry negativo e rewardStatus=reversed.

removePoints
  → Foto permanece aprovada e no feed; não remove MediaAsset;
    cria PointEntry negativo exatamente uma vez; rewardStatus=reversed.
```

Moment criado com consentimento público nasce `publicationStatus=public` e `moderationStatus=approved`. Não há fila de pré-moderação; Admin corrige posteriormente se necessário.

## 9. Implementação por vertical slice

Implementar na sequência abaixo. Para cada recurso, copiar o padrão `Task` por todas as camadas; não criar handler/repository isolado.

1. Ajustar `User` e `Group`, auth e autorização por papel.
2. `Space` e `Activity`.
3. `QRCode`, `Participation` e serviço transacional de QR.
4. `PointEntry`, saldo e `GameOverview`.
5. `MediaAsset`, adapter S3 e `Moment`/likes.
6. `ActivityRun` e resultados da Radicalidade.
7. `LiveAnnouncement`, moderação e auditoria.

Para cada slice: entity + repository interface + GORM model + mapper + repository + migration + DTOs + service + handler + router + providers Wire + mocks + testes. Após mudar DI, rodar `go generate ./cmd/api/di`; após interface mockada, rodar `mockery`.

## 10. Testes de aceite obrigatórios

1. Dois scans concorrentes no último uso do QR: somente um cria Participation.
2. Mesmo `Idempotency-Key` de QR: mesma Participation, mesmo saldo e uma PointEntry.
3. Moment livre: Participation nula e zero pontos.
4. Moment desafio de outro usuário/inválido/expirado: falha sem criar Moment nem ponto.
5. Reenvio de desafio, inclusive com outra Participation criada indevidamente: uma foto e uma premiação apenas por usuário + Activity.
6. Like paralelo: uma linha por par Moment/User.
7. Feed de grupo inclui Moment livre de integrante, mas não Moment privado.
8. Asset não confirmado não cria Moment; objeto órfão é limpo sem afetar foto válida.
9. Reversão administrativa cria entrada negativa e corrige saldo uma vez.
10. `sum(point_entries.delta) = users.points` para usuários de teste.

Executar como mínimo: `go generate ./cmd/api/di`, `mockery`, `go build ./...`, `make test-migrations` e `make test-cover-check`.

## 11. Pendências explícitas

| Tema | Default proposto | Precisa confirmar? |
| --- | --- | --- |
| Provider OTP | Adapter próprio; e-mail ou SMS definido por configuração. | Sim. |
| Telefone | Obrigatório se OTP for SMS. | Sim. |
| Foto por desafio | Uma por usuário + Activity, via índice parcial em `challenge_activity_id`. | Não. |
| Moderação | Publica aprovada; `removePhoto` e `removePoints` são ações independentes. | Não. |
| Retenção/LGPD | Prazo e processo de exclusão de fotos. | Sim. |
| Webhook de inscrição | Remover exemplo até requisito concreto. | Sim. |

## 12. Definition of Done

- O Go API substitui integralmente as chamadas backend/Supabase do front.
- Não há tabela, endpoint ou migration DNJ dependente de `events`, filas ou Gallery.
- O banco vazio sobe por `cmd/migrate` e passa o teste de idempotência.
- O fluxo funciona ponta a ponta: identidade → QR → pontos → S3 → Moment livre/desafio → like → ranking → operação/admin.
- Nenhuma resposta expõe documento puro, token QR bruto, credencial AWS ou URL pública permanente do S3.
