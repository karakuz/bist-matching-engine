import { useState } from "react";
import type { SymbolSummary } from "../types/market";

type MarketSummaryProps = {
  symbol: string;
  symbolSummary: SymbolSummary
};

export function MarketSummary({ symbol, symbolSummary }: MarketSummaryProps) {
   const [openingPrice, setOpeningPrice] = useState(symbolSummary.openingPrice)
   const [lastTradePrice, setLastTradePrice] = useState(symbolSummary.lastTradePrice)
   const [priceChangePerc, setPriceChangePerc] = useState(symbolSummary.priceChangePerc)
   const [lowerLimit, setLowerLimit] = useState(symbolSummary.lowerLimit)
   const [upperLimit, setUpperLimit] = useState(symbolSummary.upperLimit)
   const [tickSize, setTickSize] = useState(symbolSummary.tickSize)

  return (
    <header className="panel">
      <h1>{symbol} Market</h1>
      <div>
        <span>Current: {(lastTradePrice/100).toFixed(2)}₺</span>
        <span>{priceChangePerc}%</span>
      </div>
    </header>
  )
}
