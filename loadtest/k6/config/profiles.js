// Perfis de carga selecionáveis por env PROFILE. Cada perfil define os `scenarios`
// do k6 (executors coordenados por startTime) e os `thresholds`.
//
// Convenções herdadas de docs/game-frontend-handoff.md:468-510 e docs/load-testing.md:
//   - smoke  : validação segura (poucos VUs, 2 min, zero 5xx tolerado).
//   - soak   : carga sustentada realista.
//   - spike  : pico coordenado (todos abrindo o Game).
//   - full   : >= 5000 VUs participantes + telão + gestor + spike + mutações, coordenados.
//
// Overrides por env: K6_PARTICIPANTS, K6_HOLD, K6_RAMP, K6_DISPLAYS, K6_MANAGERS,
// K6_SPIKE_VUS, K6_MUT_RATE.
import { ROUTES } from '../lib/routes.js';

const num = (v, d) => (v !== undefined && v !== '' ? parseInt(v, 10) : d);

// Lista única de templates de rota exercitados (para sub-métricas por rota).
const ROUTE_TEMPLATES = Array.from(
  new Set(Object.values(ROUTES).map((r) => r.template).concat([
    '/v2/moments/:momentId/likes',
    '/v2/notifications/:notificationId/read',
  ])),
);

const PARTICIPANTS = num(__ENV.K6_PARTICIPANTS, null);
const DISPLAYS = num(__ENV.K6_DISPLAYS, null);
const MANAGERS = num(__ENV.K6_MANAGERS, null);
const SPIKE_VUS = num(__ENV.K6_SPIKE_VUS, null);
const MUT_RATE = num(__ENV.K6_MUT_RATE, null);
const HOLD = __ENV.K6_HOLD || null;
const RAMP = __ENV.K6_RAMP || null;

// Thresholds base. `abortOnFail` derruba o teste cedo se estourar (critérios de
// abort de docs/game-frontend-handoff.md:501-509: p99 > timeout, erro > 5%).
function thresholds(errRatePct, hardServerErrors) {
  const t = {
    // p99 nunca deve encostar no timeout de 10 s do cliente.
    http_req_duration: ['p(95)<3000', `p(99)<8000`],
    dnj_timeout_rate: ['rate<0.02'],
    // taxa de erro agregada (checks) — abort se disparar de forma sustentada.
    // K6_NO_ABORT=1 desliga o abort (para achar o joelho vendo o regime estável).
    http_req_failed:
      __ENV.K6_NO_ABORT === '1'
        ? [`rate<${errRatePct / 100}`]
        : [{ threshold: `rate<${errRatePct / 100}`, abortOnFail: true, delayAbortEval: '30s' }],
  };
  if (hardServerErrors === 0) {
    t.dnj_server_errors = [{ threshold: 'count<1', abortOnFail: true, delayAbortEval: '15s' }];
  }
  // Sub-métricas por rota: threshold sempre-verdadeiro força o k6 a incluir
  // p50/p95/p99 por rota no objeto de handleSummary (base das tabelas do HTML).
  for (const tpl of ROUTE_TEMPLATES) {
    t[`dnj_route_duration{route:${tpl}}`] = ['max>=0'];
    t[`dnj_route_reqs{route:${tpl}}`] = ['count>=0'];
  }
  // Sub-métricas por código HTTP: expõem o breakdown exato no summary/relatório
  // (ex.: quantos 429 de throttle vs 500 vs 0/timeout).
  for (const code of ['0', '200', '204', '400', '401', '403', '404', '409', '429', '500', '502', '503', '504']) {
    t[`dnj_by_status{code:${code}}`] = ['count>=0'];
  }
  return t;
}

function participantScenario(vus, ramp, hold, startTime) {
  return {
    executor: 'ramping-vus',
    exec: 'participant',
    startTime: startTime || '0s',
    startVUs: 0,
    stages: [
      { duration: ramp, target: vus },
      { duration: hold, target: vus },
      { duration: '30s', target: 0 },
    ],
    gracefulStop: '15s',
    gracefulRampDown: '15s',
    tags: { scenario: 'participant' },
  };
}

