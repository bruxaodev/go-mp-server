package main

import (
	"encoding/json"

	"github.com/bruxaodev/go-mp-sdk/pkg/server"
	"github.com/quic-go/quic-go"
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

func ClientFactory(conn *quic.Conn) *Player {
	return &Player{
		Client:    server.NewClient(conn),
		Health:    100,
		Position:  Point3D{0, 0, 0},
		Inventory: make([]string, 0),
	}
}

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func MessageFactory(msg *server.Message) *Message {
	return &Message{
		Type: msg.Type,
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
		println("Received message from", c.GetID(), "type:", msg.Type)
	}
	s.Start()
	defer s.Stop()
	select {}
}
