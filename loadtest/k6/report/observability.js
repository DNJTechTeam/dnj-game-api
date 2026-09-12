// Conteúdo estático da seção "Onde observar e o quê" do relatório.
// Descreve, por camada, quais métricas olhar e onde, com queries prontas.
// As queries usam a janela de tempo do teste e o prefixo de requestId "k6-".
export function observabilitySections(meta) {
  const region = 'sa-east-1';
  const fnHint = meta.functionName || '<LAMBDA_FUNCTION_NAME>';
  const logGroup = `/aws/lambda/${fnHint}`;
  const from = meta.startedAtUTC || '<início UTC>';
  const to = meta.endedAtUTC || '<fim UTC>';

  return [
    {
      layer: 'Supabase — banco (pooler + Postgres)',
      why:
        'O pool é de apenas 4 conexões por container Lambda. Sob concorrência alta, ' +
        'o gargalo esperado é o pooler Supavisor saturar, não a CPU da Lambda. ' +
        'Este é o risco número 1 do teste.',
      items: [
        'Dashboard → Database → Roles/Connections: nº de conexões client vs server no pooler; ' +
          'compare com o teto do plano. Saturação aqui aparece como espera/erro de conexão.',
        'Dashboard → Reports → Database: CPU, Memory, Disk IO, e "Database connections" ao longo da janela.',
        'Advisors / Query Performance (pg_stat_statements): olhe os statements mais chamados. ' +
          'Se "pgbouncer.get_auth" dominar, é churn de conexão (reabertura), não query lenta.',
        'Logs → Postgres / Pooler: procure "too many connections", "remaining connection slots", timeouts.',
        'Foco de queries: /v2/game/overview e /v2/rankings (CTE com window function sobre todos os users) ' +
          'são os pesos-pesados; confirme o plano de execução se p95 subir.',
      ],
    },
    {
      layer: 'AWS Lambda — métricas nativas (CloudWatch → AWS/Lambda)',
      why:
        'Sem provisioned concurrency conhecida, um pico coordenado pode gerar cold starts em massa ' +
        'e, no limite, consumir a cota de concorrência da conta inteira em ' +
        region +
        '.',
      items: [
        `ConcurrentExecutions (Max) da função ${fnHint}: quantos containers simultâneos — multiplique por 4 para estimar conexões no Supabase.`,
        'Throttles (Sum): se > 0, a conta/função atingiu o limite de concorrência. Correlaciona com 5xx/timeouts no k6.',
        'Duration p50/p99 e o par Invocations/Errors: compare Duration p99 com o timeout de 10 s do cliente.',
        'InitDuration (via logs REPORT): magnitude e frequência de cold start durante o ramp e o spike.',
        'Métrica de conta "ClaimedAccountConcurrency" / cota de concorrência regional: quão perto do teto.',
      ],
    },
    {
      layer: 'AWS Lambda Insights (se a layer estiver habilitada)',
      why:
        'Dá CPU, memória, rede e cold starts por função sem instrumentar código. ' +
        'Não há EMF/X-Ray custom no projeto, então Insights é a melhor visão de recurso.',
      items: [
        'CloudWatch → Lambda Insights → a função: memory utilization (chega perto do limite configurado?), ' +
          'cpu total time, network rx/tx, e contagem de cold starts.',
        'Se a layer NÃO estiver habilitada, isto fica vazio — habilite antes do teste ' +
          'ou use apenas as métricas nativas AWS/Lambda acima.',
      ],
    },
    {
      layer: 'CloudWatch Logs Insights — log estruturado da app',
      why:
        'Cada request emite um evento JSON http_request_completed com route, status, latencyMs, ' +
        'dbQueries e dbMs. É a fonte para atribuir latência (app vs DB) e cruzar com o k6.',
      items: [
        `Log group: ${logGroup} — janela ${from} → ${to} (UTC).`,
        'Query — percentis por rota e status:',
        'CODE:filter msg = "http_request_completed"\n' +
          '| stats count() as reqs, pct(latencyMs,50) as p50, pct(latencyMs,95) as p95, pct(latencyMs,99) as p99, avg(dbMs) as avgDbMs, avg(dbQueries) as avgQ by route, status\n' +
          '| sort reqs desc',
        'Query — atribuição de latência ao banco (quanto do tempo foi DB):',
        'CODE:filter msg = "http_request_completed"\n' +
          '| stats avg(latencyMs) as appMs, avg(dbMs) as dbMs, avg(dbMs)/avg(latencyMs)*100 as pctDb by route\n' +
          '| sort pctDb desc',
        'Query — correlação 1:1 com uma iteração específica do k6 (requestId tem prefixo k6-):',
        'CODE:filter requestId like /^k6-/ and status >= 500\n' +
          '| fields @timestamp, requestId, route, status, latencyMs, dbMs\n' +
          '| sort @timestamp desc | limit 100',
        'Query — panics:',
        'CODE:filter msg = "http_request_panic" | stats count() by route',
      ],
    },
    {
      layer: 'CloudWatch Live Tail — durante a execução',
      why: 'Feedback em tempo real de erros enquanto o k6 roda, sem esperar o fim.',
      items: [
        `Console → CloudWatch → Live Tail → log group ${logGroup}.`,
        'Filtro só de erros de servidor:',
        'CODE:{ $.msg = "http_request_completed" && $.status >= 500 }',
        'Filtro de uma rota específica (ex. o overview pesado):',
        'CODE:{ $.route = "/v2/game/overview" }',
        'Se aparecer "http_request_panic", pare e investigue antes de subir a carga.',
      ],
    },
  ];
}
