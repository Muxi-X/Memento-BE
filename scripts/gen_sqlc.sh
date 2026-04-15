#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

if command -v sqlc >/dev/null 2>&1; then
  sqlc generate -f sqlc.yaml
else
  go run github.com/sqlc-dev/sqlc/cmd/sqlc generate -f sqlc.yaml
fi

echo "generated -> sqlc outputs"
