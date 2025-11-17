package websocketutils

import "encoding/json"

type WireMessage struct {
	Type      string          `json:"type"`
	Namespace string          `json:"namespace,omitempty"`
	Event     string          `json:"event,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	AckID     string          `json:"ackId,omitempty"`
	Room      string          `json:"room,omitempty"`
	Target    string          `json:"target,omitempty"`
	Error     string          `json:"error,omitempty"`
}

func encodeMessage(t, namespace, event string, payload any, ackID string) ([]byte, error) {
	var raw json.RawMessage
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	msg := WireMessage{
		Type:      t,
		Namespace: namespace,
		Event:     event,
		Data:      raw,
		AckID:     ackID,
	}
	return json.Marshal(msg)
}
