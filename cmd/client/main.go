package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/vmihailenco/msgpack/v5"
)

type Message struct {
	Type string      `msgpack:"t"`
	Data interface{} `msgpack:"d"`
}

func main() {
	addr := "localhost:8888"  // Porta 8888

	fmt.Println("Conectando ao servidor...")
	session, err := quic.DialAddr(context.Background(), addr, &tls.Config{InsecureSkipVerify: true}, nil)
	if err != nil {
		log.Fatal("Erro ao conectar:", err)
	}
	defer session.CloseWithError(0, "bye")

	fmt.Println("Conexão estabelecida!")

	// Abrir stream bidirecional
	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatal("Erro ao abrir stream:", err)
	}
	// Remover defer - vamos fechar manualmente no final

	// Envia mensagem packeada
	msg := Message{Type: "ping", Data: map[string]interface{}{"timestamp": time.Now().Unix()}}
	b, err := msgpack.Marshal(msg)
	if err != nil {
		log.Fatal("Erro ao serializar mensagem:", err)
	}

	fmt.Printf("Enviando mensagem (%d bytes): %+v\n", len(b), msg)

	// Escrever dados
	n, err := stream.Write(b)
	if err != nil {
		log.Fatal("Erro ao escrever no stream:", err)
	}
	fmt.Printf("Bytes escritos: %d\n", n)

	// NÃO fechar o stream - usar o mesmo para receber resposta
	fmt.Println("Aguardando resposta no mesmo stream...")

	// Ler resposta com timeout MAIOR
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)  // 30 segundos
	defer cancel()

	// Canal para receber resultado da leitura
	type readResult struct {
		data []byte
		err  error
	}

	readCh := make(chan readResult, 1)

	go func() {
		// Ler resposta com buffer fixo ao invés de ReadAll
		buf := make([]byte, 1024)
		n, err := stream.Read(buf)
		if err != nil {
			readCh <- readResult{nil, err}
			return
		}
		readCh <- readResult{buf[:n], nil}
	}()

	select {
	case result := <-readCh:
		if result.err != nil {
			if result.err == io.EOF {
				fmt.Println("Servidor fechou a conexão sem resposta")
			} else {
				log.Printf("Erro ao ler resposta: %v", result.err)
			}
		} else if len(result.data) > 0 {
			fmt.Printf("Recebidos %d bytes de resposta\n", len(result.data))
			var r Message
			err = msgpack.Unmarshal(result.data, &r)
			if err != nil {
				log.Printf("Erro ao deserializar resposta: %v", err)
				fmt.Printf("Dados brutos recebidos: %x\n", result.data)
				fmt.Printf("Dados como string: %s\n", string(result.data))
			} else {
				fmt.Printf("Resposta recebida: %+v\n", r)
			}
		} else {
			fmt.Println("Resposta vazia recebida")
		}
	case <-ctx.Done():
		fmt.Println("Timeout aguardando resposta do servidor")
	}

	fmt.Println("Cliente finalizando...")
	time.Sleep(500 * time.Millisecond)
}
