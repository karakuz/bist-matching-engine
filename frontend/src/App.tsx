import './App.css'
import { EventHistory } from './components/EventHistory'
import { MarketDepth } from './components/MarketDepth'
import { MarketSummary } from './components/MarketSummary'
import { OrderEntry } from './components/OrderEntry'

function App() {
  return (
    <main className="trading-screen">
      <MarketSummary />

      <div className="trading-screen__top">
        <MarketDepth />
        <OrderEntry />
      </div>

      <EventHistory />
    </main>
  )
}

export default App
