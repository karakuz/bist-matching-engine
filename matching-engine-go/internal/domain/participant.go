package domain

import (
	"errors"
	"strings"
)

type ParticipantType string

const (
	ParticipantTypeBroker ParticipantType = "BROKER"
	ParticipantTypeTrader ParticipantType = "TRADER"
)

type Participant struct {
	ID   int64
	Name string
	Type ParticipantType
}

var (
	ErrInvalidParticipantID   = errors.New("participant id cannot be <= 0")
	ErrEmptyParticipantName   = errors.New("participant name cannot be empty")
	ErrInvalidParticipantType = errors.New("participant type must be BROKER or TRADER")
)

func NewParticipant(id int64, name string, participantType ParticipantType) (Participant, error) {
	name = strings.TrimSpace(name)

	if id <= 0 {
		return Participant{}, ErrInvalidParticipantID
	}
	if name == "" {
		return Participant{}, ErrEmptyParticipantName
	}
	if participantType != ParticipantTypeBroker && participantType != ParticipantTypeTrader {
		return Participant{}, ErrInvalidParticipantType
	}

	return Participant{
		ID:   id,
		Name: name,
		Type: participantType,
	}, nil
}
