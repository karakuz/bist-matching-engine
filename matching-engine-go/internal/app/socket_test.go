package app

import (
	"encoding/json"
	"testing"
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
