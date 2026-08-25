#!/usr/bin/env bash
# Starts the API server built at $1, waits for it to answer healthcheck, runs
# the loadtest binary built at $2 against it with the given profile, then
# always stops the server — regardless of whether the profile passed — so a
# failed run never leaks a background process in CI or on a laptop.
set -uo pipefail

api_binary="${1:?path to the built API binary is required}"
loadtest_binary="${2:?path to the built loadtest binary is required}"
url="${3:?base url is required}"
path="${4:?request path is required}"
concurrency="${5:?concurrency is required}"
rps="${6:?rps is required}"
duration="${7:?duration is required}"
p95_budget_ms="${8:?p95 budget is required}"
error_budget_percent="${9:?error budget is required}"
request_timeout_seconds="${10:?request timeout is required}"
max_server_errors="${11:?max server errors is required}"

"$api_binary" >/tmp/dnj-loadtest-smoke-api.log 2>&1 &
api_pid=$!
trap 'kill "$api_pid" 2>/dev/null || true; wait "$api_pid" 2>/dev/null || true' EXIT

healthcheck_url="${url%/}/healthcheck"
ready=0
for _ in $(seq 1 30); do
  if curl -sf "$healthcheck_url" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if [ "$ready" -ne 1 ]; then
  echo "API server never became healthy at $healthcheck_url" >&2
  tail -100 /tmp/dnj-loadtest-smoke-api.log >&2 || true
  exit 1
fi

"$loadtest_binary" \
  -url "$url" -path "$path" \
  -concurrency "$concurrency" -rps "$rps" -duration "$duration" \
  -p95-budget-ms "$p95_budget_ms" -error-budget-percent "$error_budget_percent" \
  -request-timeout-seconds "$request_timeout_seconds" -max-server-errors "$max_server_errors"
exit_code=$?
if [ "$exit_code" -ne 0 ]; then
  echo "--- API server log (tail) ---" >&2
  tail -100 /tmp/dnj-loadtest-smoke-api.log >&2 || true
fi
exit "$exit_code"
