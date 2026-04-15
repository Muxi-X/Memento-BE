$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Push-Location (Join-Path $root "api\openapi")
try {
  $cmd = Get-Command oapi-codegen -ErrorAction SilentlyContinue
  if ($cmd) {
    & $cmd.Source --config oapi-codegen.v1.yaml openapi.v1.yaml
  } else {
    go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config oapi-codegen.v1.yaml openapi.v1.yaml
  }
  Write-Host "generated -> internal/transport/http/v1/gen/openapi.gen.go"
} finally {
  Pop-Location
}
