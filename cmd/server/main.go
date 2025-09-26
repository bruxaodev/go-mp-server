package main

import (
	"encoding/json"

	"github.com/bruxaodev/go-mp-sdk/pkg/server"
	"github.com/quic-go/quic-go"
)

type Client struct {
	*server.Client

	Nickname string
	Room     string
}

func NewClient(conn *quic.Conn) *Client {
	return &Client{
		Client:   server.NewClient(conn),
		Nickname: "Guest",
		Room:     "lobby",
	}
}

func main() {
	s, err := server.New("localhost:8888", 60, NewClient)
	if err != nil {
		panic(err)
	}

	s.OnConn = func(c *Client) {
		println("Client connected:", c.GetID())
	}

	s.OnDisc = func(c *Client, err error) {
		println("Client disconnected:", c.GetID(), "error:", err.Error())
	}

	s.OnMsg = func(c *Client, msg *server.Message) {
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
