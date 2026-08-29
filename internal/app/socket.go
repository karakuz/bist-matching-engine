package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type WsTopic string

const (
	CurrentPriceTopic WsTopic = "CURRENT_PRICE"
	MarketDepthTopic  WsTopic = "MARKET_DEPTH"
	TradesTopic       WsTopic = "TRADES"
)

func (t WsTopic) IsValid() bool {
	switch t {
	case CurrentPriceTopic, MarketDepthTopic, TradesTopic:
		return true
	}
	return false
}

type WebSocketServer struct {
	mu            sync.RWMutex
	subscriptions map[WsTopic]map[*websocket.Conn]struct{}
}

type ClientMessage struct {
	Action string  `json:"action"`
	Topic  WsTopic `json:"topic"`
}

type LastTradePriceMessage struct {
	Price int64 `json:"price"`
}

type TradeMessage struct {
	TradeID     string    `json:"tradeId"`
	Symbol      string    `json:"symbol"`
	BuyOrderID  string    `json:"buyOrderId"`
	SellOrderID string    `json:"sellOrderId"`
	Price       int64     `json:"price"`
	Quantity    int64     `json:"quantity"`
	CreatedAt   time.Time `json:"createdAt"`
}

type TradeMessages struct {
	Symbol string         `json:"symbol"`
	Trades []TradeMessage `json:"trades"`
}

func NewWebSocketServer() *WebSocketServer {
	return &WebSocketServer{
		subscriptions: make(map[WsTopic]map[*websocket.Conn]struct{}),
	}
}

func (s *WebSocketServer) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Println("websocket accept error:", err)
		return
	}

	defer func() {
		s.RemoveConnection(conn)

		conn.Close(
			websocket.StatusNormalClosure,
			"connection closed",
		)
	}()

	ctx := r.Context()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			log.Println("websocket read error:", err)
			return
		}

		var message ClientMessage

		err = json.Unmarshal(data, &message)
		if err != nil {
			log.Println("invalid websocket message:", err)
			continue
		}

		switch message.Action {
		case "subscribe":
			s.Subscribe(message.Topic, conn)

		case "unsubscribe":
			s.Unsubscribe(message.Topic, conn)
		}
	}
}

func (s *WebSocketServer) Subscribe(
	topic WsTopic,
	conn *websocket.Conn,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.subscriptions[topic] == nil {
		s.subscriptions[topic] = make(map[*websocket.Conn]struct{})
	}

	s.subscriptions[topic][conn] = struct{}{}

	log.Println("subscribed to:", topic)
}

func (s *WebSocketServer) Unsubscribe(
	topic WsTopic,
	conn *websocket.Conn,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	connections := s.subscriptions[topic]

	if connections == nil {
		return
	}

	delete(connections, conn)

	if len(connections) == 0 {
		delete(s.subscriptions, topic)
	}

	log.Println("unsubscribed from:", topic)
}

func (s *WebSocketServer) RemoveConnection(
	conn *websocket.Conn,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for topic, connections := range s.subscriptions {
		delete(connections, conn)

		if len(connections) == 0 {
			delete(s.subscriptions, topic)
		}
	}
}

func (s *WebSocketServer) Publish(
	ctx context.Context,
	topic WsTopic,
	data []byte,
) {
	s.mu.RLock()

	connections := make([]*websocket.Conn, 0)

	for conn := range s.subscriptions[topic] {
		connections = append(connections, conn)
	}

	s.mu.RUnlock()

	for _, conn := range connections {
		err := conn.Write(
			ctx,
			websocket.MessageText,
			data,
		)

		if err != nil {
			log.Println("websocket write error:", err)
		}
	}
}
