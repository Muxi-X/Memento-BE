#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}/api/openapi"

if command -v oapi-codegen >/dev/null 2>&1; then
  oapi-codegen --config oapi-codegen.v1.yaml openapi.v1.yaml
else
  go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config oapi-codegen.v1.yaml openapi.v1.yaml
fi

echo "generated -> internal/transport/http/v1/gen/openapi.gen.go"
