package main

import (
	"encoding/json"

	"github.com/bruxaodev/go-mp-sdk/pkg/server"
)

func main() {
	s, err := server.New("localhost:8888", 60)
	if err != nil {
		panic(err)
	}

	s.OnConn = func(c *server.Client) {
		println("Client connected:", c.ID)
	}

	s.OnDisc = func(c *server.Client, err error) {
		println("Client disconnected:", c.ID, "error:", err)
	}

	s.OnMsg = func(c *server.Client, msg *server.Message) {
		println("Received message from", c.ID, "type:", msg.Type)
		str, err := c.Conn.OpenStream()
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
