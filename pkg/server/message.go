package server

import "encoding/json"

type MessageInterface interface {
	GetType() string
	GetData() json.RawMessage
}

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (m *Message) GetType() string {
	return m.Type
}

func (m *Message) GetData() json.RawMessage {
	return m.Data
}
