type EventHistoryProps = {
  symbol: string;
};

export function EventHistory({  }: EventHistoryProps) {
  return (
    <section className="panel">
      <h2>Event History</h2>
      <p>Live market events will appear here.</p>
    </section>
  )
}
