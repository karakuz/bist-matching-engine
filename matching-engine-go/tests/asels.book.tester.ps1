param(
  [int]$DurationSeconds = 10,
  [int]$Workers = 8
)

$jobs = foreach ($worker in 1..$Workers) {
  Start-Job -ArgumentList $worker, $DurationSeconds -ScriptBlock {
    param($worker, $durationSeconds)

    $counts = @{}
    $watch = [System.Diagnostics.Stopwatch]::StartNew()

    while ($watch.Elapsed.TotalSeconds -lt $durationSeconds) {
      $body = @{
        participantId = (($worker - 1) % 3) + 1
        symbol        = "ASELS"
        side          = if ($worker % 2 -eq 0) { "BUY" } else { "SELL" }
        price         = (Get-Random -Minimum 3150 -Maximum 3851) * 10
        quantity      = Get-Random -Minimum 10 -Maximum 10001
      } | ConvertTo-Json -Compress

      try {
        $response = Invoke-WebRequest `
          -Method Post `
          -Uri "http://localhost:8080/orders" `
          -ContentType "application/json" `
          -Body $body `
          -TimeoutSec 5 `
          -UseBasicParsing

        $status = [string][int]$response.StatusCode
      } catch {
        $status = if ($_.Exception.Response) {
          [string][int]$_.Exception.Response.StatusCode
        } else {
          "ERROR"
        }
      }

      if (-not $counts.ContainsKey($status)) {
        $counts[$status] = 0
      }
      $counts[$status]++
    }

    [pscustomobject]@{
      Worker = $worker
      Attempts = ($counts.Values | Measure-Object -Sum).Sum
      StatusCounts = ($counts | ConvertTo-Json -Compress)
    }
  }
}

$results = Receive-Job -Job $jobs -Wait
$results | Format-Table -AutoSize
Remove-Job -Job $jobs
