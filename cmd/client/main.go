package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/bruxaodev/go-mp-sdk/pkg/server"
	"github.com/quic-go/quic-go"
)

type Client struct {
	conn   *quic.Conn
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewClient(addr string) (*Client, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
	}
	conn, err := quic.DialAddr(context.Background(), addr, tlsConf, &quic.Config{EnableDatagrams: true})
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

func (c *Client) SendMessage(msg server.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	str, err := c.conn.OpenStream()
	if err != nil {
		return err
	}
	_, err = str.Write(data)
	if err != nil {
		return err
	}
	return str.Close()
}

func (c *Client) Receive() {
	for {
		str, err := c.conn.AcceptStream(context.Background())
		if err != nil {
			fmt.Println("Accept stream error:", err)
			return
		}
		data, err := io.ReadAll(str)
		if err != nil {
			fmt.Println("Read error:", err)
			continue
		}
		var msg server.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			fmt.Println("Unmarshal error:", err)
			continue
		}
		fmt.Printf("Received %s: %s\n", msg.Type, msg.Data)
	}
}

func (c *Client) SendDatagram(data []byte) error {
	return c.conn.SendDatagram(data)
}

func (c *Client) Close() {
	c.conn.CloseWithError(0, "")
}

func (c *Client) keepAlive(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := c.SendMessage(server.Message{Type: "ping", Data: json.RawMessage(`{"time": "` + time.Now().String() + `"}`)})
			if err != nil {
				fmt.Println("Send datagram error:", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	c, err := NewClient("localhost:8888")
	if err != nil {
		os.Exit(1)
	}
	defer c.Close()
	go c.keepAlive(context.Background())
	err = c.SendMessage(server.Message{Type: "join", Data: json.RawMessage(`{"player": "test"}`)})
	if err != nil {
		fmt.Println("Send error:", err)
	}

	c.Receive()
}
