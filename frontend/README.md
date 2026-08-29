# Frontend

This directory will contain the user-facing trading application.

The first implementation will consume the Go API and symbol-aware WebSocket
feed to display market depth, current prices, and trades, and will allow users
to submit orders. It is not limited to a read-only market dashboard.

Authentication, authorization, persistent user identity, and order ownership
are planned future capabilities and are not implemented yet.
