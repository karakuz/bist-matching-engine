import { useSearchParams } from 'react-router'
import { EventHistory } from '../components/EventHistory'
import { MarketDepth } from '../components/MarketDepth'
import { MarketSummary } from '../components/MarketSummary'
import { OrderEntry } from '../components/OrderEntry'

import './css/OrderPage.css'
import { getSymbolSummary } from '../api/market'
import { useEffect, useState } from 'react'
import type { SymbolSummary } from '../types/market'

function OrderPage() {
  const [searchParams] = useSearchParams()
  const [symbolSummary, setSymbolSummary] = useState<SymbolSummary | null>(null)
  const [errorMessage, setErrorMessage] = useState('')

  const symbol = searchParams.get('symbol')?.trim().toUpperCase()

  useEffect(() => {
    if (!symbol) {
      return
    }

    const selectedSymbol = symbol

    async function loadSymbolSummary() {
      try {
        setErrorMessage('')
        setSymbolSummary(null)

        const summary = await getSymbolSummary(selectedSymbol)
        setSymbolSummary(summary)
      } catch (error) {
        setErrorMessage(
          error instanceof Error
            ? error.message
            : 'Could not load symbol summary',
        )
      }
    }

    loadSymbolSummary()
  }, [symbol])

  if (!symbol) {
    return <p>Symbol is required.</p>
  }

  if (errorMessage) {
    return <p>{errorMessage}</p>
  }

  if (!symbolSummary) {
    return <p>Loading {symbol}...</p>
  }

  return (
    <main className="trading-screen">
      <MarketSummary
         key={symbol}
        symbol={symbol}
        symbolSummary={symbolSummary}
      />

      <div className="trading-screen__top">
        <MarketDepth key={symbol} symbol={symbol} />
        <OrderEntry symbol={symbol} />
      </div>

      <EventHistory symbol={symbol} />
    </main>
  )
}

export default OrderPage