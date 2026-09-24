import type { CSSProperties } from 'react'
import {
  Badge,
  Group,
  Paper,
  ScrollArea,
  Skeleton,
  Stack,
  Table,
  Text,
} from '@mantine/core'
import type { MarketDepthSnapshot, PriceLevel } from '../types/market'
import { formatMarketPrice, formatQuantity } from '../utils/marketFormat'

type MarketDepthProps = {
  snapshot: MarketDepthSnapshot | null
  errorMessage: string
}

type CumulativeLevel = PriceLevel & {
  total: number
}

function addCumulativeTotals(levels: PriceLevel[]): CumulativeLevel[] {
  let total = 0

  return levels.map((level) => {
    total += level.quantity

    return {
      ...level,
      total,
    }
  })
}

function getBarWidth(total: number, maximum: number) {
  return maximum > 0 ? `${(total / maximum) * 100}%` : '0%'
}

function getBarStyle(total: number, maximum: number): CSSProperties {
  return {
    '--depth-bar-width': getBarWidth(total, maximum),
  } as CSSProperties
}

function renderPrice(level: CumulativeLevel | undefined) {
  return level ? formatMarketPrice(level.price) : ''
}

function renderQuantity(level: CumulativeLevel | undefined) {
  return level ? formatQuantity(level.quantity) : ''
}

function renderTotal(level: CumulativeLevel | undefined) {
  return level ? formatQuantity(level.total) : ''
}

export function MarketDepth({ snapshot, errorMessage }: MarketDepthProps) {
  const bids = snapshot ? addCumulativeTotals(snapshot.buy) : []
  const asks = snapshot ? addCumulativeTotals(snapshot.sell) : []
  const bidMaximum = bids.at(-1)?.total ?? 0
  const askMaximum = asks.at(-1)?.total ?? 0
  const rowCount = Math.max(bids.length, asks.length)

  return (
    <Paper className="panel-card depth-panel" radius="md" p="lg" withBorder>
      <Group justify="space-between" mb="md">
        <Stack gap={2}>
          <Text className="eyebrow">LIVE BOOK</Text>
          <Text className="panel-title">Order book</Text>
        </Stack>
        <Badge color="gray" variant="light">20 LEVELS</Badge>
      </Group>

      {errorMessage ? (
        <Text c="red" size="sm">{errorMessage}</Text>
      ) : snapshot ? (
        <ScrollArea type="auto">
          <Table
            className="depth-book"
            aria-label={`${snapshot.symbol} order book`}
            miw={620}
            horizontalSpacing="xs"
            verticalSpacing={0}
          >
            <Table.Thead>
              <Table.Tr>
                <Table.Th className="depth-book__side depth-book__side--bid" colSpan={3}>
                  BIDS
                </Table.Th>
                <Table.Th className="depth-book__side depth-book__side--ask" colSpan={3}>
                  ASKS
                </Table.Th>
              </Table.Tr>
              <Table.Tr>
                <Table.Th>Price</Table.Th>
                <Table.Th ta="right">Amount</Table.Th>
                <Table.Th ta="right">Total</Table.Th>
                <Table.Th>Total</Table.Th>
                <Table.Th>Amount</Table.Th>
                <Table.Th ta="right">Price</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {rowCount === 0 ? (
                <Table.Tr>
                  <Table.Td colSpan={6} className="depth-book__empty">
                    No resting orders
                  </Table.Td>
                </Table.Tr>
              ) : (
                Array.from({ length: rowCount }, (_, index) => {
                  const bid = bids[index]
                  const ask = asks[index]

                  return (
                    <Table.Tr key={`${bid?.price ?? 'empty'}-${ask?.price ?? 'empty'}`}>
                      <Table.Td className="depth-book__price depth-book__price--bid">
                        {renderPrice(bid)}
                      </Table.Td>
                      <Table.Td className="depth-book__amount depth-book__amount--bid">
                        {renderQuantity(bid)}
                      </Table.Td>
                      <Table.Td
                        className="depth-book__total depth-book__total--bid"
                        style={getBarStyle(bid?.total ?? 0, bidMaximum)}
                      >
                        <span>{renderTotal(bid)}</span>
                      </Table.Td>
                      <Table.Td
                        className="depth-book__total depth-book__total--ask"
                        style={getBarStyle(ask?.total ?? 0, askMaximum)}
                      >
                        <span>{renderTotal(ask)}</span>
                      </Table.Td>
                      <Table.Td className="depth-book__amount depth-book__amount--ask">
                        {renderQuantity(ask)}
                      </Table.Td>
                      <Table.Td className="depth-book__price depth-book__price--ask">
                        {renderPrice(ask)}
                      </Table.Td>
                    </Table.Tr>
                  )
                })
              )}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      ) : (
        <Stack gap="sm">
          <Skeleton height={28} />
          <Skeleton height={28} />
          <Skeleton height={28} />
        </Stack>
      )}
    </Paper>
  )
}
