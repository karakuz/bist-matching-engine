import { useState, type FormEvent } from 'react'
import type {
  OrderSide,
  SubmitOrderRequest,
} from '../types/order'

import {submitOrderRequest} from "../api/orders"

type OrderEntryProps = {
  symbol: string;
};

const PARTICIPANT_ID = 2;//TODO: Dynamic

export function OrderEntry({symbol}: OrderEntryProps) {
  const [price, setPrice] = useState('')
  const [quantity, setQuantity] = useState('')
  const [side, setSide] = useState<OrderSide>('BUY')
  const [submissionMessage, setSubmissionMessage] = useState('')

  async function submitOrder(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const request: SubmitOrderRequest = {
      participantId: PARTICIPANT_ID,
      symbol,
      side,
      price: Number(price),
      quantity: Number(quantity),
    }

    try {
      const response = await submitOrderRequest(request)
      setSubmissionMessage(response.message)
    } catch (error) {
      const message = error instanceof Error
        ? error.message
        : 'Unexpected order submission error'
      setSubmissionMessage(message)
    }
  }
  

  return (
    <section className="panel">
      <h2>Order Entry</h2>
      <form onSubmit={submitOrder}>
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
            gap: '1rem',
            padding: '1rem 0',
          }}
        >
          <div>
            <label htmlFor="orderEntryPrice">Price</label>
            <input
              style={{ width: '100%' }}
              type="number"
              id="orderEntryPrice"
              min="1"
              required
              value={price}
              onChange={(event) => setPrice(event.target.value)}
            />
          </div>

          <div>
            <label htmlFor="orderEntryQuantity">Quantity</label>
            <input
              style={{ width: '100%' }}
              type="number"
              id="orderEntryQuantity"
              min="1"
              required
              value={quantity}
              onChange={(event) => setQuantity(event.target.value)}
            />
          </div>
        </div>

        <div>
          <select
            name="sideSelect"
            id="orderEntrySideSelect"
            value={side}
            onChange={(event) =>
              setSide(event.target.value === 'BUY' ? 'BUY' : 'SELL')
            }
          >
            <option value="BUY">BUY</option>
            <option value="SELL">SELL</option>
          </select>
        </div>
        
        <div>
          <button>Send</button>
        </div>
      </form>
      <div>
        {submissionMessage && <p>{submissionMessage}</p>}
      </div>
    </section>
  )
}