function displayScenario(vus, ramp, hold) {
  return {
    executor: 'constant-vus',
    exec: 'display',
    vus: vus,
    duration: hold,
    startTime: '0s',
    gracefulStop: '15s',
    tags: { scenario: 'display' },
  };
}

function managerScenario(vus, hold) {
  return {
    executor: 'constant-vus',
    exec: 'manager',
    vus: vus,
    duration: hold,
    startTime: '0s',
    gracefulStop: '15s',
    tags: { scenario: 'manager' },
  };
}

// Spike coordenado: começa depois do ramp do baseline, num instante fixo.
function spikeScenario(peakVus, startTime) {
  return {
    executor: 'ramping-arrival-rate',
    exec: 'spike',
    startTime: startTime,
    startRate: 0,
    timeUnit: '1s',
    preAllocatedVUs: peakVus,
    maxVUs: peakVus,
    stages: [
      { duration: '10s', target: peakVus }, // sobe o pico em 10 s
      { duration: '20s', target: peakVus }, // segura
      { duration: '10s', target: 0 },
    ],
    tags: { scenario: 'spike' },
  };
}

function mutationsScenario(rate, hold, startTime) {
  return {
    executor: 'constant-arrival-rate',
    exec: 'mutations',
    rate: rate,
    timeUnit: '1s',
    duration: hold,
    startTime: startTime || '30s',
    preAllocatedVUs: Math.max(50, rate * 4),
    maxVUs: Math.max(100, rate * 10),
    tags: { scenario: 'mutations' },
  };
}

const PROFILES = {
  // Validação segura antes de qualquer carga real.
  smoke: () => ({
    scenarios: {
      participant: participantScenario(PARTICIPANTS ?? 50, RAMP || '20s', HOLD || '1m30s'),
      display: displayScenario(DISPLAYS ?? 2, '5s', HOLD || '1m30s'),
    },
    thresholds: thresholds(1, 0),
  }),

  // Carga sustentada realista (equivalente ao "develop soak" documentado, ampliado).
  soak: () => ({
    scenarios: {
      participant: participantScenario(PARTICIPANTS ?? 1500, RAMP || '3m', HOLD || '20m'),
      display: displayScenario(DISPLAYS ?? 8, '5s', HOLD || '20m'),
      manager: managerScenario(MANAGERS ?? 10, HOLD || '20m'),
      mutations: mutationsScenario(MUT_RATE ?? 20, HOLD || '18m', '2m'),
    },
    thresholds: thresholds(2, null),
  }),

  // Pico coordenado sobre um baseline moderado.
  spike: () => ({
    scenarios: {
      participant: participantScenario(PARTICIPANTS ?? 1000, RAMP || '1m', HOLD || '5m'),
      spike: spikeScenario(SPIKE_VUS ?? 800, '1m30s'),
    },
    thresholds: thresholds(5, null),
  }),

  // Teste alvo: >= 5000 participantes + telão + gestor + mutações + pico coordenado.
  full: () => ({
    scenarios: {
      participant: participantScenario(PARTICIPANTS ?? 5000, RAMP || '5m', HOLD || '20m'),
      display: displayScenario(DISPLAYS ?? 10, '5s', HOLD || '25m'),
      manager: managerScenario(MANAGERS ?? 20, HOLD || '25m'),
      mutations: mutationsScenario(MUT_RATE ?? 30, HOLD || '20m', '6m'),
      // Pico coordenado disparado depois que os 5000 já estão no ar.
      spike: spikeScenario(SPIKE_VUS ?? 1500, '12m'),
    },
    thresholds: thresholds(5, null),
  }),
};

export function buildOptions() {
  const name = (__ENV.PROFILE || 'smoke').toLowerCase();
  const factory = PROFILES[name];
  if (!factory) {
    throw new Error(`PROFILE inválido: "${name}". Use smoke|soak|spike|full.`);
  }
  const cfg = factory();
  return {
    profileName: name,
    scenarios: cfg.scenarios,
    thresholds: cfg.thresholds,
    discardResponseBodies: true, // cenários que precisam do corpo usam responseType:'text'.
    noConnectionReuse: false,
    userAgent: 'dnj-k6-loadtest/1.0',
  };
}
