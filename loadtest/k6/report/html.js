// Gera um relatório HTML self-contained (CSS inline, sem dependência de CDN) a
// partir do objeto `data` do handleSummary do k6.
import { observabilitySections } from './observability.js';

function esc(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}
function n(v, d) {
  if (v === undefined || v === null || isNaN(v)) return '—';
  return Number(v).toFixed(d === undefined ? 0 : d);
}
function ms(v) {
  return v === undefined || v === null || isNaN(v) ? '—' : `${Number(v).toFixed(0)} ms`;
}

function metric(data, name) {
  const m = data.metrics[name];
  return m ? m.values : {};
}

// Extrai as sub-métricas por rota do summary: contagem vem do Counter
// dnj_route_reqs{route:...}; latências do Trend dnj_route_duration{route:...}.
function routeRows(data) {
  const rows = [];
  const reDur = /^dnj_route_duration\{route:(.+)\}$/;
  for (const key of Object.keys(data.metrics)) {
    const mt = key.match(reDur);
    if (!mt) continue;
    const route = mt[1];
    const dur = data.metrics[key].values || {};
    const cnt = (data.metrics[`dnj_route_reqs{route:${route}}`] || {}).values || {};
    const count = cnt.count || 0;
    if (!count) continue; // rota não exercitada neste run.
    rows.push({
      route: route,
      count: count,
      p50: dur.med,
      p95: dur['p(95)'],
      p99: dur['p(99)'],
      max: dur.max,
      avg: dur.avg,
    });
  }
  rows.sort((a, b) => b.count - a.count);
  return rows;
}

function thresholdRows(data) {
  const rows = [];
  for (const name of Object.keys(data.metrics)) {
    const m = data.metrics[name];
    if (!m.thresholds) continue;
    for (const expr of Object.keys(m.thresholds)) {
      const th = m.thresholds[expr];
      const ok = th.ok !== undefined ? th.ok : !th.fails;
      rows.push({ metric: name, expr: expr, ok: ok });
    }
  }
  return rows;
}

function bar(pct, color) {
  const w = Math.max(0, Math.min(100, pct));
  return `<div class="bar"><div class="fill" style="width:${w}%;background:${color}"></div></div>`;
}

