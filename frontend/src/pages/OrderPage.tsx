import {
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type FormEvent,
  type KeyboardEvent,
} from 'react'
import { useSearchParams } from 'react-router'
import {
  createEngine,
  fmt,
  levels,
  type MarketEngine,
  type Level,
  type Side,
  type TapeTrade,
} from '../api/engine'
import { getAvailableSymbols } from '../api/market'
import './css/OrderPage.css'

type Theme = 'light' | 'dark'

type PricePick = {
  price: string
  nonce: number
}

function parsePrice(value: string) {
  const parsed = Number.parseFloat(value.trim().replace(',', '.'))
  return Number.isNaN(parsed) ? Number.NaN : Math.round(parsed * 100)
}

function OrderPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedSymbol = searchParams.get('symbol')?.trim().toUpperCase()
  const [availableStocks, setAvailableStocks] = useState<string[] | null>(null)
  const [stockListError, setStockListError] = useState('')
  const symbol = requestedSymbol || availableStocks?.[0] || ''
  const engine = useMemo(() => createEngine(symbol), [symbol])
  const market = useSyncExternalStore(
    engine.subscribe,
    engine.getState,
    engine.getState,
  )
  const [tab, setTab] = useState<'order' | 'open'>('order')
  const [pick, setPick] = useState<PricePick>({ price: '', nonce: 0 })
  const [theme, setTheme] = useState<Theme>('light')

  useEffect(() => {
    if (!symbol) return
    return engine.start()
  }, [engine, symbol])
  useEffect(() => {
    let active = true
    getAvailableSymbols()
      .then((stocks) => {
        if (active) setAvailableStocks(stocks)
      })
      .catch(() => {
        if (!active) return
        setAvailableStocks([])
        setStockListError('Unable to load available stocks')
      })

    return () => {
      active = false
    }
  }, [])
  useEffect(() => {
    if (requestedSymbol || !availableStocks?.length) return
    setSearchParams({ symbol: availableStocks[0] }, { replace: true })
  }, [availableStocks, requestedSymbol, setSearchParams])
  useEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])

  const bids = levels(market, 'buy')
  const asks = levels(market, 'sell')
  const bestBid = bids[0]?.p ?? null
  const bestAsk = asks[0]?.p ?? null
  const change = market.last !== null && market.open
    ? ((market.last - market.open) / market.open) * 100
    : null
  const spread = bestBid !== null && bestAsk !== null
    ? fmt(bestAsk - bestBid)
    : '—'
  const range = market.lowerLimit !== null && market.upperLimit !== null
    ? `${fmt(market.lowerLimit)} – ${fmt(market.upperLimit)}`
    : '—'

  function pickPrice(priceKurus: number) {
    setTab('order')
    setPick((current) => ({
      price: fmt(priceKurus),
      nonce: current.nonce + 1,
    }))
  }

  function selectStock(stock: string) {
    setSearchParams({ symbol: stock })
  }

  if (!symbol) {
    return <div className="page-message">{stockListError || 'Loading available stocks'}</div>
  }

  return (
    <>
      <header>
        <div className="sym">
          {symbol}
          <small>BIST · simulated</small>
        </div>
        <div className="stat">
          <span>Last</span>
          <b id="last" className={market.lastDir}>{fmt(market.last)}</b>
          <span
            style={{
              color:
                change === null
                  ? 'var(--dim)'
                  : change > 0
                    ? 'var(--buy)'
                    : change < 0
                      ? 'var(--sell)'
                      : 'var(--dim)',
            }}
          >
            {change === null
              ? '—'
              : `${change >= 0 ? '+' : ''}${change.toFixed(2).replace('.', ',')}%`}
          </span>
        </div>
        <div className="stat"><span>Bid</span><b>{fmt(bestBid)}</b></div>
        <div className="stat"><span>Ask</span><b>{fmt(bestAsk)}</b></div>
        <div className="stat"><span>Spread</span><b>{spread}</b></div>
        <div className="stat"><span>Range</span><b>{range}</b></div>
        <div className={`conn${market.connected ? '' : ' off'}`}>
          <i />
          <span>{market.connected ? 'Live' : 'Disconnected'}</span>
        </div>
        <button
          className="theme-switch"
          type="button"
          role="switch"
          aria-checked={theme === 'dark'}
          aria-label="Dark theme"
          onClick={() => setTheme((current) => current === 'light' ? 'dark' : 'light')}
        >
          <span className="theme-switch__track" aria-hidden="true">
            <span className="theme-switch__thumb" />
          </span>
          <span>{theme === 'dark' ? 'Dark' : 'Light'}</span>
        </button>
      </header>

      <main>
        <aside className="stocks">
          <h2>Stocks</h2>
          <nav className="stocks-list" aria-label="Available stocks">
            {stockListError && <div className="empty">{stockListError}</div>}
            {availableStocks?.map((stock) => (
              <button
                key={stock}
                type="button"
                className="stock-option"
                aria-current={symbol === stock ? 'page' : undefined}
                onClick={() => selectStock(stock)}
              >
                {stock}
              </button>
            ))}
          </nav>
        </aside>

        <div className="center">
          {market.error && <div className="empty" role="alert">{market.error}</div>}
          <section>
            <div className="books">
              <BookSide title="Buy orders" side="buy" rows={bids} onPick={pickPrice} />
              <BookSide title="Sell orders" side="sell" rows={asks} onPick={pickPrice} />
            </div>
          </section>
          <section>
            <h2>History</h2>
            <Trades rows={market.tape} />
          </section>
        </div>

        <section className="order">
          <div className="tabs" role="tablist" aria-label="Order panel">
            <button
              type="button"
              role="tab"
              aria-selected={tab === 'order'}
              onClick={() => setTab('order')}
            >
              Order
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={tab === 'open'}
              onClick={() => setTab('open')}
            >
              My orders
            </button>
          </div>
          {tab === 'order' ? (
            <OrderForm
              key={pick.nonce}
              engine={engine}
              pick={pick}
              symbol={symbol}
              tickSize={market.tickSize}
            />
          ) : (
            <MyOrders />
          )}
        </section>
      </main>
    </>
  )
}

