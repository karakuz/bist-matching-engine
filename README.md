
Run migrations:
.\migrate.ps1


for tests under domain run:

go test ./internal/domain -v

If you want to run just one test function inside domain:

go test ./internal/domain -run TestOrderValidation -v