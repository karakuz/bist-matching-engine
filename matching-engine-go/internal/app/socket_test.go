package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestClientMessageUnmarshalsSymbol(t *testing.T) {
	data := []byte(`{"action":"subscribe","symbol":"ASELS","topic":"MARKET_DEPTH"}`)

	var message ClientMessage
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("Expected client message to unmarshal, got: %v", err)
	}

	if message.Symbol != "ASELS" {
		t.Fatalf("Expected symbol to be ASELS, got: %s", message.Symbol)
	}
}

func TestWebSocketWriterBatchesQueuedMessagesInOrder(t *testing.T) {
	messages := []json.RawMessage{
		json.RawMessage(`{"symbol":"ASELS","price":35780}`),
		json.RawMessage(`{"symbol":"ASELS","trades":[{"tradeId":"trade-1"}]}`),
		json.RawMessage(`{"symbol":"ASELS","buy":[],"sell":[]}`),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		clientCtx, clientCancel := context.WithCancel(ctx)
		client := &websocketClient{
			conn: conn, outbound: make(chan []byte, len(messages)), ctx: clientCtx, cancel: clientCancel,
		}
		for _, message := range messages {
			client.outbound <- message
		}
		NewWebSocketServer().writeMessages(client)
	}))
	defer server.Close()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.CloseNow()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read batch: %v", err)
	}
	var batch []json.RawMessage
	if err := json.Unmarshal(data, &batch); err != nil {
		t.Fatalf("expected JSON array, got %s: %v", data, err)
	}
	if len(batch) != len(messages) {
		t.Fatalf("expected %d messages in one write, got %d", len(messages), len(batch))
	}
	for index, message := range batch {
		if string(message) != string(messages[index]) {
			t.Fatalf("message %d: expected %s, got %s", index, messages[index], message)
		}
	}
}
