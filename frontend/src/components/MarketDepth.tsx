type MarketDepthProps = {
  symbol: string;
};

export function MarketDepth({  }: MarketDepthProps) {
  return (
    <section className="panel">
      <h2>Market Depth</h2>
      <p>Buy and sell price levels will appear here.</p>
    </section>
  ) 
}
