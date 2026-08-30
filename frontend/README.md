# Frontend

This is the React and TypeScript user-facing trading application. Vite provides
the local development server and production build.

The first implementation will consume the Go API and symbol-aware WebSocket
feed to display market depth, current prices, and trades, and will allow users
to submit orders. It is not limited to a read-only market dashboard.

Authentication, authorization, persistent user identity, and order ownership
are planned future capabilities and are not implemented yet.

## Commands

Run these commands from this `frontend` directory:

```powershell
npm install
npm run dev
```

Use `npm run build` to type-check the application and produce a production
bundle. Use `npm run lint` to run the configured source-code checks.

## Source structure

```text
src/
  api/          HTTP and WebSocket communication
  components/   Visible screen sections
  types/        TypeScript representations of backend messages
  App.tsx       Screen composition and shared application state
  main.tsx      Browser entry point
```
