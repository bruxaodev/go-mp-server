package server

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/vmihailenco/msgpack/v5"
)

type Message struct {
	Type string      `msgpack:"t"`
	Data interface{} `msgpack:"d"`
}

type OnConnectFn func(p *Client)
type OnDisconnectFn func(p *Client, err error)
type OnMessageFn func(p *Client, msg *Message, stream *quic.Stream)

type Server struct {
	Addr         string
	tslConfig    *tls.Config
	listner      *quic.Listener
	clients      map[string]*Client
	clientsMutex sync.RWMutex

	OnConnectFn    OnConnectFn
	OnDisconnectFn OnDisconnectFn
	OnMessageFn    OnMessageFn

	tickInterfal time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewServer(addr string, tlsConf *tls.Config, ticksPerSec int) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	interval := time.Second / time.Duration(ticksPerSec)
	return &Server{
		Addr:         addr,
		tslConfig:    tlsConf,
		clients:      make(map[string]*Client),
		tickInterfal: interval,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (s *Server) Start() error {
	ln, err := quic.ListenAddr(s.Addr, s.tslConfig, nil)
	if err != nil {
		return err
	}

	s.listner = ln
	s.wg.Add(1)
	go s.acceptLoop()
	s.wg.Add(1)
	go s.tickLoop()

	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listner.Accept(s.ctx)
		if err != nil {

			select {
			case <-s.ctx.Done():
				return
			default:
			}
			continue

		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn *quic.Conn) {
	defer s.wg.Done()
	peer := conn.RemoteAddr().String()
	client := &Client{
		ID:      peer,
		Conn:    conn,
		Streams: make(map[uint64]quic.Stream),
		Meta:    make(map[string]interface{}),
	}

	s.clientsMutex.Lock()
	s.clients[client.ID] = client
	s.clientsMutex.Unlock()

	if s.OnConnectFn != nil {
		s.OnConnectFn(client)
	}

	streamCtx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	for {
		stream, err := conn.AcceptStream(streamCtx)
		if err != nil {
			s.clientsMutex.Lock()
			delete(s.clients, client.ID)
			s.clientsMutex.Unlock()
			if s.OnDisconnectFn != nil {
				s.OnDisconnectFn(client, err)
			}
			return
		}

		s.wg.Add(1)
		go func(st *quic.Stream) {
			defer s.wg.Done()
			// Ler mensagem de forma mais eficiente
			data, err := io.ReadAll(st)
			if err != nil {
				st.Close()
				return
			}
			
			var msg Message
			if err := msgpack.Unmarshal(data, &msg); err != nil {
				st.Close()
				return
			}
			if s.OnMessageFn != nil {
				s.OnMessageFn(client, &msg, st)
			}
			// Fechar stream após callback
			st.Close()
		}(stream)
	}
}

func (s *Server) Broadcast(v interface{}) {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()
	for _, p := range s.clients {
		pLocal := p
		go func() {
			_ = pLocal.SendPacked(context.Background(), v)
		}()
	}
}

func (s *Server) SendTo(clientID string, v interface{}) error {
	s.clientsMutex.RLock()
	p, ok := s.clients[clientID]
	s.clientsMutex.RUnlock()
	if !ok {
		return errors.New("client not found")
	}
	return p.SendPacked(context.Background(), v)
}

func (s *Server) tickLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.tickInterfal)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:

		}
	}
}

func (s *Server) Stop() error {
	s.cancel()
	s.listner.Close()

	s.clientsMutex.RLock()
	for _, p := range s.clients {
		p.Conn.CloseWithError(0, "server stopping")
	}
	s.clientsMutex.RUnlock()
	s.wg.Wait()
	return nil
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
