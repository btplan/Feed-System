# Starts the API with project-local Go caches.
Set-Location -LiteralPath $PSScriptRoot

$env:GOCACHE = "$PSScriptRoot\.run\build-cache"
$env:GOMODCACHE = "$PSScriptRoot\.run\gomodcache"
$env:GOBIN = "$PSScriptRoot\.run\bin"

go run ./cmd/api