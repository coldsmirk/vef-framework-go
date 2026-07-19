package push

import (
	"time"

	"github.com/coldsmirk/vef-framework-go/id"
)

// Message is the wire envelope every push delivers, serialized as one JSON
// text frame. Type is the business-defined discriminator clients dispatch on;
// Payload is an arbitrary JSON-serializable value.
type Message struct {
	ID      string    `json:"id"`
	Type    string    `json:"type"`
	Payload any       `json:"payload,omitempty"`
	Time    time.Time `json:"time"`
}

// NewMessage builds a message with a generated ID and the current time.
func NewMessage(messageType string, payload any) Message {
	return Message{
		ID:      id.Generate(),
		Type:    messageType,
		Payload: payload,
		Time:    time.Now(),
	}
}
