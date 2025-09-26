package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type OnConnectFn func(c *Client)
type OnDisconnectFn func(c *Client, err error)
type OnMessageFn func(c *Client, msg *Message)
type TickFn func(s *Server)

type Server struct {
	ln    quic.Listener
	conns sync.Map

	OnConn OnConnectFn
	OnDisc OnDisconnectFn
	OnMsg  OnMessageFn
	TickFn TickFn

	tps    time.Duration
	ctx    context.Context
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func New(addr string, tickRate int) (*Server, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	tr := &quic.Transport{Conn: udpConn}
	tlsConf := GenerateTLSConfig()
	ln, err := tr.Listen(tlsConf, &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  5 * time.Minute,
	})
	if err != nil {
		return nil, err
	}

	t := time.Second / time.Duration(tickRate)

	return &Server{
		ln:  *ln,
		tps: t,
	}, nil
}

func (s *Server) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.ctx = ctx
	s.wg.Add(1)
	go s.acceptLoop()
	s.wg.Add(1)
	go s.tickLoop()
	fmt.Printf("Server started, listening on %s\n", s.ln.Addr().String())
}

func (s *Server) Stop() {
	s.cancel()
	s.ln.Close()
	s.wg.Wait()
}

func (s *Server) tickLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.tps)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if s.TickFn != nil {
				s.TickFn(s)
			}
		}
	}
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept(s.ctx)
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				fmt.Println("accept error:", err)
				continue
			}
		}
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn *quic.Conn) {
	defer s.wg.Done()
	c := &Client{
		ID:   conn.RemoteAddr().String(),
		Conn: conn,
		Meta: make(map[string]interface{}),
	}
	s.conns.Store(conn, c)

	if s.OnConn != nil {
		c, ok := s.conns.Load(conn)
		if ok {
			s.OnConn(c.(*Client))
		}
	}
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			fmt.Println("stream accept error:", err)
			if s.OnDisc != nil {
				c, ok := s.conns.Load(conn)
				if ok {
					s.OnDisc(c.(*Client), err)
				}
			}
			s.conns.Delete(conn)
			return
		}
		s.wg.Add(1)
		go s.handleStream(stream, c)
	}
}

func (s *Server) handleStream(stream *quic.Stream, c *Client) {
	defer s.wg.Done()
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		fmt.Println("read stream error:", err)
		return
	}
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		fmt.Println("unmarshal message error:", err)
		return
	}
	fmt.Printf("Received message: %+v\n", msg)
	if s.OnMsg != nil {
		s.OnMsg(c, &msg)
	}
	// s.Broadcast(&msg)
}

func (s *Server) Broadcast(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("marshal message error:", err)
		return
	}
	s.conns.Range(func(key, value interface{}) bool {
		// fmt.Printf("Broadcasting to %v\n", key)
		conn := key.(*quic.Conn)
		str, err := conn.OpenStream()
		if err != nil {
			fmt.Println("open stream error:", err)
			return true
		}
		_, err = str.Write(data)
		if err != nil {
			fmt.Println("write stream error:", err)
		}
		str.Close()
		return true
	})
}

func (s *Server) SendDatagram(conn *quic.Conn, data []byte) error {
	return conn.SendDatagram(data)
}

func GenerateTLSConfig() *tls.Config {
	cert, key, err := GenerateSelfSigned()
	if err != nil {
		panic(err)
	}
	c, err := tls.X509KeyPair(cert, key)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{c},
	}
}
