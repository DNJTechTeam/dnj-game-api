// Cenário gestor (EVENT_MANAGER): manager/game-overview a cada 15 s.
// Requer identidades com role EVENT_MANAGER em data/users.json; se não houver,
// o cenário se auto-desabilita (VU encerra sem gerar carga).
import { sleep } from 'k6';
import { get } from '../lib/http.js';
import { ROUTES as R } from '../lib/routes.js';
import { signIdentityToken } from '../lib/jwt.js';
import { managerUsers, pickUser } from '../lib/users.js';

const SECRET = __ENV.JWT_IDENTITY_SECRET || '';
const TOKEN_TTL = parseInt(__ENV.K6_TOKEN_TTL_SECONDS || '10800', 10);
const SCENARIO = 'manager';

let vuToken = null;
let disabled = false;

export function manager() {
  if (disabled) return;
  if (!vuToken) {
    const pool = managerUsers;
    if (!pool.length) {
      disabled = true;
      return;
    }
    vuToken = signIdentityToken(pickUser(__VU, pool).id, SECRET, TOKEN_TTL);
  }
  get(R.managerOverview.template, R.managerOverview.path, { token: vuToken, scenario: SCENARIO, seq: 0 });
  sleep(15);
}
