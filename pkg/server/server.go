package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

type MessageConstraint[T any] interface {
	*T
	MessageInterface
}

type MessageFactory[T any] func(msg *Message) T

type ClientConstraint[T any] interface {
	*T
	ClientInterface
}

type Conn struct {
	*quic.Conn
}

func (c *Conn) OpenStream() (*Stream, error) {
	stream, err := c.Conn.OpenStream()
	if err != nil {
		return nil, err
	}
	return &Stream{Stream: stream}, nil
}

func (c *Conn) SendDatagram(data []byte) error {
	return c.Conn.SendDatagram(data)
}

func (c *Conn) AcceptStream(ctx context.Context) (*Stream, error) {
	stream, err := c.Conn.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return &Stream{Stream: stream}, nil
}

type Stream struct {
	*quic.Stream
}
type ClientFactory[T any] func(conn *Conn) T

type OnConnectFn[T any] func(c T)
type OnDisconnectFn[T any] func(c T, err error)
type OnMessageFn[T, M any] func(c T, msg M)
type TickFn[T, M any] func(s *Server[T, M])

type Server[T, M any] struct {
	ln    quic.Listener
	conns sync.Map // key: *quic.Conn, value: T

	ClientFactory  ClientFactory[T]
	MessageFactory MessageFactory[M]
	OnConn         OnConnectFn[T]
	OnDisc         OnDisconnectFn[T]
	OnMsg          OnMessageFn[T, M]
	TickFn         TickFn[T, M]

	tps    time.Duration
	ctx    context.Context
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func New[T, M any](addr string, tickRate int, clientFactory ClientFactory[T], messageFactory MessageFactory[M]) (*Server[T, M], error) {
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

	return &Server[T, M]{
		ln:             *ln,
		tps:            t,
		ClientFactory:  clientFactory,
		MessageFactory: messageFactory,
	}, nil
}

func (s *Server[T, M]) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.ctx = ctx
	s.wg.Add(1)
	go s.acceptLoop()
	s.wg.Add(1)
	go s.tickLoop()
	log.Printf("Server started, listening on %s\n", s.ln.Addr().String())
}

func (s *Server[T, M]) Stop() {
	s.cancel()
	s.ln.Close()
	s.wg.Wait()
}

func (s *Server[T, M]) tickLoop() {
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

func (s *Server[T, M]) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept(s.ctx)
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Println("accept error:", err)
				continue
			}
		}
		s.wg.Add(1)
		go s.handleConnection(&Conn{conn})
	}
}

func (s *Server[T, M]) handleConnection(conn *Conn) {
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
			log.Println("stream accept error:", err)
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

func (s *Server[T, M]) handleStream(stream *Stream, c T) {
	defer s.wg.Done()
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		log.Println("read stream error:", err)
		return
	}
	var baseMsg Message
	if err := json.Unmarshal(data, &baseMsg); err != nil {
		log.Println("unmarshal message error:", err)
		return
	}
	msg := s.MessageFactory(&baseMsg)
	if s.OnMsg != nil {
		s.OnMsg(c, msg)
	}
}

func (s *Server[T, M]) Broadcast(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("marshal message error:", err)
		return
	}
	s.conns.Range(func(key, value interface{}) bool {
		// fmt.Printf("Broadcasting to %v\n", key)
		conn := key.(*Conn)
		str, err := conn.OpenStream()
		if err != nil {
			log.Println("open stream error:", err)
			return true
		}
		_, err = str.Write(data)
		if err != nil {
			log.Println("write stream error:", err)
		}
		str.Close()
		return true
	})
}

func (s *Server[T, M]) SendDatagram(conn *Conn, data []byte) error {
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

// NewDefaultServer cria um servidor usando o client padrão e message padrão
func NewDefaultServer(addr string, tickRate int) (*Server[*Client, *Message], error) {
	return New(addr, tickRate, NewClient, NewMessage)
}

// NewMessage cria uma nova instância de Message
func NewMessage(msg *Message) *Message {
	return msg
}

// Funções auxiliares para trabalhar com clients
func (s *Server[T, M]) GetClients() []T {
	var clients []T
	s.conns.Range(func(key, value interface{}) bool {
		if client, ok := value.(T); ok {
			clients = append(clients, client)
		}
		return true
	})
	return clients
}

func (s *Server[T, M]) GetClientByConn(conn *Conn) (T, bool) {
	var zero T
	if value, ok := s.conns.Load(conn); ok {
		if client, ok := value.(T); ok {
			return client, true
		}
	}
	return zero, false
}
