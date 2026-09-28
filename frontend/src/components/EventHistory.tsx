import { Badge, Paper, ScrollArea, Table, Text } from '@mantine/core'
import type { TradeMessage } from '../types/market'
import {
  formatMarketPrice,
  formatQuantity,
  formatTradeTime,
} from '../utils/marketFormat'

type EventHistoryProps = {
  trades: TradeMessage[]
}

export function EventHistory({ trades }: EventHistoryProps) {
  return (
    <Paper className="panel-card" radius="md" p="lg" withBorder>
      <div className="activity-heading">
        <div>
          <Text className="eyebrow">AFTER PAGE LOAD</Text>
          <Text className="panel-title">Trade activity</Text>
        </div>
        <Badge color="gray" variant="light">{trades.length} EXECUTIONS</Badge>
      </div>

      {trades.length === 0 ? (
        <Text c="dimmed" size="sm" py="xl" ta="center">
          Matched trades will appear here as they happen.
        </Text>
      ) : (
        <ScrollArea>
          <Table className="activity-table" miw={620} highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Time</Table.Th>
                <Table.Th>Price</Table.Th>
                <Table.Th ta="right">Quantity</Table.Th>
                <Table.Th>Buy order</Table.Th>
                <Table.Th>Sell order</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {trades.map((trade) => (
                <Table.Tr key={trade.tradeId}>
                  <Table.Td>{formatTradeTime(trade.createdAt)}</Table.Td>
                  <Table.Td className="activity-table__price">
                    {formatMarketPrice(trade.price)}
                  </Table.Td>
                  <Table.Td ta="right">
                    {formatQuantity(trade.quantity)}
                  </Table.Td>
                  <Table.Td className="activity-table__id">{trade.buyOrderId}</Table.Td>
                  <Table.Td className="activity-table__id">{trade.sellOrderId}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      )}
    </Paper>
  )
}
