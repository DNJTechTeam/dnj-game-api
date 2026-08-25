# Handoff incremental do frontend — jogos, ranking, mídia e Moments (Iterações 6–7)

O clone do frontend foi consultado somente como fonte de descoberta. Nenhum
arquivo dele deve ser alterado, commitado ou enviado por este trabalho.

## Consumidores atuais e rota de migração

| Tela/módulo atual | Rota atual | Rota V2 | Ajuste obrigatório |
|---|---|---|---|
| `src/features/game/game-screen.tsx` | `/api/v1/game/overview` | `GET /v2/game/overview` | remover `isUser`/`group`; usar `current` e `groupName` |
| mesma tela | `/api/v1/activity-runs/current` | `GET /v2/activity-runs/current` | tratar 204 e polling terminal |
| mesma tela e galeria | `/api/v1/participations/current` | `GET /v2/participations/current` | manter 204; não iniciar mídia na Iteração 6 |
| `src/features/scanner/qr-scanner-modal.tsx` | `/api/v1/qr/validate` com chave no corpo | `POST /v2/qr/validate` | enviar chave no header e corpo somente `{qrToken}` |
| `src/components/manager/manager-dashboard.tsx` | `/api/manager/overview` | `GET /v2/manager/game-overview` | preservar shape `scope:actions` |
| painel/Route Handlers `src/app/api/manager/actions/**` | ações legadas com `runId` no corpo | `/v2/manager/runs*` | mover runId para path e usar uma chave por intenção |
| ranking/display e leituras diretas | payloads agregados/Supabase | `GET /v2/rankings` | separar `individual` e `groups`; usar `position` do servidor |
| `src/features/moments/moment-composer.tsx` | `POST /api/v1/moments` multipart (`participationId,image,publishConsent,idempotencyKey`) | intenção + PUT direto ao S3 + `POST /v2/media/{id}/complete` + `POST /v2/moments` | quatro chamadas em vez de uma; publishConsent decide `private`/`public`, nunca decide pontos sozinho — pontos vêm só de challenge público |
| `src/features/gallery/gallery-screen.tsx` (abas feed/mine/group) | `GET /api/v1/moments?scope=` | `GET /v2/moments?scope=&cursor=` | `scope` passa a ser obrigatório e exatamente um valor; paginação por `cursor` opaco em vez de `nextCursor:null` fixo |
| mesma tela, botão de curtir | `POST /api/v1/gallery/{id}/likes` sem corpo, sem idempotência | `POST /v2/moments/{momentId}/likes` | exigir `Idempotency-Key` UUID por toque; resposta troca de `{ok:true}` para `{momentId,liked,likesCount}` |
| `src/app/api/v1/media/[...storageKey]/route.ts` (proxy de leitura) | GET direto ao Storage por `storageKey` | consumir `imageUrl`/`thumbnailUrl`/`shareImageUrl` já assinados retornados pela API | remover o proxy; as três URLs apontam para a mesma versão sanitizada nesta iteração e expiram em até 5 minutos — não cachear além disso nem reescrever a URL |
| `src/components/admin/admin-dashboard.tsx` + `src/app/api/admin/moderation/route.ts` | `GET/PATCH /api/admin/moderation` com `reason` opcional | `GET /v2/admin/moments/moderation?queue=&page=` + `POST /v2/admin/moments/{momentId}/moderation` | `queue` obrigatória (`general`/`challenge`); `PATCH` vira `POST` idempotente; **campo `reason` deixa de existir** — a V2 não aceita motivo livre |
| `src/types/experience.ts` (`Moment`, `GalleryPage`) | campos `comments: []`, `removalReason` | `Moment` da V2 (sem `comments`; `moderationMessage` já vem derivada) | remover `comments` do tipo; `moderationMessage` substitui qualquer leitura de `removal_reason` no cliente |
| `src/lib/repositories/experience-repositories.ts` | queries Supabase diretas a `moments`/`media_objects`/`moment_likes` | repositórios V2 via `apiV2` | nenhuma leitura/escrita direta ao Supabase Storage ou tabelas de mídia a partir do cliente |
| `src/lib/moments/moderation.ts` (`ModerationAction`) | `"approve" \| "deny_points" \| "delete_photo"` | `"deny_points" \| "delete_photo"` | remover a ação `approve` do tipo — Moments entram `approved` por padrão na V2, sem fila de aprovação inicial |

