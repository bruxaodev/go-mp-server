package main

import (
	"encoding/json"

	"github.com/bruxaodev/go-mp-sdk/pkg/server"
)

type Client struct {
	*server.Client

	Nickname string
	Room     string
}

func NewClient(conn *server.Conn) *Client {
	return &Client{
		Client:   server.NewClient(conn),
		Nickname: "Guest",
		Room:     "lobby",
	}
}

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func NewMessage(msg *server.Message) *Message {
	return &Message{
		Type: msg.Type,
		Data: msg.Data,
	}
}

func main() {
	s, err := server.New("localhost:8888", 60, NewClient, NewMessage)
	if err != nil {
		panic(err)
	}

	s.OnConn = func(c *Client) {
		println("Client connected:", c.GetID())
	}

	s.OnDisc = func(c *Client, err error) {
		println("Client disconnected:", c.GetID(), "error:", err.Error())
	}

	s.OnMsg = func(c *Client, msg *Message) {
		println("Received message from", c.GetID(), "type:", msg.Type)
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
	}
	s.Start()
	defer s.Stop()
	select {}
}
