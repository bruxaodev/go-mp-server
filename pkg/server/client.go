package server

import (
	"github.com/quic-go/quic-go"
)

type Client struct {
	ID   string
	Conn *quic.Conn
	Meta map[string]interface{}
}
