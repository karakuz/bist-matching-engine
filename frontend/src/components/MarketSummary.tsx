import { Badge, Group, Paper, SimpleGrid, Stack, Text } from '@mantine/core'
import type { SymbolSummary } from '../types/market'
import { formatMarketPrice } from '../utils/marketFormat'

type MarketSummaryProps = {
  symbol: string
  summary: SymbolSummary | null
  bestBid: number | null
  bestAsk: number | null
}

export function MarketSummary({
  symbol,
  summary,
  bestBid,
  bestAsk,
}: MarketSummaryProps) {
  const priceChange = summary?.priceChangePerc ?? 0
  const changeColor = priceChange > 0 ? 'teal' : priceChange < 0 ? 'red' : 'gray'

  return (
    <Paper className="market-summary" radius="md" p="lg" withBorder>
      <Group justify="space-between" align="flex-start" mb="lg">
        <Stack gap={2}>
          <Text className="eyebrow">BIST · EQUITIES</Text>
          <Text className="market-summary__symbol">{symbol}</Text>
        </Stack>
        <Badge color="teal" variant="light" size="sm">
          SIMULATED MARKET
        </Badge>
      </Group>

      <SimpleGrid cols={2} spacing="lg">
        <div>
          <Text className="metric-label">LAST TRADE</Text>
          <Text className="metric-value metric-value--primary">
            {formatMarketPrice(summary?.lastTradePrice ?? null)}
          </Text>
          <Badge color={changeColor} variant="light" mt={6}>
            {summary ? `${priceChange > 0 ? '+' : ''}${priceChange.toFixed(2)}%` : '—'}
          </Badge>
        </div>
        <div>
          <Text className="metric-label">BEST BID</Text>
          <Text className="metric-value">{formatMarketPrice(bestBid)}</Text>
        </div>
        <div>
          <Text className="metric-label">BEST ASK</Text>
          <Text className="metric-value">{formatMarketPrice(bestAsk)}</Text>
        </div>
        <div>
          <Text className="metric-label">DAY RANGE</Text>
          <Text className="metric-value metric-value--range">
            {summary
              ? `${formatMarketPrice(summary.lowerLimit)} – ${formatMarketPrice(summary.upperLimit)}`
              : '—'}
          </Text>
        </div>
      </SimpleGrid>
    </Paper>
  )
}