Criação/edição de Activity competitive continua no contrato administrativo da
Iteração 4. A Iteração 6 não oferece CRUD de `Game` porque Game é projeção. A
Iteração 7 não oferece CRUD de evento: a instalação permanece de edição única.

## Grafo de requests — abertura do Game

```text
abrir tela Game
  ├─ GET /v2/game/overview              [auth, obrigatório]
  ├─ GET /v2/participations/current     [auth, 200|204]
  └─ GET /v2/activity-runs/current      [auth, 200|204]
       └─ se run aberta: iniciar polling por runId
```

Fan-out inicial: três requests paralelos, uma vez por abertura. Dependências:
sessão válida e onboarding completo. Deduplicar por chave de cache durante a
montagem; cancelar as três com `AbortController` ao desmontar; timeout sugerido
8 s. Um 401 dispara no máximo um refresh compartilhado e uma repetição. 409
`ONBOARDING_REQUIRED` navega para onboarding; não aplicar retry automático.

Orçamento: 3 requests por abertura; mais 1 refresh apenas quando o access token
expirar. Não buscar `/rankings` adicional quando `/game/overview` já atende a
tela autenticada.

## Grafo de requests — scan

```text
leitura estável da câmera
  └─ POST /v2/qr/validate
       header Idempotency-Key: UUID estável desta leitura/intenção
       body {qrToken}
       ├─ 201|200 → renderizar participação; GET current run opcional
       ├─ 409 QR_UNAVAILABLE → mensagem neutra e permitir nova leitura
       └─ 410 QR_EXPIRED → mensagem de expiração e fechar tentativa
```

Orçamento: 1 POST por leitura lógica e, quando necessário para atualizar UI,
1 GET de run. `busyRef` já impede leituras simultâneas; a chave deve nascer
antes do POST e sobreviver a timeout/reconexão. Retry somente para timeout,
rede, 429 futuro ou 5xx, com a mesma chave, no máximo 3 tentativas em
0,5 s/1 s/2 s com jitter de ±20%. Nunca colocar a chave ou QR em URL/log.

```ts
async function validateQr(qrToken: string, signal: AbortSignal) {
  const key = crypto.randomUUID();
  return apiV2("/qr/validate", {
    method: "POST",
    signal,
    headers: { "Idempotency-Key": key },
    body: JSON.stringify({ qrToken }),
  });
}
```

## Grafos de requests — gestor

```text
abrir dashboard
  ├─ obter/renovar sessão V2
  └─ GET /v2/manager/game-overview
       └─ se actions.run != null: polling do mesmo overview

abrir partida(gameId)
  └─ POST /v2/manager/runs {gameId}
       └─ POST /v2/manager/runs/{runId}/qr [corpo vazio]

rotacionar QR
  └─ POST /v2/manager/runs/{runId}/qr [corpo vazio]

iniciar | pausar | retomar
  └─ POST /v2/manager/runs/{runId}/{start|pause|resume} [corpo vazio]

apurar/finalizar
  └─ POST /v2/manager/runs/{runId}/results {results:[...]}
       └─ resposta completed → parar polling e invalidar overview/rankings

cancelar
  └─ POST /v2/manager/runs/{runId}/cancel [corpo vazio]
       └─ resposta cancelled → parar polling
```

Cada POST usa um UUID novo por clique/intenção e reutiliza a mesma chave em
retry. Desabilitar o botão enquanto a intenção estiver em voo. 409 não deve ser
repetido automaticamente: atualizar overview uma vez e reconciliar a UI.
Orçamento por ação: um POST e no máximo um GET de reconciliação; criação com QR
consome dois POSTs porque são intenções distintas.

O backend retorna `qrToken`, não imagem/URL. O frontend deve gerar o bitmap
localmente (biblioteca QR já existente) sem enviar o token a terceiros e
descartá-lo ao sair de `draft` ou atingir `expiresAt`.

## Grafo de requests — abrir a galeria (três abas)

