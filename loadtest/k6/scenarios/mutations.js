// Cenário de mutações idempotentes leves.
//
// Escopo autorizado pelo usuário: apenas toggle de like e mark-read, ambos com
// Idempotency-Key UUID único por chamada. NÃO inclui QR (bloqueio de 10 min/usuário)
// nem upload de mídia. Cada iteração:
//   1. GET /moments?scope=feed  -> pega um momentId real da página.
//   2. POST /moments/:id/likes  -> toggle (200). Como é toggle, alternamos para
//      não deixar estado sujo acumulado: like numa iteração, unlike na seguinte.
//   3. GET /notifications -> pega um id não lido, se houver.
//   4. POST /notifications/:id/read -> marca como lida (idempotente).
import { get, postIdempotent } from '../lib/http.js';
import { ROUTES as R } from '../lib/routes.js';
import { signIdentityToken } from '../lib/jwt.js';
import { defaultUsers, pickUser } from '../lib/users.js';

const SECRET = __ENV.JWT_IDENTITY_SECRET || '';
const TOKEN_TTL = parseInt(__ENV.K6_TOKEN_TTL_SECONDS || '10800', 10);
const SCENARIO = 'mutations';

let vuToken = null;

function firstId(res, key) {
  try {
    const body = res.json();
    const arr = (body && (body.items || body.data)) || [];
    if (arr.length && arr[0] && arr[0].id !== undefined) return String(arr[0].id);
  } catch (e) {
    /* corpo vazio/erro -> sem id */
  }
  return null;
}

export function mutations() {
  if (!vuToken) {
    if (!SECRET) throw new Error('JWT_IDENTITY_SECRET ausente.');
    vuToken = signIdentityToken(pickUser(__VU, defaultUsers).id, SECRET, TOKEN_TTL);
  }
  const opts = { token: vuToken, scenario: SCENARIO };

  // Like (toggle) — precisa parsear o corpo, então não descartamos aqui.
  const feed = get(R.momentsFeed.template, R.momentsFeed.path, { ...opts, seq: 0, responseType: 'text' });
  const momentId = firstId(feed, 'items');
  if (momentId) {
    postIdempotent('/v2/moments/:momentId/likes', `/moments/${momentId}/likes`, null, {
      ...opts,
      seq: 1,
    });
  }

  // Mark-read — body vazio obrigatório.
  const notifs = get(R.notifications.template, R.notifications.path, { ...opts, seq: 2, responseType: 'text' });
  const notifId = firstId(notifs, 'data');
  if (notifId) {
    postIdempotent('/v2/notifications/:notificationId/read', `/notifications/${notifId}/read`, null, {
      ...opts,
      seq: 3,
    });
  }
}
