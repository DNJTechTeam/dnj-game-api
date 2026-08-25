# Mídia, Moments, galeria, moderação e retenção — Iteração 7

Este guia descreve o contrato implementado em `/v2`. `Moment` é a única
entidade persistida; galeria é somente consulta de `Moment`. Não existem
tabelas `gallery` nem suporte multi-evento (`events`, `event_id`).

## Upload e sanitização

`POST /media/upload-intents` recebe estritamente `{contentType, bytes,
checksumSha256}`. Aceita somente `image/jpeg` e `image/png`, até 10 MiB; o
checksum é SHA-256 em Base64 canônico. O servidor gera uma `staging key`
privada e aleatória, sem PII, e assina `Content-Type`, tamanho/checksum
suportado e o checksum nativo do S3. A resposta contém somente `id`,
`uploadUrl`, `method`, `headers` e `expiresAt`; a intenção e a URL assinada
expiram em 10 minutos. A URL assinada **nunca** é persistida — retry
idempotente reconstrói a mesma resposta a partir de metadados seguros e do
instante original da intenção; retry após expiração retorna o resultado
original já expirado, e uma nova intenção sempre gera uma `staging key` nova.

`POST /media/{mediaAssetId}/complete` não aceita corpo e:

1. valida ator, ownership, estado e expiração;
2. obtém metadados reais do objeto no provedor (HEAD);
3. confere tamanho, content type e checksum contra o que foi declarado na
   intenção — nunca confia apenas no `Content-Type` do cliente;
4. baixa o objeto com limite estrito de bytes;
5. valida magic bytes e a configuração real da imagem (dimensões/pixels);
6. rejeita conteúdo disfarçado, truncado ou malformado;
7. limita a 20 megapixels o total de pixels decodificados;
8. decodifica e reencoda a imagem em formato permitido, removendo EXIF, GPS
   e demais metadados;
9. grava uma versão final privada, imutável e com chave distinta da staging
   key;
10. só marca o asset `available` depois da versão final gravada com sucesso;
11. enfileira a remoção assíncrona da staging key.

A confirmação é serializada por um **claim/lease persistido** (não por
transação de banco aberta durante as chamadas ao provedor): o lease tem
expiração e pode ser retomado por outro processo, sobrevivendo a reinício.
Erros publicados: `409 UPLOAD_INCOMPLETE` (objeto ainda não chegou ao
provedor), `409 UPLOAD_STATE_CONFLICT` (estado incompatível ou confirmação já
em voo sob outra chave), `410 UPLOAD_EXPIRED`, `503 MEDIA_UNAVAILABLE`
(indisponibilidade do provedor). Asset alheio ou inexistente sempre retorna o
mesmo `404 NOT_FOUND`, sem permitir enumeração.

O `version ID` da versão final confirmada é persistido internamente para
impedir que um PUT tardio ao mesmo objeto substitua o conteúdo já vinculado a
um Moment.

## Moments e galeria

`POST /moments` recebe estritamente `{mediaAssetId, publishConsent,
participationId?}`. Sem `participationId` o Moment é `free`, sem pontos.
Com `participationId`, a origem é `challenge` e a Participation deve:
pertencer ao ator, referenciar uma Activity existente, ter
`canShareMoment=true`, a Activity precisa ter `allowsMoment=true`, não estar
cancelada e estar dentro da janela operacional da Activity (até o instante
exato de `endsAt` quando existir; sem `endsAt`, até a Activity deixar de estar
`active` ou ser arquivada). O servidor deriva usuário, origem, Activity,
grupo, local, estados, timestamps, pontos e ownership — nunca aceita nenhum
desses campos do cliente. `publishConsent=true` publica `public` e, se
`challenge`, concede o snapshot de `Activity.momentPoints`; `publishConsent=
false` publica `private` e nunca concede pontos. Moments entram sempre
`approved`, preservando o fluxo corretivo. Um `mediaAssetId` não pode ser
reutilizado e uma Participation não pode produzir dois Moments, mesmo sob
chaves ou requisições concorrentes — a unicidade é garantida por índices
parciais no banco, não em memória.