type BookSideProps = {
  title: string
  side: Side
  rows: Level[]
  onPick: (priceKurus: number) => void
}

function BookSide({ title, side, rows, onPick }: BookSideProps) {
  const maximum = rows.at(-1)?.cum ?? 0
  const className = side === 'buy' ? 'bid' : 'ask'

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>, price: number) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      onPick(price)
    }
  }

  return (
    <div>
      <h2>{title}</h2>
      <div className="cols">
        <span>Price</span>
        <span className="num">Qty</span>
        <span className="num">Total</span>
      </div>
      {rows.length === 0 ? (
        <div className="empty">Waiting for market depth</div>
      ) : (
        rows.map((row) => (
          <div
            key={row.p}
            className={`row ${className}${row.mine ? ' mine' : ''}`}
            role="button"
            tabIndex={0}
            aria-label={`Select ${fmt(row.p)} price`}
            onClick={() => onPick(row.p)}
            onKeyDown={(event) => handleKeyDown(event, row.p)}
          >
            <i
              className="bar"
              aria-hidden="true"
              style={{ width: `${maximum > 0 ? (row.cum / maximum) * 80 : 0}%` }}
            />
            <span className="p">{fmt(row.p)}</span>
            <span className="num q">{row.q}</span>
            <span className="num">{row.cum}</span>
          </div>
        ))
      )}
    </div>
  )
}

function Trades({ rows }: { rows: TapeTrade[] }) {
  return (
    <>
      <div className="tbl tape head">
        <span className="num">Price</span>
        <span className="num">Qty</span>
        <span>Buyer</span>
        <span>Seller</span>
        <span className="num">Time</span>
      </div>
      {rows.length === 0 && (
        <div className="empty empty--history">No trades yet</div>
      )}
      {rows.map((trade) => (
        <div className="tbl tape" key={trade.id}>
          <span className="num">{fmt(trade.p)}</span>
          <span className="num">{trade.q}</span>
          <span className="buy-c" title={`Order ${trade.buyOrderId}`}>
            {trade.buyParticipantName || `Participant ${trade.buyParticipantId}`}
          </span>
          <span className="sell-c" title={`Order ${trade.sellOrderId}`}>
            {trade.sellParticipantName || `Participant ${trade.sellParticipantId}`}
          </span>
          <span className="num">{trade.t}</span>
        </div>
      ))}
    </>
  )
}

