package main

import (
	"fmt"

	"github.com/bruxaodev/go-mp-sdk/pkg/server"
	"github.com/quic-go/quic-go"
	"github.com/vmihailenco/msgpack/v5"
)

func main() {
	serverAddress := "localhost:8888" // Porta 8888
	tlsCof := server.GenerateTLSConfig()
	srv := server.NewServer(serverAddress, tlsCof, 60) // Porta 8888
	defer srv.Stop()

	srv.OnConnectFn = func(p *server.Client) {
		fmt.Println("Client connected:", p.ID)
		srv.SendTo(p.ID, map[string]interface{}{"msg": "Welcome!"})
	}

	srv.OnDisconnectFn = func(p *server.Client, err error) {
		fmt.Println("Client disconnected:", p.ID, "error:", err)
	}

	srv.OnMessageFn = func(p *server.Client, msg *server.Message, stream *quic.Stream) {
		fmt.Printf("Received message from %s, type: %s, data: %+v\n", p.ID, msg.Type, msg.Data)
		if msg.Type == "ping" {
			// Responder no mesmo stream
			response := map[string]interface{}{
				"t": "pong",
				"d": map[string]interface{}{"from": p.ID},
			}
			fmt.Printf("Enviando resposta: %+v\n", response)

			// Serializar resposta
			b, err := msgpack.Marshal(response)
			if err != nil {
				fmt.Printf("Erro ao serializar resposta: %v\n", err)
				return
			}

			// Enviar no mesmo stream
			_, err = stream.Write(b)
			if err != nil {
				fmt.Printf("Erro ao escrever resposta no stream: %v\n", err)
			} else {
				fmt.Printf("Resposta de %d bytes enviada no mesmo stream\n", len(b))
			}
		}
	}

	if err := srv.Start(); err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	fmt.Println("Server started on ", serverAddress)

	// Block forever
	select {}

}
