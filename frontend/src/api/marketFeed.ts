import type {
  CurrentPriceMessage,
  MarketDepthSnapshot,
  MarketFeedMessage,
  TradeMessages,
} from '../types/market'

export type MarketFeedStatus = 'connecting' | 'live' | 'reconnecting'

type MarketFeedHandlers = {
  onMessage: (message: MarketFeedMessage) => void
  onStatusChange: (status: MarketFeedStatus) => void
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isMarketFeedMessage(value: unknown): value is MarketFeedMessage {
  if (!isRecord(value) || typeof value.symbol !== 'string') {
    return false
  }

  if ('buy' in value && 'sell' in value) {
    return Array.isArray(value.buy) && Array.isArray(value.sell)
  }

  if ('price' in value) {
    return typeof value.price === 'number'
  }

  return 'trades' in value && Array.isArray(value.trades)
}

function getWebSocketUrl() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/ws`
}

export function connectMarketFeed(
  symbol: string,
  handlers: MarketFeedHandlers,
) {
  let isClosed = false
  let reconnectTimer: number | undefined
  let socket: WebSocket | undefined

  function connect() {
    if (isClosed) {
      return
    }

    handlers.onStatusChange(socket ? 'reconnecting' : 'connecting')
    socket = new WebSocket(getWebSocketUrl())

    socket.onopen = () => {
      const topics = ['MARKET_DEPTH', 'CURRENT_PRICE', 'TRADES']

      for (const topic of topics) {
        socket?.send(JSON.stringify({ action: 'subscribe', symbol, topic }))
      }

      handlers.onStatusChange('live')
    }

    socket.onmessage = (event) => {
      try {
        const message: unknown = JSON.parse(event.data)

        if (isMarketFeedMessage(message) && message.symbol === symbol) {
          handlers.onMessage(message)
        }
      } catch {
        return
      }
    }

    socket.onclose = () => {
      if (!isClosed) {
        handlers.onStatusChange('reconnecting')
        reconnectTimer = window.setTimeout(connect, 2000)
      }
    }
  }

  connect()

  return () => {
    isClosed = true

    if (reconnectTimer !== undefined) {
      window.clearTimeout(reconnectTimer)
    }

    socket?.close()
  }
}

export function isDepthMessage(
  message: MarketFeedMessage,
): message is MarketDepthSnapshot {
  return 'buy' in message && 'sell' in message
}

export function isCurrentPriceMessage(
  message: MarketFeedMessage,
): message is CurrentPriceMessage {
  return 'price' in message
}

export function isTradesMessage(
  message: MarketFeedMessage,
): message is TradeMessages {
  return 'trades' in message
}
