#!/usr/bin/env bash
# Descobre a configuração da Lambda de develop (não versionada no repo) para
# preencher o relatório e planejar limites. Requer credenciais AWS válidas.
# Uso: bash loadtest/k6/scripts/k6-aws-discover.sh
set -uo pipefail
REGION="${AWS_DEFAULT_REGION:-sa-east-1}"
HOST_HINT="ttwkfudhvvhuhp5yvsoydxggum0ictpg"

echo "== Descoberta AWS (região ${REGION}) =="
if ! aws sts get-caller-identity --region "$REGION" >/dev/null 2>&1; then
  echo "✗ Credenciais AWS inválidas/expiradas. Renove (aws sso login / aws configure) e repita."
  exit 1
fi
echo "✓ Credenciais válidas: $(aws sts get-caller-identity --query Arn --output text --region "$REGION")"

echo
echo "-- Function URL casando com o hostname da develop --"
fn=""
for name in $(aws lambda list-functions --region "$REGION" --query 'Functions[].FunctionName' --output text); do
  url=$(aws lambda list-function-url-configs --function-name "$name" --region "$REGION" --query 'FunctionUrlConfigs[0].FunctionUrl' --output text 2>/dev/null)
  if [[ "$url" == *"$HOST_HINT"* ]]; then fn="$name"; echo "✓ função: ${name}"; echo "  url: ${url}"; break; fi
done
[[ -z "$fn" ]] && { echo "! nenhuma Function URL casou '${HOST_HINT}'. Liste manualmente com: aws lambda list-functions"; exit 0; }

echo
echo "-- Configuração de runtime --"
aws lambda get-function-configuration --function-name "$fn" --region "$REGION" \
  --query '{Memory:MemorySize,Timeout:Timeout,Arch:Architectures,Runtime:PackageType,Layers:Layers[].Arn,Env:Environment.Variables.SERVER_ENVIRONMENT}' --output table

echo
echo "-- Concorrência reservada/provisionada --"
aws lambda get-function-concurrency --function-name "$fn" --region "$REGION" --output table 2>/dev/null || echo "  (sem reserved concurrency)"
aws lambda list-provisioned-concurrency-configs --function-name "$fn" --region "$REGION" --output table 2>/dev/null || echo "  (sem provisioned concurrency)"

echo
echo "Para o relatório, exporte: K6_LAMBDA_FUNCTION_NAME=${fn}"
echo "Log group das queries: /aws/lambda/${fn}"
