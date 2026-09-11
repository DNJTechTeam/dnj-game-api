// Ponto de entrada da suíte k6. Seleciona o perfil por env PROFILE e coordena
// múltiplos executors em paralelo (participante + telão + gestor + mutações + spike).
//
// Uso típico (via Makefile na raiz do repo):
//   PROFILE=smoke make k6-run
//   PROFILE=full  make k6-run
// Ou direto:
//   k6 run -e PROFILE=smoke loadtest/k6/main.js
//
// Configuração e credenciais vêm de variáveis de ambiente (ver .env.k6.example):
//   K6_BASE_URL           URL da API V2 (default: Lambda develop).
//   JWT_IDENTITY_SECRET   segredo HS256 de develop, para forjar tokens.
//   K6_USERS_FILE         caminho do JSON de identidades (default ./data/users.json).
import { buildOptions } from './config/profiles.js';
import { BASE_URL } from './lib/http.js';
import { renderHTML } from './report/html.js';

// Reexporta as funções de cenário (o campo `exec` de cada scenario aponta para elas).
export { participant } from './scenarios/participant.js';
export { display } from './scenarios/display.js';
export { manager } from './scenarios/manager.js';
export { spike } from './scenarios/spike.js';
export { mutations } from './scenarios/mutations.js';

const built = buildOptions();
export const options = {
  scenarios: built.scenarios,
  thresholds: built.thresholds,
  discardResponseBodies: built.discardResponseBodies,
  noConnectionReuse: built.noConnectionReuse,
  userAgent: built.userAgent,
  summaryTrendStats: ['avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

const PROFILE = built.profileName;
const OUT_DIR = __ENV.K6_OUT_DIR || './out';

export function setup() {
  const startedAtUTC = new Date().toISOString();
  // eslint-disable-next-line no-console
  console.log(`[dnj-k6] perfil=${PROFILE} alvo=${BASE_URL} início(UTC)=${startedAtUTC}`);
  return { startedAtUTC };
}

export function handleSummary(data) {
  const now = new Date();
  const stamp = now.toISOString().replace(/[:.]/g, '-');
  const meta = {
    profile: PROFILE,
    baseUrl: BASE_URL,
    generatedAt: now.toISOString(),
    startedAtUTC: (data.setup_data && data.setup_data.startedAtUTC) || null,
    endedAtUTC: now.toISOString(),
    functionName: __ENV.K6_LAMBDA_FUNCTION_NAME || null,
  };
  const html = renderHTML(data, meta);
  const out = {};
  out[`${OUT_DIR}/report-${PROFILE}-${stamp}.html`] = html;
  out[`${OUT_DIR}/summary-${PROFILE}-${stamp}.json`] = JSON.stringify(data, null, 2);
  // stdout curto para não poluir o terminal em runs longos.
  out.stdout = `\n[dnj-k6] relatório: ${OUT_DIR}/report-${PROFILE}-${stamp}.html\n`;
  return out;
}