`GET /moments` aceita somente `scope=feed|mine|group` (exatamente um valor) e
`cursor` opaco, assinado por HMAC e sem PII; cursor inválido retorna `400`.
Paginação de 20 itens por página, ordenada por `capturedAt DESC,id DESC`.
`feed` e `group` mostram somente Moments públicos, aprovados, com asset
disponível e autor elegível (existente, onboarding completo, papel atual
`DEFAULT`); `group` usa o grupo atual persistido do ator, refletindo
imediatamente qualquer mudança de grupo. `mine` preserva todo o histórico
próprio, inclusive privados ou rejeitados, mas nunca emite URL para um asset
removido. Moment de usuário removido some de `feed`/`group` mas seu histórico
permanece intacto. `imageUrl`, `thumbnailUrl` e `shareImageUrl` apontam para a
mesma versão sanitizada e assinada nesta iteração (sem thumbnails/imagem de
compartilhamento separadas) e expiram em no máximo 5 minutos.

`POST /moments/{momentId}/likes` não aceita corpo e alterna o like do ator
atual. Só é permitido em Moment público, aprovado e com asset disponível;
inexistente ou invisível retorna o mesmo `404`. É serializado por
`(momentId,userId)`; a mesma chave idempotente retorna exatamente o estado
original, e chaves diferentes concorrentes aplicam-se deterministicamente em
ordem transacional. A resposta contém somente `momentId`, `liked` e
`likesCount`. Like nunca gera pontos nem `operation_audit`, e o contador é
sempre derivado/materializado de forma transacional — nunca mantido só em
memória.

## Pontos de challenge

O ledger da Iteração 6 é estendido, sem editar lançamentos existentes: uma
referência segura ao Moment é adicionada a `point_entries`, com origens e
motivos explícitos para premiação (`moment_challenge_award`) e reversão
(`moment_moderation_reversal`). O trigger append-only e as FKs `RESTRICT` são
preservados. Para uma premiação de challenge: Moment, Participation e usuário
são bloqueados em ordem estável; exatamente um lançamento positivo é
inserido; `users.points` é atualizado na mesma transação; a recompensa só
passa a `awarded` depois de ambos persistidos; a unicidade é por
`(moment_id,user_id,reason)`.

## Moderação corretiva

`GET /admin/moments/moderation` aceita somente `queue=general|challenge` e
`page` opcional; envelope `{data,pagination}`, limite 50, ordenado por
`capturedAt ASC,id ASC`. `general` lista Moments `free` públicos aprovados
disponíveis; `challenge`, o mesmo para origem `challenge`. O DTO expõe somente
id do Moment, imagem assinada de curta duração, `capturedAt`, nome público do
participante, Activity, pontos concedidos, estados de publicação/moderação/
recompensa/foto e ações corretivas disponíveis — nunca email, documento,
telefone, grupo interno, chave de storage ou auditoria.

`POST /admin/moments/{momentId}/moderation` recebe estritamente `{action}`,
sem campo `reason`. Ações permitidas: `deny_points` (só válida quando houve
premiação; reverte pontos atomicamente; torna o Moment `private`/`rejected`;
preserva a foto para o autor; não remove o asset) e `delete_photo` (torna o
Moment `private`/`rejected`; marca o asset removido logicamente; reverte
pontos quando existirem; enfileira limpeza física assíncrona; preserva
Moment, decisão, ledger e histórico). `deny_points` sobre Moment sem prêmio
retorna `409 MODERATION_ACTION_INVALID`. Uma decisão já aplicada, reenviada
com chave nova, retorna o estado terminal seguro sem novo efeito. Operações
bem-sucedidas geram `operation_audit` mínimo (ação + referências), nunca
imagem, URL assinada, chave de storage ou PII; retries e no-ops não duplicam
audit.

## Retenção e recuperação

`DNJ_MEDIA_RETENTION_ANCHOR_AT` é uma configuração obrigatória em RFC 3339
com offset, representando o término canônico da edição; é normalizada para
UTC antes de qualquer cálculo — nunca por concatenação ingênua de `Z`. Cada
asset guarda o snapshot `retentionDueAt = anchor + 90 dias`; se um upload
ocorrer depois do anchor, a retenção nunca é reduzida para menos de 90 dias
desde a criação. Ausência ou configuração inválida impede novas intenções com
`503 MEDIA_UNAVAILABLE`, sem derrubar healthcheck, catálogo ou autenticação —
o readiness verifica apenas a configuração do provedor, nunca faz um HEAD
externo síncrono que possa derrubar a API por uma falha transitória.

