# Frontend

The frontend is the React and TypeScript trading application. It displays the
selected symbol's market summary and order book, accepts limit orders, and
shows trade executions received while the page is open.

## Run locally

Start the Go API from `matching-engine-go` and keep it listening on port 8080.
Then run these commands from this `frontend` directory:

```powershell
npm install
npm run dev
```

Vite proxies HTTP requests and the `/ws` WebSocket connection to the Go API.
The default route opens ASELS; another symbol can be selected through the
`symbol` query parameter, for example `/order?symbol=ASELS`.

## Market updates

The page loads the summary and 20 levels of market depth over HTTP while it
opens a WebSocket subscription to the symbol's `MARKET_DEPTH`, `CURRENT_PRICE`,
and `TRADES` topics. Trade activity begins when the page opens and is held in
browser memory for that session; the backend does not currently expose a trade
history endpoint.

Order submissions use the existing API request shape and demo participant ID.
Persistent user identity and order ownership are planned for a later phase.
The `My orders`, participant-specific fills, and cancellation actions are shown
as unavailable because the current API does not expose those operations.

## Checks

Run these from this directory:

```powershell
npm run build
npm run lint
```

## Source structure

```text
src/
  api/          HTTP requests and market WebSocket connection
  components/   Market summary, depth, order entry, and trade activity
  pages/        Routed trading screen and page styles
  types/        TypeScript representations of API and feed messages
```
