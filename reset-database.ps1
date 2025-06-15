# reset-database.ps1
# Hard reset of local Neon database: drops public & auth schemas (and all contained objects)
# then recreates empty schemas and re-runs migrations.

# Load env vars from .env
foreach ($line in [System.IO.File]::ReadAllLines(".env")) {
    if ($line -match '^([^=]+)=(.*)$') {
        $key = $matches[1].Trim()
        $value = $matches[2].Trim()
        Set-Item "env:$key" $value
    }
}

$port      = if ($env:DB_PORT) { $env:DB_PORT } else { "5432" }
$DB_URL    = "postgres://$($env:DB_USER):$($env:DB_PASSWORD)@$($env:DB_HOST):$port/$($env:DB_NAME)?sslmode=require"
$psqlExe   = "psql"   # assumes psql is on PATH
$migrateExe = "C:\Users\naqee\go\bin\migrate.exe"

Write-Host "Running HARD database reset on $($env:DB_HOST)/$($env:DB_NAME)" -ForegroundColor Yellow

# SQL to nuke schemas + enum types (if drop schema fails due to extensions still referencing them)
$sql = @"
DO $$
BEGIN
    -- Drop application schemas
    PERFORM 1;
    EXECUTE 'DROP SCHEMA IF EXISTS public CASCADE';
    EXECUTE 'CREATE SCHEMA public';
    EXECUTE 'DROP SCHEMA IF EXISTS auth  CASCADE';
    EXECUTE 'CREATE SCHEMA auth';

    -- Clean up stray enum types that might survive outside schema drops
    PERFORM 1;
    EXECUTE 'DROP TYPE IF EXISTS trip_status CASCADE';
    EXECUTE 'DROP TYPE IF EXISTS membership_role CASCADE';
    EXECUTE 'DROP TYPE IF EXISTS membership_status CASCADE';
    EXECUTE 'DROP TYPE IF EXISTS todo_status CASCADE';
    EXECUTE 'DROP TYPE IF EXISTS notification_type CASCADE';
    EXECUTE 'DROP TYPE IF EXISTS invitation_status CASCADE';
    EXECUTE 'DROP TYPE IF EXISTS location_privacy CASCADE';
END $$;
"@

# Run the SQL using psql
& $psqlExe $DB_URL -v ON_ERROR_STOP=1 -c $sql
if ($LASTEXITCODE -ne 0) {
    Write-Host "Hard reset SQL failed. Aborting." -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "Schemas dropped and recreated. Running fresh migrations..." -ForegroundColor Cyan

& $migrateExe -path db/migrations -database $DB_URL force 0
& $migrateExe -path db/migrations -database $DB_URL up
if ($LASTEXITCODE -eq 0) {
    Write-Host "Hard reset + migrations completed successfully!" -ForegroundColor Green
} else {
    Write-Host "Migrations failed after hard reset." -ForegroundColor Red
    exit $LASTEXITCODE
} 