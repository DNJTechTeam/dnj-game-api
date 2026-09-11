// Fonte de identidades para os VUs.
//
// Duas origens, em ordem de preferência:
//   1. data/users.json  -> array de { "id": "123", "role": "DEFAULT" } ou array de ids.
//      Gerado por scripts/k6-export-users.sql / k6-run-managed.sh.
//   2. env K6_USER_IDS   -> lista separada por vírgula (fallback rápido, sem arquivo).
//
// IMPORTANTE (memória): exponha as identidades como SharedArray. O callback do
// SharedArray roda UMA vez e o resultado é compartilhado entre todos os VUs; acessar
// um elemento por índice devolve uma cópia pequena, não o array inteiro. NUNCA
// materialize um array filtrado no escopo de módulo de um cenário: isso rodaria por
// VU e, com 15k identidades × milhares de VUs, estoura a RAM na inicialização.
import { SharedArray } from 'k6/data';

const USERS_FILE = __ENV.K6_USERS_FILE || './data/users.json';

function parseInline() {
  const raw = (__ENV.K6_USER_IDS || '').trim();
  if (!raw) return [];
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
    .map((id) => ({ id: String(id), role: 'DEFAULT' }));
}

function normalize(entry) {
  if (entry === null || entry === undefined) return null;
  if (typeof entry === 'object') {
    if (entry.id === undefined || entry.id === null) return null;
    return { id: String(entry.id), role: entry.role || 'DEFAULT' };
  }
  return { id: String(entry), role: 'DEFAULT' };
}

// Carrega e normaliza todas as identidades. Roda dentro dos callbacks de SharedArray
// (uma vez por SharedArray, na init), não por VU.
function loadAll() {
  let list = [];
  try {
    const parsed = JSON.parse(open(USERS_FILE));
    if (Array.isArray(parsed)) list = parsed;
    else if (parsed && Array.isArray(parsed.users)) list = parsed.users;
  } catch (e) {
    list = [];
  }
  if (list.length === 0) list = parseInline();
  const normalized = list.map(normalize).filter(Boolean);
  if (normalized.length === 0) {
    throw new Error(
      'Nenhuma identidade carregada. Popule loadtest/k6/data/users.json ' +
        '(veja scripts/k6-export-users.sql) ou defina K6_USER_IDS.',
    );
  }
  return normalized;
}

// Participantes (role DEFAULT) e gestores (EVENT_MANAGER), cada um compartilhado.
export const defaultUsers = new SharedArray('dnj_default_users', () =>
  loadAll().filter((u) => u.role === 'DEFAULT'),
);
export const managerUsers = new SharedArray('dnj_manager_users', () =>
  loadAll().filter((u) => u.role === 'EVENT_MANAGER'),
);

// Escolhe uma identidade determinística por VU (wrap-around se houver menos que VUs).
// `pool` deve ser um SharedArray (defaultUsers/managerUsers); nunca um array copiado.
export function pickUser(vuId, pool) {
  const src = pool && pool.length ? pool : defaultUsers;
  return src[(vuId - 1) % src.length];
}
