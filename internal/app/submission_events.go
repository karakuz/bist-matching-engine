package app

import (
	"encoding/json"
	"fmt"
	"time"

	"bist-matching-engine/internal/domain"
	"bist-matching-engine/internal/matching"
)

func newSubmissionEvent(orderID string, eventType domain.OrderEventType, value any) (domain.OrderEvent, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return domain.OrderEvent{}, fmt.Errorf("marshal event payload: %w", err)
	}

	event, err := domain.NewOrderEvent(
		orderID,
		eventType,
		payload,
		time.Now().UTC(),
	)
	if err != nil {
		return domain.OrderEvent{}, err
	}

	return *event, nil
}

func finalOrderEventType(order domain.Order) domain.OrderEventType {
	switch order.Status {
	case domain.StatusFilled:
		return domain.EventOrderFilled

	case domain.StatusPartiallyFilled:
		return domain.EventOrderPartiallyFilled

	default:
		return domain.EventOrderAddedToBook
	}
}

func buildSubmissionEvents(plan matching.MatchPlan) ([]domain.OrderEvent, error) {
	events := make([]domain.OrderEvent, 0, 2+len(plan.RestingOrderUpdates)+len(plan.Trades))

	acceptedEvent, err := newSubmissionEvent(
		plan.IncomingOrder.ID,
		domain.EventOrderAccepted,
		plan.IncomingOrder,
	)
	if err != nil {
		return nil, err
	}

	events = append(events, acceptedEvent)

	finalIncomingEvent, err := newSubmissionEvent(
		plan.IncomingOrder.ID,
		finalOrderEventType(plan.IncomingOrder),
		plan.IncomingOrder,
	)
	if err != nil {
		return nil, err
	}

	events = append(events, finalIncomingEvent)

	for _, restingOrder := range plan.RestingOrderUpdates {
		event, err := newSubmissionEvent(
			restingOrder.ID,
			finalOrderEventType(restingOrder),
			restingOrder,
		)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	for _, trade := range plan.Trades {
		event, err := newSubmissionEvent(
			plan.IncomingOrder.ID,
			domain.EventTradeCreated,
			trade,
		)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, nil
}