Assets pendentes após a expiração passam a `failed` e entram na limpeza.
Quando `retentionDueAt` chega, a API para imediatamente de emitir URLs para
aquele asset. O worker interno (`cmd/media-worker`, sem endpoint
administrativo de disparo) expira uploads pendentes, reclama jobs de limpeza
com lease persistido e remove versões no provedor, marcando o job concluído
somente após a remoção confirmada. Falhas usam retry com backoff, jitter e
limite de tentativas, com estado persistido; claims expirados podem ser
retomados por outra execução do worker, inclusive após reinício do processo.
Jobs nunca guardam URL, credencial, PII ou conteúdo. Exclusão do objeto no
provedor **não** remove fisicamente Moment, asset, likes históricos,
moderação ou ledger — a fonte auditável permanece sempre no banco; o
lifecycle do bucket é somente defesa em profundidade contra órfãos.

O worker expõe métricas para pendentes, processando, expirados, falhos,
retries e idade do job mais antigo, sem usar `user ID`, `asset ID` ou
`moment ID` como label de alta cardinalidade.

## Operação S3/MinIO

Buckets permanecem privados, com TLS e versionamento habilitado. O
provisionamento local/develop habilita versionamento e uma regra de
lifecycle que expira objetos sob o prefixo `staging/` como defesa adicional
(a fonte auditável continua sendo o banco). Nunca logar `Authorization`,
cookies, URL assinada, query string assinada, token, credencial AWS, chave
S3, checksum, imagem, PII ou payload integral; erros do SDK de storage são
redigidos antes de qualquer log. Cache de URLs assinadas é sempre privado —
nunca permitir cache público de mídia privada. Relógio, presigner e cálculo
de retenção são todos injetáveis para viabilizar testes determinísticos.

## Idempotência e concorrência

Toda escrita HTTP desta iteração exige `Idempotency-Key` UUID no header. A
intenção idempotente inclui ator, operação, recurso e um fingerprint
canônico do payload; mesma chave e mesma intenção repetem exatamente o
resultado original. Reuso da mesma chave para outra operação, asset, Moment,
Participation, ação ou payload retorna `409 IDEMPOTENCY_KEY_REUSED`. Essa
unicidade atravessa também as operações de participante, gestor e admin das
Iterações 1–6, através da tabela unificada `idempotency_operations` — não é
só um backfill de dados históricos: `admin_operation_repository.go`,
`favorite_repository.go` e `game_repository.go` gravam nela em cada escrita.
Nenhuma resposta idempotente persiste corpo integral ou URL assinada; locks
são sempre ordenados por asset, Moment, Participation e usuário, usando IDs
estáveis.

## Matriz HTTP publicada

| Família | Status publicados |
|---|---|
| intenção de upload | 201, 400, 401, 403, 409, 413, 415, 500, 503 |
| confirmação/sanitização | 200, 400, 401, 403, 404, 409, 410, 413, 422, 500, 503 |
| listagem de Moments | 200, 400, 401, 403, 409, 500 |
| criação de Moment | 201, 400, 401, 403, 404, 409, 500 |
| like | 200, 400, 401, 403, 404, 409, 500 |
| listagem de moderação | 200, 400, 401, 403, 500 |
| decisão de moderação | 200, 400, 401, 403, 404, 409, 500 |

`429` não é publicado nesta iteração; limites e perfis de carga permanecem
insumo explícito da Iteração 9.

## Operação e diagnóstico

- Nunca logar corpo integral, checksum, imagem, PII, URL assinada ou
  credencial do provedor.
- Para investigar um retry duvidoso, consulte `idempotency_operations` pela
  identidade e chave, sem registrar corpo original ou `ResponseSnapshot`
  fora do necessário.
- Para divergência de pontos originada em Moments, use a mesma auditoria de
  reconciliação `SUM(point_entries.delta) = users.points` já publicada na
  Iteração 6 — ela cobre também prêmios e reversões de Moments.
- Para um claim/lease preso, verifique `media_processing_claims` e
  `media_cleanup_jobs`; leases expirados são retomáveis por qualquer execução
  do worker, sem exigir intervenção manual.
