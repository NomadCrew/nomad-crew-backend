# Load environment variables from .env file
foreach ($line in [System.IO.File]::ReadAllLines(".env")) {
    if ($line -match '^([^=]+)=(.*)$') {
        $key = $matches[1].Trim()
        $value = $matches[2].Trim()
        Set-Item "env:$key" $value
    }
}

# Construct the database URL (using standard port 5432 if not specified)
$port = if ($env:DB_PORT) { $env:DB_PORT } else { "5432" }
$DB_URL = "postgres://$($env:DB_USER):$($env:DB_PASSWORD)@$($env:DB_HOST):$port/$($env:DB_NAME)?sslmode=require"

Write-Host "Running migrations..."
Write-Host "Using database: $($env:DB_NAME) on host: $($env:DB_HOST)"

# Run the migrations
& "C:\Users\naqee\go\bin\migrate.exe" -path db/migrations -database $DB_URL up

if ($LASTEXITCODE -eq 0) {
    Write-Host "Migrations completed successfully!" -ForegroundColor Green
} else {
    Write-Host "Migration failed with exit code $LASTEXITCODE" -ForegroundColor Red
} 