export function renderHTML(data, meta) {
  const dur = metric(data, 'http_req_duration');
  const reqs = metric(data, 'http_reqs');
  const failed = metric(data, 'http_req_failed');
  const srvErr = metric(data, 'dnj_server_errors');
  const cliErr = metric(data, 'dnj_client_errors');
  const toRate = metric(data, 'dnj_timeout_rate');
  const vusMax = metric(data, 'vus_max');
  const dataRecv = metric(data, 'data_received');

  const routes = routeRows(data);
  const maxCount = routes.reduce((a, r) => Math.max(a, r.count), 1);
  const maxP99 = routes.reduce((a, r) => Math.max(a, r.p99 || 0), 1);
  const ths = thresholdRows(data);
  const failedTh = ths.filter((t) => !t.ok);
  const sections = observabilitySections(meta);

  const routeTable = routes
    .map(
      (r) => `<tr>
      <td class="mono">${esc(r.route)}</td>
      <td class="num">${n(r.count)}</td>
      <td class="num">${ms(r.p50)}</td>
      <td class="num">${ms(r.p95)}</td>
      <td class="num strong">${ms(r.p99)}</td>
      <td class="num">${ms(r.max)}</td>
      <td>${bar((r.count / maxCount) * 100, '#3b82f6')}</td>
      <td>${bar((( r.p99 || 0) / maxP99) * 100, (r.p99 || 0) > 8000 ? '#ef4444' : (r.p99 || 0) > 3000 ? '#f59e0b' : '#22c55e')}</td>
    </tr>`,
    )
    .join('\n');

  const thTable = ths
    .map(
      (t) =>
        `<tr><td class="mono">${esc(t.metric)}</td><td class="mono">${esc(t.expr)}</td><td>${
          t.ok ? '<span class="pass">PASS</span>' : '<span class="fail">FAIL</span>'
        }</td></tr>`,
    )
    .join('\n');

  const obsHTML = sections
    .map((s) => {
      const items = s.items
        .map((it) => {
          if (typeof it === 'string' && it.startsWith('CODE:')) {
            return `<pre class="code">${esc(it.slice(5))}</pre>`;
          }
          return `<li>${esc(it)}</li>`;
        })
        .join('\n');
      return `<div class="obs">
        <h3>${esc(s.layer)}</h3>
        <p class="why">${esc(s.why)}</p>
        <ul>${items}</ul>
      </div>`;
    })
    .join('\n');

  const verdict = failedTh.length === 0 ? 'APROVADO' : 'REPROVADO';
  const verdictClass = failedTh.length === 0 ? 'ok' : 'bad';

  return `<!doctype html><html lang="pt-BR"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Relatório de carga k6 — DNJ Game API</title>
<style>
:root{--bg:#0b1020;--card:#141b2e;--ink:#e8edf7;--mut:#9aa7bd;--line:#26304a;--blue:#3b82f6}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:14px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif}
.wrap{max-width:1100px;margin:0 auto;padding:24px 16px}
h1{font-size:22px;margin:0 0 4px}
h2{font-size:17px;margin:28px 0 12px;border-bottom:1px solid var(--line);padding-bottom:6px}
h3{font-size:15px;margin:0 0 6px}
.sub{color:var(--mut);margin:0 0 16px}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:12px}
.kpi{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:14px}
.kpi .v{font-size:22px;font-weight:700}
.kpi .l{color:var(--mut);font-size:12px;text-transform:uppercase;letter-spacing:.04em}
.badge{display:inline-block;padding:4px 12px;border-radius:999px;font-weight:700;font-size:13px}
.badge.ok{background:#0f3d24;color:#7ee2a8;border:1px solid #1c7a47}
.badge.bad{background:#3d1414;color:#ffb0b0;border:1px solid #7a1c1c}
table{width:100%;border-collapse:collapse;background:var(--card);border:1px solid var(--line);border-radius:10px;overflow:hidden}
th,td{padding:8px 10px;text-align:left;border-bottom:1px solid var(--line);font-size:13px}
th{color:var(--mut);font-weight:600;text-transform:uppercase;font-size:11px;letter-spacing:.04em}
td.num{text-align:right;font-variant-numeric:tabular-nums}
td.strong{font-weight:700}
.mono{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px}
.bar{background:#1e2740;border-radius:4px;height:10px;width:100%;overflow:hidden}
.fill{height:100%}
.pass{color:#7ee2a8;font-weight:700}.fail{color:#ffb0b0;font-weight:700}
.obs{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:14px;margin-bottom:12px}
.obs .why{color:var(--mut);margin:0 0 8px;font-size:13px}
.obs ul{margin:6px 0;padding-left:18px}.obs li{margin:3px 0}
pre.code{background:#0a0f1e;border:1px solid var(--line);border-radius:8px;padding:10px;overflow-x:auto;font-size:12px;color:#cfe0ff;white-space:pre}
.foot{color:var(--mut);font-size:12px;margin-top:24px}
</style></head><body><div class="wrap">
<h1>Relatório de teste de carga — DNJ Game API</h1>
<p class="sub">Perfil <b>${esc(meta.profile)}</b> · alvo <span class="mono">${esc(meta.baseUrl)}</span> · gerado ${esc(
    meta.generatedAt,
  )}</p>
<p><span class="badge ${verdictClass}">${verdict}</span> &nbsp; ${failedTh.length} threshold(s) reprovado(s)</p>

<div class="grid">
  <div class="kpi"><div class="l">Requests</div><div class="v">${n(reqs.count)}</div></div>
  <div class="kpi"><div class="l">RPS médio</div><div class="v">${n(reqs.rate, 1)}</div></div>
  <div class="kpi"><div class="l">VUs máx</div><div class="v">${n(vusMax.max || vusMax.value)}</div></div>
  <div class="kpi"><div class="l">p95 global</div><div class="v">${ms(dur['p(95)'])}</div></div>
  <div class="kpi"><div class="l">p99 global</div><div class="v">${ms(dur['p(99)'])}</div></div>
  <div class="kpi"><div class="l">Erro (http_req_failed)</div><div class="v">${n(
    (failed.rate || 0) * 100,
    2,
  )}%</div></div>
  <div class="kpi"><div class="l">Erros 5xx/transporte</div><div class="v">${n(srvErr.count || 0)}</div></div>
  <div class="kpi"><div class="l">4xx inesperados</div><div class="v">${n(cliErr.count || 0)}</div></div>
  <div class="kpi"><div class="l">Taxa de timeout</div><div class="v">${n((toRate.rate || 0) * 100, 2)}%</div></div>
  <div class="kpi"><div class="l">Dados recebidos</div><div class="v">${n(
    (dataRecv.count || 0) / 1e6,
    1,
  )} MB</div></div>
</div>

<h2>Por rota (route = template do log da API)</h2>
<table><thead><tr>
<th>Rota</th><th class="num">Reqs</th><th class="num">p50</th><th class="num">p95</th><th class="num">p99</th><th class="num">max</th><th>Volume</th><th>p99</th>
</tr></thead><tbody>
${routeTable || '<tr><td colspan="8">Sem sub-métricas por rota neste run.</td></tr>'}
</tbody></table>

<h2>Thresholds</h2>
<table><thead><tr><th>Métrica</th><th>Expressão</th><th>Resultado</th></tr></thead><tbody>
${thTable}
</tbody></table>

<h2>Onde observar e o quê</h2>
${obsHTML}

<p class="foot">Correlação: cada request leva um <span class="mono">X-Request-ID</span> com prefixo
<span class="mono">k6-&lt;cenário&gt;-&lt;vu&gt;-&lt;iter&gt;-&lt;seq&gt;</span>, ecoado no header de resposta e no log
<span class="mono">http_request_completed</span>. Janela do teste (UTC): ${esc(meta.startedAtUTC || '—')} →
${esc(meta.endedAtUTC || '—')}.</p>
</div></body></html>`;
}
