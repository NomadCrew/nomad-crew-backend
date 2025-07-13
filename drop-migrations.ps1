# drop-migrations.ps1
# Purpose: completely reset the database by dropping all objects via migrate CLI.

# Load environment variables from .env (same pattern as run-migrations.ps1)
foreach ($line in [System.IO.File]::ReadAllLines(".env")) {
    if ($line -match '^([^=]+)=(.*)$') {
        $key = $matches[1].Trim()
        $value = $matches[2].Trim()
        Set-Item "env:$key" $value
    }
}

$port = if ($env:DB_PORT) { $env:DB_PORT } else { "5432" }
$DB_URL = "postgres://$($env:DB_USER):$($env:DB_PASSWORD)@$($env:DB_HOST):$port/$($env:DB_NAME)?sslmode=require"

Write-Host "Dropping ALL database objects in $($env:DB_NAME) on host $($env:DB_HOST)..." -ForegroundColor Yellow

$migrateExe = "C:\Users\naqee\go\bin\migrate.exe"

& $migrateExe -path db/migrations -database $DB_URL drop -f

if ($LASTEXITCODE -eq 0) {
    Write-Host "Database drop completed successfully!" -ForegroundColor Green
} else {
    Write-Host "Database drop failed with exit code $LASTEXITCODE" -ForegroundColor Red
    exit $LASTEXITCODE
} 