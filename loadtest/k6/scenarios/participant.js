// Cenário participante: 1 VU = 1 participante com o navegador aberto.
//
// Reproduz o shell do front (src/components/dnj-app.tsx): quatro pollings de 15 s
// desalinhados em fase, rodando em TODAS as telas principais, sem guard de
// visibilidade. Na tela "game" com run ativa, troca game/overview (15 s) por
// activity-runs/current (5 s). Baseline: ~16 req/min por participante.
//
// Em vez de quatro timers reais, cada iteração do VU representa um "tick" de 15 s:
// dispara os 4 GETs do ciclo (ou o mix da tela game) e dorme o restante do intervalo,
// com jitter de fase por VU para não sincronizar todos os VUs no mesmo instante.
import { sleep } from 'k6';
import { get } from '../lib/http.js';
import { ROUTES as R } from '../lib/routes.js';
import { signIdentityToken } from '../lib/jwt.js';
import { defaultUsers, pickUser } from '../lib/users.js';

const SECRET = __ENV.JWT_IDENTITY_SECRET || '';
const TOKEN_TTL = parseInt(__ENV.K6_TOKEN_TTL_SECONDS || '10800', 10); // 3 h
const CYCLE_SECONDS = 15;
const SCENARIO = 'participant';

// Cache de token por VU (não re-assina a cada iteração).
let vuToken = null;
let vuUser = null;

function ensureToken() {
  if (vuToken) return;
  if (!SECRET) throw new Error('JWT_IDENTITY_SECRET ausente: defina no .env.k6 / ambiente.');
  vuUser = pickUser(__VU, defaultUsers);
  vuToken = signIdentityToken(vuUser.id, SECRET, TOKEN_TTL);
}

// Distribuição de telas aproximando o uso real durante o evento.
function pickScreen() {
  const r = Math.random();
  if (r < 0.45) return 'common'; // home / cronograma / mapa / conta
  if (r < 0.70) return 'gallery';
  if (r < 0.90) return 'game_run'; // tela game com run ativa (polling 5 s)
  return 'game_idle'; // tela game sem run
}

export function participant() {
  ensureToken();
  const opts = { token: vuToken, scenario: SCENARIO };
  const screen = pickScreen();
  const start = Date.now();

  if (screen === 'game_run') {
    // Mount da tela game + 3 ciclos de polling de 5 s (= 15 s de janela).
    get(R.gameOverview.template, R.gameOverview.path, { ...opts, seq: 0 });
    get(R.currentParticipation.template, R.currentParticipation.path, { ...opts, seq: 1, expected: [204] });
    for (let i = 0; i < 3; i++) {
      get(R.currentRun.template, R.currentRun.path, { ...opts, seq: 2 + i, expected: [204] });
      if (i < 2) sleep(5);
    }
  } else {
    // Shell: os 3 pollings sempre ativos + game/overview (exceto na tela game).
    get(R.specialEventsActive.template, R.specialEventsActive.path, { ...opts, seq: 0, expected: [204] });
    get(R.notifications.template, R.notifications.path, { ...opts, seq: 1 });
    get(R.challenges.template, R.challenges.path, { ...opts, seq: 2 });
    if (screen === 'game_idle') {
      get(R.currentRun.template, R.currentRun.path, { ...opts, seq: 3, expected: [204] });
    } else {
      get(R.gameOverview.template, R.gameOverview.path, { ...opts, seq: 3 });
    }
    if (screen === 'gallery') {
      get(R.momentsFeed.template, R.momentsFeed.path, { ...opts, seq: 4 });
    }
  }

  // Completa a janela de 15 s. Jitter de fase determinístico por VU evita
  // que todos os VUs disparem no mesmo milissegundo (thundering herd artificial).
  const elapsed = (Date.now() - start) / 1000;
  const phase = (__VU % CYCLE_SECONDS) / CYCLE_SECONDS; // 0..1
  const rest = Math.max(0.5, CYCLE_SECONDS - elapsed + (phase - 0.5) * 2);
  sleep(rest);
}
