// Cenário spike coordenado: "todo mundo abre o Game ao mesmo tempo".
//
// Executor arrival-rate no main.js dispara este código num startTime específico,
// simulando o instante em que uma atividade abre e milhares de participantes
// entram na tela game simultaneamente. Cada iteração = 1 abertura de Game:
// os 3 GETs de mount (game/overview é o endpoint MAIS pesado da API — 6 queries,
// 4 window-function scans + ledger sem LIMIT).
import { get } from '../lib/http.js';
import { ROUTES as R } from '../lib/routes.js';
import { signIdentityToken } from '../lib/jwt.js';
import { defaultUsers, pickUser } from '../lib/users.js';

const SECRET = __ENV.JWT_IDENTITY_SECRET || '';
const TOKEN_TTL = parseInt(__ENV.K6_TOKEN_TTL_SECONDS || '10800', 10);
const SCENARIO = 'spike';

let vuToken = null;

export function spike() {
  if (!vuToken) {
    if (!SECRET) throw new Error('JWT_IDENTITY_SECRET ausente.');
    vuToken = signIdentityToken(pickUser(__VU, defaultUsers).id, SECRET, TOKEN_TTL);
  }
  const opts = { token: vuToken, scenario: SCENARIO };
  get(R.gameOverview.template, R.gameOverview.path, { ...opts, seq: 0 });
  get(R.currentParticipation.template, R.currentParticipation.path, { ...opts, seq: 1, expected: [204] });
  get(R.currentRun.template, R.currentRun.path, { ...opts, seq: 2, expected: [204] });
}