```text
abrir galeria (aba ativa = feed|mine|group)
  └─ GET /v2/moments?scope={aba}                [auth, obrigatório]
       └─ até 20 itens; cada item já traz imageUrl/thumbnailUrl/shareImageUrl assinados
            (nenhum GET adicional ao Storage; o `<img>`/`NextImage` busca a URL assinada
             diretamente do provedor, fora da API — até 20 GETs de objeto, não de API)
trocar de aba
  └─ repete GET /v2/moments?scope={nova aba}     [nova requisição; não reaproveitar cursor de outra aba]
```

Orçamento por abertura de aba: **uma chamada à API**, mais até 20 GETs de objeto
ao provedor de storage (não à API V2) para renderizar as miniaturas — conforme
o orçamento da Iteração 7. Deduplicar por `(scope, cursor)`: trocar de aba ou
remontar com o mesmo par não deve emitir uma segunda chamada em voo.
`AbortController` por aba ao desmontar ou trocar de aba antes da resposta.
Nunca persistir `imageUrl`/`thumbnailUrl`/`shareImageUrl` em cache além da
sessão da tela — elas expiram em até 5 minutos; ao reabrir a galeria depois
desse intervalo, refazer o `GET` em vez de reutilizar a URL antiga.

## Grafo de requests — abrir o composer (a partir do Game e da Galeria)

```text
abrir composer (participation já resolvida pela tela de origem)
  └─ nenhuma chamada de rede até o usuário capturar/selecionar a foto
     (câmera é local; getUserMedia não toca a API)
```

O composer não muda de contrato conforme é aberto a partir do Game (moment
challenge, `participationId` presente) ou da Galeria (moment free, sem
`participationId`). A única diferença é se `POST /v2/moments` recebe
`participationId` ou não.

## Grafo de requests — cálculo de checksum e upload em quatro etapas

```text
usuário confirma a foto (arquivo local, já capturado/selecionado)
  1. calcular SHA-256 do arquivo localmente (crypto.subtle.digest, sem rede)
       └─ falha ao calcular → mensagem "não foi possível preparar a imagem", sem chamada
  2. POST /v2/media/upload-intents                          [auth]
       header Idempotency-Key: UUID novo desta intenção
       body {contentType, bytes, checksumSha256}
       ├─ 201 → {id, uploadUrl, method:"PUT", headers, expiresAt}
       ├─ 413 IMAGE_TOO_LARGE → abortar antes de qualquer upload
       ├─ 415 UNSUPPORTED_MEDIA_TYPE → abortar antes de qualquer upload
       └─ 503 MEDIA_UNAVAILABLE → "mídia indisponível no momento", permitir tentar de novo
  3. PUT {uploadUrl}                                         [direto ao provedor, headers assinados do passo 2]
       ├─ sucesso (2xx) → seguir para o passo 4
       ├─ falha de rede/5xx → retry do PUT com o mesmo uploadUrl enquanto expiresAt não passou
       └─ expiresAt já passado → descartar a intenção; voltar ao passo 2 com uma chave NOVA
  4. POST /v2/media/{id}/complete                            [auth, sem corpo]
       header Idempotency-Key: mesma chave do passo 2 (é a confirmação da mesma intenção)
       ├─ 200 → {id, contentType, bytes, state:"available", availableAt, retentionDueAt}
       ├─ 409 UPLOAD_INCOMPLETE → o PUT ainda não chegou ao provedor; esperar e tentar de novo com a MESMA chave
       ├─ 409 UPLOAD_STATE_CONFLICT → outra confirmação está em voo ou o asset já mudou de estado; não reemitir
       ├─ 410 UPLOAD_EXPIRED → a intenção expirou; descartar e voltar ao passo 2 com chave nova
       ├─ 413/422 → arquivo rejeitado na sanitização; descartar e pedir nova foto (não reusar o mesmo arquivo)
       └─ 503 MEDIA_UNAVAILABLE → permitir tentar novamente com a MESMA chave
  5. POST /v2/moments                                        [auth]
       header Idempotency-Key: UUID novo (é uma intenção distinta da de upload)
       body {mediaAssetId: id do passo 4, publishConsent, participationId?}
       └─ 201|200 → Moment publicado; invalidar galeria da aba afetada e, se challenge público, o overview
```

