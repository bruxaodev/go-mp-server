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

// ClientConstraint define que tipos podem ser usados como client
// T deve ser um pointer para um tipo que implementa ClientInterface
type ClientConstraint[T any] interface {
	*T
	ClientInterface
}

// Factory function type para criar novos clients
type ClientFactory[T any] func(conn *quic.Conn) T

// Callback functions agora são genéricas
type OnConnectFn[T any] func(c T)
type OnDisconnectFn[T any] func(c T, err error)
type OnMessageFn[T any] func(c T, msg *Message)
type TickFn[T any] func(s *Server[T])

type Server[T any] struct {
	ln    quic.Listener
	conns sync.Map // key: *quic.Conn, value: T

	ClientFactory ClientFactory[T]
	OnConn        OnConnectFn[T]
	OnDisc        OnDisconnectFn[T]
	OnMsg         OnMessageFn[T]
	TickFn        TickFn[T]

	tps    time.Duration
	ctx    context.Context
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func New[T any](addr string, tickRate int, factory ClientFactory[T]) (*Server[T], error) {
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

	return &Server[T]{
		ln:            *ln,
		tps:           t,
		ClientFactory: factory,
	}, nil
}

func (s *Server[T]) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.ctx = ctx
	s.wg.Add(1)
	go s.acceptLoop()
	s.wg.Add(1)
	go s.tickLoop()
	fmt.Printf("Server started, listening on %s\n", s.ln.Addr().String())
}

func (s *Server[T]) Stop() {
	s.cancel()
	s.ln.Close()
	s.wg.Wait()
}

func (s *Server[T]) tickLoop() {
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

func (s *Server[T]) acceptLoop() {
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

func (s *Server[T]) handleConnection(conn *quic.Conn) {
	defer s.wg.Done()
	c := s.ClientFactory(conn)
	s.conns.Store(conn, c)

	if s.OnConn != nil {
		s.OnConn(c)
	}

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			fmt.Println("stream accept error:", err)
			if s.OnDisc != nil {
				s.OnDisc(c, err)
			}
			s.conns.Delete(conn)
			return
		}
		s.wg.Add(1)
		go s.handleStream(stream, c)
	}
}

func (s *Server[T]) handleStream(stream *quic.Stream, c T) {
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

func (s *Server[T]) Broadcast(msg *Message) {
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

func (s *Server[T]) SendDatagram(conn *quic.Conn, data []byte) error {
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

// NewDefaultServer cria um servidor usando o client padrão
func NewDefaultServer(addr string, tickRate int) (*Server[*Client], error) {
	return New(addr, tickRate, NewClient)
}

// Funções auxiliares para trabalhar com clients
func (s *Server[T]) GetClients() []T {
	var clients []T
	s.conns.Range(func(key, value interface{}) bool {
		if client, ok := value.(T); ok {
			clients = append(clients, client)
		}
		return true
	})
	return clients
}

func (s *Server[T]) GetClientByConn(conn *quic.Conn) (T, bool) {
	var zero T
	if value, ok := s.conns.Load(conn); ok {
		if client, ok := value.(T); ok {
			return client, true
		}
	}
	return zero, false
}
