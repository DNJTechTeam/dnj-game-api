// Cliente HTTP compartilhado: espelha o comportamento do front (client.ts) no que
// importa para carga — timeout de 10 s, bearer, e correlação por X-Request-ID.
//
// Cuidados específicos da API DNJ:
//   - NÃO enviar header Origin: o CORS (gin-contrib/cors) aborta 403 fora da allowlist.
//   - NÃO poluir a query string: a API usa allowlist estrita (param extra -> 400).
//   - X-Request-ID precisa casar ^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$ e é ecoado no
//     log http_request_completed -> usamos um id determinístico com prefixo "k6-"
//     para correlação 1:1 no CloudWatch Logs Insights.
import http from 'k6/http';
import { Trend, Counter, Rate } from 'k6/metrics';
import crypto from 'k6/crypto';

// UUID v4 self-contained (sem import remoto) para a suíte rodar offline.
function uuidv4() {
  const b = new Uint8Array(crypto.randomBytes(16));
  b[6] = (b[6] & 0x0f) | 0x40;
  b[8] = (b[8] & 0x3f) | 0x80;
  const h = [];
  for (let i = 0; i < 16; i++) h.push((b[i] + 0x100).toString(16).slice(1));
  return `${h[0]}${h[1]}${h[2]}${h[3]}-${h[4]}${h[5]}-${h[6]}${h[7]}-${h[8]}${h[9]}-${h[10]}${h[11]}${h[12]}${h[13]}${h[14]}${h[15]}`;
}

export const BASE_URL = (__ENV.K6_BASE_URL ||
  'https://ttwkfudhvvhuhp5yvsoydxggum0ictpg.lambda-url.sa-east-1.on.aws/v2').replace(/\/+$/, '');

const REQUEST_TIMEOUT = __ENV.K6_REQUEST_TIMEOUT || '10s';

// Métricas custom por rota (route = template, ex. /game/overview).
const durByRoute = new Trend('dnj_route_duration', true);
const reqByRoute = new Counter('dnj_route_reqs'); // contagem por rota (Trend não expõe count)
const serverErrors = new Counter('dnj_server_errors'); // 5xx + transporte
const clientErrors = new Counter('dnj_client_errors'); // 4xx inesperados
const byStatus = new Counter('dnj_by_status'); // contagem por código HTTP (auditoria)
const timeouts = new Rate('dnj_timeout_rate');

// requestId determinístico e correlacionável.
export function makeRequestId(scenario, seq) {
  const vu = (typeof __VU === 'number' ? __VU : 0);
  const iter = (typeof __ITER === 'number' ? __ITER : 0);
  return `k6-${scenario}-${vu}-${iter}-${seq || 0}`;
}

function baseHeaders(token, requestId, extra) {
  const h = {
    Accept: 'application/json',
    'X-Request-ID': requestId,
  };
  if (token) h.Authorization = `Bearer ${token}`;
  return Object.assign(h, extra || {});
}

// GET com tag de rota. `expected` lista status extras aceitáveis (ex.: 204, 404).
export function get(route, path, opts) {
  opts = opts || {};
  const requestId = makeRequestId(opts.scenario || 'na', opts.seq);
  const params = {
    headers: baseHeaders(opts.token, requestId, opts.headers),
    timeout: REQUEST_TIMEOUT,
    tags: { route: route, scenario: opts.scenario || 'na' },
  };
  // Só sobrescreve o responseCallback padrão do k6 quando explicitamente dado;
  // passar undefined desligaria o http_req_failed (e o abort de segurança).
  if (opts.responseCallback) params.responseCallback = opts.responseCallback;
  // Opt-in a receber o corpo quando o cenário precisa parsear (feed/notifications).
  // discardResponseBodies é ligado globalmente no main.js para poupar RAM.
  if (opts.responseType) params.responseType = opts.responseType;
  const res = http.get(`${BASE_URL}${path}`, params);
  record(route, res, opts.expected);
  return res;
}

// POST idempotente: sempre com Idempotency-Key UUID único por chamada.
export function postIdempotent(route, path, body, opts) {
  opts = opts || {};
  const requestId = makeRequestId(opts.scenario || 'na', opts.seq);
  const headers = baseHeaders(opts.token, requestId, opts.headers);
  headers['Idempotency-Key'] = opts.idempotencyKey || uuidv4();
  if (body !== undefined && body !== null) headers['Content-Type'] = 'application/json';
  const params = {
    headers: headers,
    timeout: REQUEST_TIMEOUT,
    tags: { route: route, scenario: opts.scenario || 'na' },
  };
  if (opts.responseCallback) params.responseCallback = opts.responseCallback;
  const res = http.post(`${BASE_URL}${path}`, body === undefined ? null : body, params);
  record(route, res, opts.expected);
  return res;
}

function record(route, res, expected) {
  durByRoute.add(res.timings.duration, { route: route });
  reqByRoute.add(1, { route: route });
  const status = res.status;
  const ok = status >= 200 && status < 400;
  const allowed = expected && expected.indexOf(status) !== -1;
  byStatus.add(1, { code: String(status) });
  timeouts.add(status === 0);
  if (status === 0 || status >= 500) {
    serverErrors.add(1, { route: route });
  } else if (!ok && !allowed) {
    clientErrors.add(1, { route: route });
  }
}