Orçamento total de uma publicação bem-sucedida: **intenção (1) + PUT (1) +
confirmação (1) + criação do Moment (1)** — quatro chamadas de rede, mais o
cálculo local de checksum. Nenhuma chamada extra de leitura é necessária: a
resposta de `POST /v2/moments` já contém tudo que a UI precisa para renderizar
o Moment recém-criado.

### Cancelamento do PUT

Se o usuário fechar o composer ou trocar de foto enquanto o PUT do passo 3
está em voo, cancelar via `AbortController` no `fetch`/`XMLHttpRequest` do PUT.
Não chamar o passo 4 (`complete`) para uma intenção cujo PUT foi cancelado —
a próxima tentativa deve recomeçar do passo 2 com uma chave nova, já que o
objeto pode ter ficado parcialmente enviado no provedor.

### Retry após falha de rede

- PUT (passo 3): retry com a mesma `uploadUrl`, até 3 tentativas, backoff
  0,5 s/1 s/2 s com jitter ±20%, somente enquanto `expiresAt` não passou.
- complete (passo 4): retry com a mesma `Idempotency-Key`, mesmo padrão de
  backoff; um 409 `UPLOAD_INCOMPLETE` é esperado se o retry chega antes do
  provedor propagar o objeto — não é erro do usuário.
- Qualquer novo arquivo/nova foto exige voltar ao passo 2 com uma
  `Idempotency-Key` nova; nunca reutilizar uma chave para um payload diferente
  (a API responde 409 `IDEMPOTENCY_KEY_REUSED`).

### Intent expirada e confirmação retomada

```text
usuário retoma o app depois de minutos (background, troca de rede)
  └─ POST /v2/media/{id}/complete com a Idempotency-Key original
       ├─ ainda dentro da janela → 200 e segue normalmente
       └─ expirou → 410 UPLOAD_EXPIRED → limpar estado local da intenção e
            reiniciar do passo 2 (nova intenção, nova chave); a foto local
            capturada pode ser reaproveitada se ainda estiver em memória/disco
            do dispositivo, mas a intenção e a staging key não podem
```

### Publicação e atualização da galeria/ranking

Após `POST /v2/moments` responder com sucesso, invalidar **no máximo uma vez**
a query da aba de galeria afetada (`mine` sempre; `feed`/`group` só se
`publishConsent=true`) e, quando o Moment for challenge público, o overview do
Game (pontos podem ter mudado). Não fazer polling da galeria à espera do novo
Moment — a resposta do `POST` já é a fonte da verdade.

## Grafo de requests — like

```text
tocar em curtir
  └─ POST /v2/moments/{momentId}/likes           [auth, sem corpo]
       header Idempotency-Key: UUID novo por toque
       ├─ 200 → {momentId, liked, likesCount}; atualizar o item na lista em memória
       ├─ 404 → Moment deixou de ser visível (moderado/removido); remover da lista local
       └─ 409 → ONBOARDING_REQUIRED/IDEMPOTENCY_KEY_REUSED; não reemitir automaticamente
```

Orçamento: **uma chamada por toque**. `busyRef`/estado local desabilita o
botão enquanto a intenção estiver em voo, evitando o duplo-toque gerar duas
chaves. Toques concorrentes em Moments diferentes usam chaves independentes e
podem estar em voo ao mesmo tempo. Otimista: pode alternar o coração
imediatamente na UI e reconciliar com a resposta; em erro, reverter ao estado
anterior.

## Grafo de requests — detalhe e compartilhamento

```text
abrir detalhe de um Moment (já presente na lista carregada)
  └─ nenhuma chamada adicional — usa os dados já carregados pela lista

compartilhar
  └─ fetch(imageUrl-já-assinada) [ao provedor, não à API V2]
       └─ compor imagem localmente (canvas) e acionar o share sheet nativo
```

O compartilhamento não deve refazer `GET /v2/moments`; deve usar a
`shareImageUrl` já obtida na última listagem. Se essa URL já expirou (mais de
5 minutos desde o carregamento da lista), refazer o `GET` da aba antes de
tentar compartilhar, em vez de tentar o `fetch` direto contra uma URL vencida.

