#!/usr/bin/env bash
# Gera public/<env>/{swagger.json,index.html} a partir da spec commitada,
# injetando host/basePath/schemes derivados da URL base do ambiente.
#
# Uso: ./scripts/publish-openapi-docs.sh <v1-spec.json> <base-url> <env> [v2-spec.json]
# Ex.: ./scripts/publish-openapi-docs.sh docs/openapi/swagger.json \
#        "https://abc.execute-api.us-east-1.amazonaws.com/develop" develop
#
# Não inclua o basePath da aplicação (ex: /v1) na base-url — ele já vem do
# swagger.json gerado e é concatenado automaticamente.
set -euo pipefail

SPEC_IN="$1"
BASE_URL="$2"
ENV_NAME="$3"
V2_SPEC_IN="${4:-}"

PAGES_DIR="$(dirname "$0")/../docs/openapi/pages"
OUT_DIR="public/${ENV_NAME}"

SCHEME="${BASE_URL%%://*}"
REST="${BASE_URL#*://}"
HOST="${REST%%/*}"
if [ "$REST" != "$HOST" ]; then
  PREFIX="/${REST#*/}"
else
  PREFIX=""
fi

ORIG_BASE_PATH="$(jq -r '.basePath' "$SPEC_IN")"
NEW_BASE_PATH="${PREFIX}${ORIG_BASE_PATH}"

mkdir -p "$OUT_DIR"

jq --arg host "$HOST" --arg basePath "$NEW_BASE_PATH" --arg scheme "$SCHEME" \
  '.host = $host | .basePath = $basePath | .schemes = [$scheme]' \
  "$SPEC_IN" > "$OUT_DIR/swagger.json"

cp "$PAGES_DIR/index.html" "$OUT_DIR/index.html"
cp "$PAGES_DIR/root-index.html" "public/index.html"

if [ -n "$V2_SPEC_IN" ]; then
  V2_OUT_DIR="$OUT_DIR/v2"
  V2_BASE_URL="${BASE_URL%/}/v2"
  mkdir -p "$V2_OUT_DIR"
  jq --arg serverUrl "$V2_BASE_URL" '.servers = [{"url": $serverUrl, "description": "'"$ENV_NAME"'"}]' \
    "$V2_SPEC_IN" > "$V2_OUT_DIR/openapi.json"
  cp "$PAGES_DIR/v2-index.html" "$V2_OUT_DIR/index.html"
fi

# Frontend handoff page (Iteration 10): publishes alongside V2 at
# /<env>/frontend-integration/, self-contained (fetches its own JSON
# manifest, no build step). __DNJ_HANDOFF_REF__ is rewritten to the exact
# commit deployed so the page's links to the canonical Markdown on GitHub
# always resolve to what is actually live, not to a branch that may move.
HANDOFF_MANIFEST="$(dirname "$0")/../docs/handoff/dnj-v2-frontend-integration.json"
if [ -f "$HANDOFF_MANIFEST" ]; then
  HANDOFF_OUT_DIR="$OUT_DIR/frontend-integration"
  mkdir -p "$HANDOFF_OUT_DIR"
  # Falls back to the branch this env actually deploys from (develop.yml
  # triggers on "develop", production.yml on "main" — ENV_NAME itself is
  # "production", not a real branch) if HEAD can't be resolved (detached/
  # shallow checkout edge case). A production publish must never point its
  # handoff links at develop's tree.
  FALLBACK_REF="develop"
  if [ "$ENV_NAME" = "production" ]; then FALLBACK_REF="main"; fi
  DEPLOY_REF="$(git -C "$(dirname "$0")/.." rev-parse HEAD 2>/dev/null || echo "$FALLBACK_REF")"
  sed "s/__DNJ_HANDOFF_REF__/${DEPLOY_REF}/" "$PAGES_DIR/frontend-integration.html" > "$HANDOFF_OUT_DIR/index.html"
  cp "$HANDOFF_MANIFEST" "$HANDOFF_OUT_DIR/dnj-v2-frontend-integration.json"
fi

echo "OpenAPI docs publicados em $OUT_DIR (host=$HOST basePath=$NEW_BASE_PATH scheme=$SCHEME)"
