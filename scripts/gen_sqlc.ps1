$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Push-Location $root
try {
  $cmd = Get-Command sqlc -ErrorAction SilentlyContinue
  if ($cmd) {
    & $cmd.Source generate -f sqlc.yaml
  } else {
    go run github.com/sqlc-dev/sqlc/cmd/sqlc generate -f sqlc.yaml
  }
  Write-Host "generated -> sqlc outputs"
} finally {
  Pop-Location
}