## Grafo de requests — moderação corretiva (admin)

```text
abrir fila (queue=general|challenge)
  └─ GET /v2/admin/moments/moderation?queue={queue}&page=   [auth ADMIN]
       └─ envelope {data:[...50 no máximo], pagination}

aplicar deny_points
  └─ POST /v2/admin/moments/{momentId}/moderation
       header Idempotency-Key: UUID novo por decisão
       body {action:"deny_points"}                          — SEM campo reason
       ├─ 200 → remover/atualizar o item na fila local (fora de general/challenge após a decisão)
       └─ 409 MODERATION_ACTION_INVALID → Moment não tinha premiação; mostrar estado atual, não reemitir

aplicar delete_photo
  └─ POST /v2/admin/moments/{momentId}/moderation
       header Idempotency-Key: UUID novo por decisão
       body {action:"delete_photo"}                         — SEM campo reason
       └─ 200 → remover o item da fila local; a foto permanece preservada para o autor
```

Orçamento: **listagem (1) + uma chamada por decisão**. O campo `reason` livre
que existe hoje em `PATCH /api/admin/moderation` **não tem equivalente na
V2** — a tela precisa remover esse campo do formulário, não apenas trocar a
rota. Cada decisão usa uma `Idempotency-Key` própria; repetir a mesma decisão
com uma chave nova sobre um Moment já decidido retorna o estado terminal sem
novo efeito (seguro chamar de novo, mas não é necessário).

## Grafo de requests — notificações (Iteração 8)

```text
abrir lista de notificações
  └─ GET /v2/notifications?page=      [auth DEFAULT]
       └─ {data:[...10 por página], pagination, unreadCount}
            └─ usar unreadCount para o badge — não somar itens da página local

badge de não lidas (sem abrir a lista)
  └─ mesmo GET /v2/notifications acima; ler somente unreadCount
       — não existe endpoint dedicado só de contagem nesta iteração

marcar como lida (ao abrir o detalhe de uma notificação)
  └─ POST /v2/notifications/{notificationId}/read
       header Idempotency-Key: UUID novo por marcação    — SEM corpo
       ├─ 200 → atualizar o item local para state:"read"; decrementar o
       │        badge local em 1 SE o item ainda estava unread no cliente
       └─ 404 → notificação alheia ou removida; remover do estado local
                sem mostrar erro (não é enumerável)

abrir preferências
  └─ GET /v2/notifications/preferences   [auth DEFAULT]
       └─ {momentModerationEnabled:true (sempre), pointsEnabled, announcementEnabled, updatedAt}
            └─ o toggle de moderação/segurança nem deve ser renderizado como editável

atualizar preferências (toggle de pontos ou anúncios)
  └─ PUT /v2/notifications/preferences
       header Idempotency-Key: UUID novo por submissão
       body {pointsEnabled?, announcementEnabled?}        — NUNCA enviar momentModerationEnabled
       └─ 200 → refletir os valores retornados (não os enviados) como fonte da verdade
```

Orçamento: **abertura de lista/badge (1 GET) + uma chamada por marcação de
leitura + preferências sob demanda (1 GET ao abrir a tela, 1 PUT por
alteração)**. `momentModerationEnabled` é sempre `true` na resposta — a UI
deve exibi-lo como informativo/desabilitado, nunca como toggle editável, já
que qualquer tentativa de enviá-lo é rejeitada com `400`. `title`/`body` de
notificações administrativas (`announcement`) já vêm prontos do servidor;
não há template client-side. O envio administrativo
(`POST /v2/admin/notifications`) é uma tela exclusiva de ADMIN, fora do fluxo
do app do participante — sem componente de destinatários individuais, já que
a resposta expõe somente `recipientCount`.

## Refresh de URL assinada

`imageUrl`/`thumbnailUrl`/`shareImageUrl`/a `uploadUrl` da intenção de upload
expiram (5 minutos para leitura, 10 minutos para o upload). Nenhuma delas deve
ser persistida além da resposta que a originou — nem em cache de imagem do
navegador com TTL maior, nem em `localStorage`, nem em log/analytics. Ao
detectar uma URL de imagem vencida (erro de carregamento após o TTL), refazer
o `GET` da listagem correspondente em vez de tentar reconstituir a URL no
cliente.

