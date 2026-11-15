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

type SubscriptionKey struct {
	Symbol string
	Topic  WsTopic
}

type WebSocketServer struct {
	mu            sync.RWMutex
	subscriptions map[SubscriptionKey]map[*websocket.Conn]struct{}
}

type ClientMessage struct {
	Action string  `json:"action"`
	Topic  WsTopic `json:"topic"`
	Symbol string  `json:"symbol"`
}

type LastTradePriceMessage struct {
	Price  int64  `json:"price"`
	Symbol string `json:"symbol"`
}

type TradeMessage struct {
	TradeID             string    `json:"tradeId"`
	Symbol              string    `json:"symbol"`
	BuyOrderID          string    `json:"buyOrderId"`
	BuyParticipantID    int64     `json:"buyParticipantId"`
	BuyParticipantName  string    `json:"buyParticipantName"`
	SellOrderID         string    `json:"sellOrderId"`
	SellParticipantID   int64     `json:"sellParticipantId"`
	SellParticipantName string    `json:"sellParticipantName"`
	Price               int64     `json:"price"`
	Quantity            int64     `json:"quantity"`
	CreatedAt           time.Time `json:"createdAt"`
}

type TradeMessages struct {
	Symbol string         `json:"symbol"`
	Trades []TradeMessage `json:"trades"`
}

func NewWebSocketServer() *WebSocketServer {
	return &WebSocketServer{
		subscriptions: make(map[SubscriptionKey]map[*websocket.Conn]struct{}),
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
			s.Subscribe(message.Topic, message.Symbol, conn)

		case "unsubscribe":
			s.Unsubscribe(message.Topic, message.Symbol, conn)
		}
	}
}

func getSubKey(symbol string, topic WsTopic) SubscriptionKey {
	return SubscriptionKey{
		Symbol: symbol,
		Topic:  topic,
	}
}

func (s *WebSocketServer) Subscribe(
	topic WsTopic,
	symbol string,
	conn *websocket.Conn,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subKey := getSubKey(symbol, topic)

	if s.subscriptions[subKey] == nil {
		s.subscriptions[subKey] = make(map[*websocket.Conn]struct{})
	}

	s.subscriptions[subKey][conn] = struct{}{}

	log.Println("subscribed to:", subKey)
}

func (s *WebSocketServer) Unsubscribe(
	topic WsTopic,
	symbol string,
	conn *websocket.Conn,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subKey := getSubKey(symbol, topic)
	connections := s.subscriptions[subKey]

	if connections == nil {
		return
	}

	delete(connections, conn)

	if len(connections) == 0 {
		delete(s.subscriptions, subKey)
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
	symbol string,
	data []byte,
) {
	s.mu.RLock()

	connections := make([]*websocket.Conn, 0)
	subKey := getSubKey(symbol, topic)

	for conn := range s.subscriptions[subKey] {
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
