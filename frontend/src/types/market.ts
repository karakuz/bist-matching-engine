export type SymbolSummary = {
  openingPrice: number,
  lastTradePrice: number,
  priceChangePerc: number,
  lowerLimit: number,
  upperLimit: number,
  tickSize: number
}

export type PriceLevel = {
  price: number
  quantity: number
  orderCount: number
}

export type MarketDepthSnapshot = {
  symbol: string
  buy: PriceLevel[]
  sell: PriceLevel[]
}

export type MarketFeedMessage =
  | MarketDepthSnapshot
  | CurrentPriceMessage
  | TradeMessages

export type CurrentPriceMessage = {
  symbol: string
  price: number
}

export type TradeMessage = {
  tradeId: string
  symbol: string
  buyOrderId: string
  buyParticipantId: number
  buyParticipantName: string
  sellOrderId: string
  sellParticipantId: number
  sellParticipantName: string
  price: number
  quantity: number
  createdAt: string
}

export type TradeMessages = {
  symbol: string
  trades: TradeMessage[]
}
