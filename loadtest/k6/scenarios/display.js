// Cenário telão/TV: poucas telas públicas, 3 GETs a cada 15 s, SEM bearer.
// Espelha src/features/display/live-ranking-display.tsx. As duas queries de
// ranking são as CTE mais pesadas da API (window function sobre todos os users).
import { sleep } from 'k6';
import { get } from '../lib/http.js';
import { ROUTES as R } from '../lib/routes.js';

const SCENARIO = 'display';

export function display() {
  const opts = { scenario: SCENARIO };
  get(R.rankingsIndividual.template, R.rankingsIndividual.path, { ...opts, seq: 0 });
  get(R.rankingsGroups.template, R.rankingsGroups.path, { ...opts, seq: 1 });
  get(R.liveDisplayTv.template, R.liveDisplayTv.path, { ...opts, seq: 2 });
  sleep(15);
}