## Desmontagem, re-render e reconexão

- Toda chamada desta seção usa `AbortController` amarrado ao ciclo de vida do
  componente que a disparou; desmontar cancela em voo.
- Re-render não deve reemitir `GET /v2/moments` para o mesmo `(scope,cursor)`
  já resolvido — deduplicar por essa chave, igual ao padrão já adotado para
  `game/overview`/`activity-runs/current`.
- Reconexão de rede durante um upload em andamento: se o PUT (passo 3) ainda
  não confirmou, retry do PUT; se já passou para `complete` (passo 4) e a
  conexão caiu antes da resposta, refazer `complete` com a MESMA chave — é
  seguro por design (claim/lease do servidor serializa a confirmação).
- Offline: a foto capturada localmente pode ser preservada em memória/disco
  do dispositivo somente com consentimento explícito do usuário (ex.: aviso
  "sem conexão — a foto ficará pendente até você reconectar"); a intenção de
  upload em si não deve ser criada offline, já que depende de uma resposta
  do servidor.
- Timeouts distintos: chamadas à API V2 (intent, complete, moments, likes,
  moderação) usam o timeout padrão de 8 s já adotado para o restante da V2;
  o PUT direto ao S3/MinIO usa um timeout maior e configurável
  separadamente (o objeto pode ter até 10 MiB e a rede do dispositivo pode
  ser mais lenta que a da API) — sugerido 30 s, reiniciado a cada retry.

## Polling seguro

O polling atual de 2 s pode multiplicar chamadas em re-render, remontagem,
reconexão e várias abas. A referência `overviewPollInFlight` evita sobreposição,
mas ainda faltam jitter, backoff e deduplicação compartilhada.

Política proposta:

1. Uma única chave de polling por `runId` no cliente, compartilhada por
   consumidores montados.
2. `AbortController` no unmount, troca de run, logout e aba oculta.
3. Intervalo base 2 s em `active/results`, 3 s em `draft`, 5 s em `paused`.
4. Jitter ±20%; em erro de rede/5xx, backoff 2/4/8/15 s, máximo 30 s.
5. Reconexão dispara uma leitura imediata deduplicada, não um novo timer.
6. Parar definitivamente em `completed` ou `cancelled` após uma última
   atualização de overview/ranking.
7. `visibilitychange`: pausar oculto; ao voltar, uma leitura imediata e retomada.

Orçamento: em uma run ativa, máximo nominal 30 GET/min por dashboard ou tela de
participante; pausado, 12 GET/min. Com deduplicação, re-render/remount não muda
esse número.

## Tipos e datas

```ts
type RankingPerson = {
  id: string; name: string; groupName: string | null;
  points: number; position: number;
};

type CurrentRun = {
  id: string;
  status: "draft" | "active" | "paused" | "results" | "completed" | "cancelled";
  gameName: string;
  startedAt: string | null;
  endedAt: string | null;
  result?: "first" | "second" | "third" | "participation";
  points?: number;
};

const localDateTime = (instant: string) =>
  new Intl.DateTimeFormat(undefined, { dateStyle: "short", timeStyle: "short" })
    .format(new Date(instant));
```

Não anexar `Z` a string local. O backend já envia UTC; `Date`/`Intl` devem usar
o fuso atual do dispositivo.

## Backlog incremental 2–6

