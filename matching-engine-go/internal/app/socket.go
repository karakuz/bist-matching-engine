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

var (
	websocketOutboundCapacity = 1024
	websocketWriteTimeout     = 2 * time.Second
)

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
	clients       map[*websocket.Conn]*websocketClient
}

type websocketClient struct {
	conn     *websocket.Conn
	outbound chan []byte
	ctx      context.Context
	cancel   context.CancelFunc
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
		clients:       make(map[*websocket.Conn]*websocketClient),
	}
}

func (s *WebSocketServer) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Println("websocket accept error:", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())

	client := &websocketClient{
		conn:     conn,
		outbound: make(chan []byte, websocketOutboundCapacity),
		ctx:      ctx,
		cancel:   cancel,
	}

	s.mu.Lock()
	s.clients[conn] = client
	s.mu.Unlock()

	go s.writeMessages(client)

	defer func() {
		s.RemoveConnection(conn)
		conn.CloseNow()
	}()

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

func (s *WebSocketServer) writeMessages(client *websocketClient) {
	defer client.cancel()
	defer client.conn.CloseNow()

	for {
		select {
		case <-client.ctx.Done():
			return

		case data := <-client.outbound:
			// Drain only the messages already waiting, so continuous arrivals
			// cannot postpone this write indefinitely.
			pending := len(client.outbound)
			if pending > 0 {
				messages := make([]json.RawMessage, 0, pending+1)
				messages = append(messages, json.RawMessage(data))
				for range pending {
					messages = append(messages, json.RawMessage(<-client.outbound))
				}
				var err error
				data, err = json.Marshal(messages)
				if err != nil {
					log.Printf("websocket batch encoding error: %v", err)
					return
				}
			}
			writeCtx, cancel := context.WithTimeout(client.ctx, websocketWriteTimeout)
			err := client.conn.Write(writeCtx, websocket.MessageText, data)
			cancel()

			if err != nil {
				log.Printf("websocket write error: %v", err)
				return
			}
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

func (s *WebSocketServer) RemoveConnection(conn *websocket.Conn) {
	s.mu.Lock()

	client := s.clients[conn]
	delete(s.clients, conn)

	for topic, connections := range s.subscriptions {
		delete(connections, conn)

		if len(connections) == 0 {
			delete(s.subscriptions, topic)
		}
	}

	s.mu.Unlock()

	if client != nil {
		client.cancel()
	}
}

func (s *WebSocketServer) Publish(
	ctx context.Context,
	topic WsTopic,
	symbol string,
	data []byte,
) {
	if ctx.Err() != nil {
		return
	}

	s.mu.RLock()
	subKey := getSubKey(symbol, topic)
	clients := make([]*websocketClient, 0, len(s.subscriptions[subKey]))

	for conn := range s.subscriptions[subKey] {
		if client := s.clients[conn]; client != nil {
			clients = append(clients, client)
		}
	}
	s.mu.RUnlock()

	for _, client := range clients {
		if client.ctx.Err() != nil {
			continue
		}

		select {
		case <-client.ctx.Done():
		case client.outbound <- data:
		default:
			log.Printf("websocket outbound queue full: symbol=%s topic=%s; disconnecting client", symbol, topic)
			client.cancel()
			client.conn.CloseNow()
		}
	}
}
