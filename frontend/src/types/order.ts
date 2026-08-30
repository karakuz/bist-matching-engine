export type OrderSide = 'BUY' | 'SELL'

export type SubmitOrderRequest = {
  participantId: number
  symbol: string
  side: OrderSide
  price: number
  quantity: number
}

export type SubmitOrderResponse = {
  message: string
}
