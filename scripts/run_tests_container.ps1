# Runs nomad-crew-backend Go tests in an isolated Linux container
# Usage: .\scripts\run_tests_container.ps1

# Resolve current project path (assumes script lives within repo)
$projectPath = (Get-Item -Path $PSScriptRoot).Parent.FullName
# Convert to POSIX-style path for Docker volume mount
$projectPathUnix = $projectPath -replace '\\', '/'

Write-Host "Running tests from $projectPathUnix inside golang:1.24-bookworm container..." -ForegroundColor Cyan

# Run tests
docker run --rm -it `
  -v "$($projectPathUnix):/workspace" `
  -w /workspace `
  -v "//var/run/docker.sock:/var/run/docker.sock" `
  -e TESTCONTAINERS_RYUK_DISABLED=true `
  -e TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal `
  --add-host=host.docker.internal:host-gateway `
  golang:1.24-bookworm `
  bash -c "go test ./... -count=1" 