| Prioridade | Item | Dependência/endpoints | Estado | Aceite e evidência esperada |
|---|---|---|---|---|
| P0 | cliente V2, refresh compartilhado, CSRF e erros | Iteração 2 | ready | teste prova um refresh para fan-out e abort no logout |
| P0 | sessão/onboarding/perfil sem campos legados | `/v2/auth*`, `/v2/users/me` | ready | testes 401/409 e atualização segura |
| P0 | grupos/membership/convites | Iteração 3 | ready | paginação e 204/404 conforme contrato |
| P0 | administração e assignments | Iteração 4 | ready | UUID de idempotência e UTC testados |
| P0 | agenda/Activities/favoritos | Iteração 5 | ready | rotas legadas removidas dos consumidores migrados |
| P0 | adaptar tipos do Game overview | `/v2/game/overview` | ready | usa `groupName`, `position`, `current.points`; snapshot visual/teste |
| P0 | migrar scan para header idempotente | `/v2/qr/validate` | ready | 200/201/409/410 e retry com mesma chave testados |
| P0 | migrar run/participação atual | dois GETs current | ready | 204, terminal por runId, abort e parada testados |
| P0 | migrar dashboard actions | `/v2/manager/game-overview`, `/runs*` | ready | ciclo completo e 409 reconciliado em teste |
| P1 | gerar QR local a partir de `qrToken` | resposta de rotate QR | ready | nenhum token em log/analytics; expiração visível |
| P1 | migrar display público de ranking | `/v2/rankings` | ready | alternância de scopes usa `position` do servidor |
| P1 | polling compartilhado/backoff/jitter | current run e manager overview | ready | fake timers provam orçamento/parada terminal |
| P2 | remover Route Handlers e mocks legados após rollout | todos acima | blocked | depende de telemetria e rollback aprovado |
| P0 | cliente de upload em 4 etapas (checksum local, intent, PUT, complete) | `/v2/media/upload-intents`, `/v2/media/{id}/complete` | ready | testes provam cancelamento do PUT, retry com mesma chave, intent expirada e confirmação retomada |
| P0 | migrar composer para `POST /v2/moments` após upload confirmado | `/v2/moments` | ready | free e challenge testados; nunca envia pontos/estado/ownership do cliente |
| P0 | migrar as três abas da galeria para `scope`/`cursor` | `GET /v2/moments` | ready | scope obrigatório único; cursor opaco testado com paginação determinística |
| P0 | migrar curtir para header idempotente | `POST /v2/moments/{id}/likes` | ready | 200/404/409 e duplo-toque testados com a mesma chave |
| P0 | remover proxy de mídia (`/api/v1/media/[...storageKey]`) e consumir URLs assinadas da API | resposta de `/v2/moments`, `/v2/media/{id}/complete` | ready | nenhuma leitura direta ao Storage a partir do cliente; TTL de 5 min respeitado |
| P1 | migrar dashboard de moderação e remover o campo `reason` do formulário | `/v2/admin/moments/moderation*` | ready | `deny_points`/`delete_photo` testados; formulário não envia `reason` |
| P1 | atualizar `src/types/experience.ts` (remover `comments`, ajustar `Moment`/`GalleryPage`) | contrato V2 | ready | typecheck e snapshot dos componentes de galeria/composer |
| P1 | invalidação pós-publicação (galeria + overview quando challenge público) | `/v2/moments`, `/v2/game/overview` | ready | teste prova no máximo uma invalidação de cada |
| P2 | remover `src/lib/moments/moderation.ts` ação `approve` e qualquer fila de aprovação prévia | tipos e UI de moderação | blocked | depende do rollout do item P1 de moderação |
| P2 | remover Route Handlers `v1/moments`, `v1/gallery*`, `v1/media/*`, `admin/moderation` legados | todos os itens P0/P1 acima | blocked | depende de telemetria e rollback aprovado |
| P0 | badge e lista de notificações | `GET /v2/notifications` | ready | `unreadCount` do servidor usado no badge; paginação de 10 testada |
| P0 | marcar notificação como lida | `POST /v2/notifications/{id}/read` | ready | idempotência e 404 uniforme (alheia/inexistente) testados |
| P1 | tela de preferências de notificação | `GET/PUT /v2/notifications/preferences` | ready | `momentModerationEnabled` renderizado como informativo, nunca editável; envio do campo é rejeitado em teste |
| P2 | tela administrativa de envio (ADMIN) | `POST /v2/admin/notifications` | ready | sem campo de destinatários individuais; só `recipientCount` exibido |

Rollback: manter o adaptador antigo atrás de flag durante o rollout, mas nunca
executar V1 e V2 mutantes em paralelo. Para recuar, parar novos POSTs, aguardar
intenções em voo, alternar a flag e preservar as chaves já emitidas até o fim
dos retries.

## Perfis futuros de carga — entrada da Iteração 9

Não executar stress amplo nesta iteração. Todos os perfis abaixo são
não destrutivos por padrão; cenários mutantes usam tenants/dados efêmeros
somente em ambiente autorizado.

