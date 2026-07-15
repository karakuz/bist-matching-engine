$ErrorActionPreference = "Stop"

$container = "postgres"
$dbUser = "pte"
$dbName = "pte"

docker compose up -d $container | Out-Null

$files = @(
  "001_create_symbols.sql",
  "002_create_participants.sql",
  "003_create_orders.sql",
  "004_create_trades.sql",
  "005_create_order_events.sql",
  "006_create_market_sessions.sql",
  "007_seed_data.sql"
)

foreach ($file in $files) {
  Write-Host "Running migration: $file"
  docker compose exec -T $container psql -v ON_ERROR_STOP=1 -U $dbUser -d $dbName -f "/migrations/$file"

  if ($LASTEXITCODE -ne 0) {
    throw "Migration failed: $file"
  }
}

Write-Host "Migrations completed."