function MyOrders() {
  return (
    <div>
      <div className="tbl mine-t head">
        <span>Side</span>
        <span className="num">Price</span>
        <span className="num">Qty</span>
        <span className="num">Filled</span>
        <span>Action</span>
      </div>
      <div className="empty" role="status">
        My orders and cancellation are unavailable from the current API.
      </div>
      <h2 className="sub">Fills</h2>
      <div className="tbl tape head">
        <span className="num">Price</span>
        <span className="num">Qty</span>
        <span>Buyer</span>
        <span>Seller</span>
        <span className="num">Time</span>
      </div>
      <div className="empty" role="status">
        Participant-specific fills are unavailable from the current API.
      </div>
    </div>
  )
}

type OrderFormProps = {
  engine: MarketEngine
  pick: PricePick
  symbol: string
  tickSize: number
}

function OrderForm({ engine, pick, symbol, tickSize }: OrderFormProps) {
  const [side, setSide] = useState<Side>('buy')
  const [price, setPrice] = useState(pick.price)
  const [quantity, setQuantity] = useState('')
  const [message, setMessage] = useState({ text: '', ok: false })
  const [invalid, setInvalid] = useState({ price: false, quantity: false })
  const quantityInput = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (pick.nonce > 0) {
      quantityInput.current?.focus()
    }
  }, [pick.nonce])

  const priceKurus = parsePrice(price)
  const parsedQuantity = Number(quantity)
  const estimatedTotal = priceKurus > 0 && parsedQuantity > 0
    ? `₺${fmt(priceKurus * parsedQuantity)}`
    : '–'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const validPrice =
      Number.isSafeInteger(priceKurus) &&
      priceKurus > 0 &&
      priceKurus % tickSize === 0
    const validQuantity = Number.isSafeInteger(parsedQuantity) && parsedQuantity > 0

    setInvalid({ price: !validPrice, quantity: !validQuantity })

    if (!validPrice) {
      setMessage({ text: `Enter a price aligned to the ${fmt(tickSize)} tick.`, ok: false })
      return
    }

    if (!validQuantity) {
      setMessage({ text: 'Enter a quantity of at least 1.', ok: false })
      return
    }

    try {
      const response = await engine.submit(side, priceKurus, parsedQuantity)
      setMessage({ text: response.message || 'Order queued.', ok: true })
      setQuantity('')
    } catch (error) {
      setMessage({
        text: error instanceof Error ? error.message : 'Order submission failed.',
        ok: false,
      })
    }
  }

  return (
    <form onSubmit={submit} noValidate>
      <div className="seg" role="group" aria-label="Side">
        <button
          type="button"
          className="b"
          aria-pressed={side === 'buy'}
          onClick={() => setSide('buy')}
        >
          Buy
        </button>
        <button
          type="button"
          className="s"
          aria-pressed={side === 'sell'}
          onClick={() => setSide('sell')}
        >
          Sell
        </button>
      </div>
      <label>
        Price (TL, tick {fmt(tickSize)})
        <input
          className={invalid.price ? 'bad' : ''}
          inputMode="decimal"
          autoComplete="off"
          placeholder="0,00"
          value={price}
          onChange={(event) => setPrice(event.target.value)}
        />
      </label>
      <label>
        Quantity
        <input
          ref={quantityInput}
          className={invalid.quantity ? 'bad' : ''}
          inputMode="numeric"
          autoComplete="off"
          placeholder="0"
          value={quantity}
          onChange={(event) => setQuantity(event.target.value)}
        />
      </label>
      <div className="est"><span>Estimated total</span><b>{estimatedTotal}</b></div>
      <button type="submit" className={`go${side === 'sell' ? ' s' : ''}`}>
        {side === 'buy' ? 'Buy' : 'Sell'} {symbol}
      </button>
      <div className={`msg${message.ok ? ' ok' : ''}`} role="status">
        {message.text}
      </div>
    </form>
  )
}

export default OrderPage