| Ambiente/perfil | Concorrência | RPS/burst/duração | Timeout/limite/pool | Orçamento de erro |
|---|---:|---|---|---:|
| local smoke | 1–5 | 1–5 RPS, burst 10, 60 s | 8 s; sem rate limit; pool 10 | <1% |
| CI smoke reproduzível | 10 | 10 RPS, burst 20, 2 min | 5 s; pool 20 | 0,5% e nenhum 5xx |
| develop soak autorizado | 100 | 50 RPS, burst 200, 30 min | 8 s; limite candidato 120/min/identidade; pool 40 | <1% |
| develop spike autorizado | 250 | 200 RPS, burst 500, 5 min | 8 s; pool 60 | <2%, sem perda de efeito |
| produção canary read-only | 25 | 10 RPS, burst 25, 10 min | 5 s; respeitar limite publicado; pool existente | <0,5% |

Perfis por fluxo:

| Fluxo | Concorrência/RPS/burst | Duração | Medidas obrigatórias |
|---|---|---|---|
| cold start + health/readiness | 20 / 5 / 20 | 5 min | p50/p95/p99, init, erros |
| login/refresh fan-out | 50 / 20 / 100 | 10 min | refresh único, 401/403, conexões |
| abertura Game (3 GETs) | 100 / 100 / 300 | 15 min | p95, cache, pool, saturação |
| catálogo/rankings/overview | 100 / 75 / 200 | 30 min | query time, payload, CPU, p99 |
| polling de run | 250 clientes / 125 RPS / 250 | 30 min | dedupe, throttling, conexões |
| burst de scans | 200 / 100 / 400 | 5 min | unicidade, locks, 409/410, latência |
| finalização concorrente | 20 intenções / 10 RPS / 40 | 5 min | um ledger, rollback, deadlocks |
| atualização de ranking pós-finalização | 100 / 50 / 150 | 10 min | consistência saldo/ledger, p99 |
| retries e reconexão | 100 / 50 / 300 | 10 min | mesmos efeitos, backoff, 5xx |
| intenção de upload | 100 / 50 / 200 | 10 min | 413/415/503, p99, unicidade de staging key |
| PUT direto ao provedor | 50 / 25 / 100 | 10 min | banda, conexões ao S3/MinIO, taxa de sucesso |
| confirmação/sanitização | 50 / 20 / 100 | 15 min | duração da sanitização, memória, claim/lease sob concorrência |
| criação de Moment | 100 / 50 / 200 | 10 min | unicidade asset/Participation sob concorrência, p99 |
| abertura de feed | 200 clientes / 100 RPS / 300 | 15 min | p95, cursor determinístico, GETs de objeto ao provedor |
| burst de imagens (publicações simultâneas) | 100 / 50 / 300 | 5 min | locks, deadlocks, unicidade, backlog do worker |
| likes | 300 clientes / 150 RPS / 400 | 10 min | serialização por (momentId,userId), duplo-toque |
| moderação | 20 / 10 / 40 | 10 min | reversão atômica de pontos, ausência de dupla reversão |
| worker de retenção | contínuo | 30 min | pendentes/processando/expirados/falhos/retries, idade do job mais antigo |
| refresh simultâneo de URLs assinadas | 200 / 100 / 300 | 10 min | TTL respeitado, sem cache público, p99 |
| cold start / indisponibilidade do S3 | 20 / 5 / 20 | 5 min | 503 sem derrubar healthcheck/catálogo/auth, recuperação |

Para cada execução registrar commit, dataset não sensível, parâmetros, p50,
p95, p99, taxa de erro, throttling, conexões, pool, locks, retries transacionais,
CPU/memória e saturação do banco. Para os fluxos de mídia/Moments, registrar
também bytes transferidos, duração da sanitização (download+validação+
reencode), conexões e locks do provedor S3/MinIO, leases de claim/cleanup
abertos e o backlog do worker de retenção. Abort imediato se houver saldo
divergente, premiação duplicada, deadlock persistente, p99 acima do timeout,
erro >5%, staging key vazando ownership/PII ou URL assinada persistida além
do TTL documentado.
