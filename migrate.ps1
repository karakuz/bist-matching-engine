$ErrorActionPreference = "Stop"

$container = "postgres"
$dbUser = "pte"
$dbName = "pte"
$composeFile = Join-Path $PSScriptRoot "docker-compose.yml"

docker compose -f $composeFile up -d $container | Out-Null

$migrationsPath = Join-Path $PSScriptRoot "matching-engine-go\migrations"

$files = Get-ChildItem `
    -LiteralPath $migrationsPath `
    -Filter "*.sql" |
    Sort-Object Name

if ($files.Count -eq 0) {
    throw "No migration files found in $migrationsPath"
}

foreach ($file in $files) {
    Write-Host "Running migration: $($file.Name)"

    docker compose -f $composeFile exec -T $container `
        psql `
        -v ON_ERROR_STOP=1 `
        -U $dbUser `
        -d $dbName `
        -f "/migrations/$($file.Name)"

    if ($LASTEXITCODE -ne 0) {
        throw "Migration failed: $($file.Name)"
    }
}

Write-Host "Migrations completed."
