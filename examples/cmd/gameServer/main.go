package main

import (
	"encoding/json"

	"github.com/bruxaodev/go-mp-sdk/pkg/server"
)

type Point3D struct {
	X, Y, Z float64
}

type Player struct {
	*server.Client
	Health    int
	Position  Point3D
	Inventory []string
}

func ClientFactory(conn *server.Conn) *Player {
	return &Player{
		Client:    server.NewClient(conn),
		Health:    100,
		Position:  Point3D{0, 0, 0},
		Inventory: make([]string, 0),
	}
}

type TypeMessages string

const (
	MessageTypeMove   TypeMessages = "move"
	MessageTypeAttack TypeMessages = "attack"
	MessageTypeChat   TypeMessages = "chat"
	MessageTypePing   TypeMessages = "ping"
)

type Message struct {
	Type TypeMessages    `json:"type"`
	Data json.RawMessage `json:"data"`
}

func MessageFactory(msg *server.Message) *Message {
	return &Message{
		Type: TypeMessages(msg.Type),
		Data: msg.Data,
	}
}

func main() {
	s, err := server.New("localhost:8888", 60, ClientFactory, MessageFactory)
	if err != nil {
		panic(err)
	}
	s.OnConn = func(c *Player) {
		println("Client connected:", c.GetID())
	}
	s.OnDisc = func(c *Player, err error) {
		println("Client disconnected:", c.GetID(), "error:", err.Error())
	}
	s.OnMsg = func(c *Player, msg *Message) {
		switch msg.Type {
		case MessageTypeMove:

		case MessageTypeAttack:

		case MessageTypeChat:

		case MessageTypePing:
			str, err := c.GetConn().OpenStream()
			if err != nil {
				println("Error opening stream:", err.Error())
				return
			}
			d, _ := json.Marshal(msg)
			_, err = str.Write(d)
			if err != nil {
				println("Error writing to stream:", err.Error())
			}
			str.Close()

		default:
			println("Unknown message type:", msg.Type)
		}
	}
	s.TickFn = func(s *server.Server[*Player, *Message]) {
		// Game loop logic here
		s.Broadcast(&server.Message{Type: "tick", Data: nil})
	}
	s.Start()
	defer s.Stop()
	select {}
}
