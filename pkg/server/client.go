package server

import (
	"context"
	"sync"

	"github.com/quic-go/quic-go"
	"github.com/vmihailenco/msgpack/v5"
)

type Client struct {
	ID        string
	Conn      *quic.Conn
	Streams   map[uint64]quic.Stream
	streamMtx sync.Mutex
	Meta      map[string]interface{}
}

func (p *Client) SendPacked(ctx context.Context, v interface{}) error {
	p.streamMtx.Lock()
	defer p.streamMtx.Unlock()

	stream, err := p.Conn.OpenStreamSync(ctx) // Mudou para bidirecional
	if err != nil {
		return err
	}
	defer stream.Close()
	b, err := msgpack.Marshal(v)
	if err != nil {
		return err
	}
	_, err = stream.Write(b)
	return err
}
