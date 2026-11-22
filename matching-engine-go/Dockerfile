FROM golang:1.25 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /matching-engine ./cmd/api

FROM debian:bookworm-slim
COPY --from=build /matching-engine /usr/local/bin/matching-engine
COPY --from=build /src/docs /app/docs
WORKDIR /app
ENTRYPOINT ["/usr/local/bin/matching-engine"]
