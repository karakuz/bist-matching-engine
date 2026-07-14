$ErrorActionPreference = "Stop"

$container = "postgres"
$dbUser = "pte"
$dbName = "pte"

docker compose up -d $container | Out-Null

$files = @(
  "001_create_orders.sql",
  "002_create_trades.sql",
  "003_create_order_events.sql",
  "004_create_symbols.sql",
  "005_insert_symbols.sql",
  "006_market_sessions.sql",
  "007_participants.sql",
  "008_add_participant_id_to_orders.sql"
)

foreach ($file in $files) {
  Write-Host "Running migration: $file"
  docker compose exec -T $container psql -U $dbUser -d $dbName -f "/migrations/$file"
}

Write-Host "Migrations completed."
