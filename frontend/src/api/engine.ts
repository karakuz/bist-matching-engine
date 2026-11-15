import { getMarketDepth, getSymbolSummary } from './market'
import { connectMarketFeed, isCurrentPriceMessage, isDepthMessage, isTradesMessage } from './marketFeed'
import { submitOrderRequest } from './orders'
import type {
  MarketDepthSnapshot,
  SymbolSummary,
  TradeMessage,
} from '../types/market'
import type { OrderSide, SubmitOrderRequest } from '../types/order'

export const LEVELS = 7
export const DEMO_PARTICIPANT_ID = 2

export type Side = 'buy' | 'sell'

export type Level = {
  p: number
  q: number
  cum: number
  mine: boolean
}

export type TapeTrade = {
  id: string
  p: number
  q: number
  buyOrderId: string
  buyParticipantId: number
  buyParticipantName: string
  sellOrderId: string
  sellParticipantId: number
  sellParticipantName: string
  t: string
}

export type MyOrder = {
  id: string
  side: Side
  p: number
  q: number
  f: number
}

export type MarketState = {
  bids: Record<number, number>
  asks: Record<number, number>
  last: number | null
  lastDir: 'up' | 'down' | ''
  open: number | null
  lowerLimit: number | null
  upperLimit: number | null
  tickSize: number
  tape: TapeTrade[]
  mine: MyOrder[]
  fills: TapeTrade[]
  connected: boolean
  error: string
}

export type MarketEngine = ReturnType<typeof createEngine>

const initialState: MarketState = {
  bids: {},
  asks: {},
  last: null,
  lastDir: '',
  open: null,
  lowerLimit: null,
  upperLimit: null,
  tickSize: 1,
  tape: [],
  mine: [],
  fills: [],
  connected: false,
  error: '',
}

const toPriceMap = (snapshot: MarketDepthSnapshot, side: Side) => {
  const levels = side === 'buy' ? snapshot.buy : snapshot.sell

  return levels.reduce<Record<number, number>>((book, level) => {
    book[level.price] = level.quantity
    return book
  }, {})
}

function formatTime(createdAt: string) {
  return new Intl.DateTimeFormat('tr-TR', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(new Date(createdAt))
}

function toTapeTrade(trade: TradeMessage): TapeTrade {
  return {
    id: trade.tradeId,
    p: trade.price,
    q: trade.quantity,
    buyOrderId: trade.buyOrderId,
    buyParticipantId: trade.buyParticipantId,
    buyParticipantName: trade.buyParticipantName,
    sellOrderId: trade.sellOrderId,
    sellParticipantId: trade.sellParticipantId,
    sellParticipantName: trade.sellParticipantName,
    t: formatTime(trade.createdAt),
  }
}

export const fmt = (priceKurus: number | null) =>
  priceKurus === null
    ? '—'
    : (priceKurus / 100).toLocaleString('tr-TR', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      })

export function levels(state: MarketState, side: Side): Level[] {
  const book = side === 'buy' ? state.bids : state.asks
  const mine = state.mine.filter((order) => order.side === side)
  const mineOn = (price: number) =>
    mine
      .filter((order) => order.p === price)
      .reduce((total, order) => total + order.q, 0)
  const prices = new Set([
    ...Object.keys(book).map(Number),
    ...mine.map((order) => order.p),
  ])
  const sortedPrices = [...prices].sort((left, right) =>
    side === 'buy' ? right - left : left - right,
  )

  let cumulativeQuantity = 0

  return sortedPrices.slice(0, LEVELS).map((price) => {
    const ownQuantity = mineOn(price)
    const quantity = (book[price] ?? 0) + ownQuantity
    cumulativeQuantity += quantity

    return {
      p: price,
      q: quantity,
      cum: cumulativeQuantity,
      mine: ownQuantity > 0,
    }
  })
}

export function createEngine(symbol: string) {
  let state: MarketState = { ...initialState }
  const subscribers = new Set<() => void>()

  function emit(update: Partial<MarketState>) {
    state = { ...state, ...update }
    subscribers.forEach((subscriber) => subscriber())
  }

  function enqueueTrades(trades: TapeTrade[]) {
    const knownTradeIds = new Set(state.tape.map((trade) => trade.id))
    const newTrades: TapeTrade[] = []

    for (const trade of trades) {
      if (!knownTradeIds.has(trade.id)) {
        newTrades.push(trade)
        knownTradeIds.add(trade.id)
      }
    }

    if (newTrades.length > 0) {
      emit({ tape: [...newTrades.reverse(), ...state.tape].slice(0, 60) })
    }
  }

  function applySummary(summary: SymbolSummary, latestPrice: number | null) {
    emit({
      last: latestPrice ?? summary.lastTradePrice,
      open: summary.openingPrice,
      lowerLimit: summary.lowerLimit,
      upperLimit: summary.upperLimit,
      tickSize: summary.tickSize,
      error: '',
    })
  }

  async function submit(side: Side, priceKurus: number, quantity: number) {
    const request: SubmitOrderRequest = {
      participantId: DEMO_PARTICIPANT_ID,
      symbol,
      side: side.toUpperCase() as OrderSide,
      price: priceKurus,
      quantity,
    }

    return submitOrderRequest(request)
  }

  function cancel(orderId: string): never {
    throw new Error(`Cancelling order ${orderId} is unavailable in the current API.`)
  }

  function start() {
    let isStopped = false
    let hasReceivedDepthUpdate = false
    let latestPrice: number | null = null

    getSymbolSummary(symbol)
      .then((summary) => {
        if (!isStopped) {
          applySummary(summary, latestPrice)
        }
      })
      .catch((error: unknown) => {
        if (!isStopped) {
          emit({ error: getMessage(error) })
        }
      })

    getMarketDepth(symbol, LEVELS)
      .then((snapshot) => {
        if (!isStopped && !hasReceivedDepthUpdate) {
          emit({
            bids: toPriceMap(snapshot, 'buy'),
            asks: toPriceMap(snapshot, 'sell'),
            error: '',
          })
        }
      })
      .catch((error: unknown) => {
        if (!isStopped && !hasReceivedDepthUpdate) {
          emit({ error: getMessage(error) })
        }
      })

    const disconnect = connectMarketFeed(symbol, {
      onStatusChange: (status) => {
        if (!isStopped) {
          emit({ connected: status === 'live' })
        }
      },
      onMessage: (message) => {
        if (isStopped) {
          return
        }

        if (isDepthMessage(message)) {
          hasReceivedDepthUpdate = true
          emit({
            bids: toPriceMap(message, 'buy'),
            asks: toPriceMap(message, 'sell'),
            error: '',
          })
          return
        }

        if (isCurrentPriceMessage(message)) {
          latestPrice = message.price
          const previousPrice = state.last
          const lastDir = previousPrice === null || previousPrice === message.price
            ? state.lastDir
            : message.price > previousPrice
              ? 'up'
              : 'down'

          emit({ last: message.price, lastDir })
          return
        }

        if (isTradesMessage(message)) {
          enqueueTrades(message.trades.map(toTapeTrade))
        }
      },
    })

    return () => {
      isStopped = true
      disconnect()
    }
  }

  function subscribe(subscriber: () => void) {
    subscribers.add(subscriber)
    return () => subscribers.delete(subscriber)
  }

  function getState() {
    return state
  }

  return {
    getState,
    subscribe,
    submit,
    cancel,
    start,
  }
}

function getMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message
  }

  return 'The market request could not be completed.'
}
