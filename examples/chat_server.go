package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bruxaodev/go-mp-sdk/pkg/server"
	"github.com/quic-go/quic-go"
)

// ChatClient - Client para um sistema de chat com salas
type ChatClient struct {
	*server.Client

	Nickname     string
	Room         string
	JoinedAt     time.Time
	IsAdmin      bool
	IsMuted      bool
	MessagesSent int
}

func NewChatClient(conn *quic.Conn) *ChatClient {
	return &ChatClient{
		Client:       server.NewClient(conn),
		Nickname:     "Guest",
		Room:         "lobby",
		JoinedAt:     time.Now(),
		IsAdmin:      false,
		IsMuted:      false,
		MessagesSent: 0,
	}
}

func (c *ChatClient) CanSendMessage() bool {
	return !c.IsMuted
}

func (c *ChatClient) IncrementMessages() {
	c.MessagesSent++
}

func RunChatServer() {
	// Servidor de chat usando ChatClient
	chatServer, err := server.New("localhost:8888", 60, NewChatClient)
	if err != nil {
		log.Fatal(err)
	}

	chatServer.OnConn = func(c *ChatClient) {
		c.SetID(fmt.Sprintf("user_%d", time.Now().Unix()))
		log.Printf("📥 [%s] Cliente conectado na sala '%s'", c.GetID(), c.Room)

		// Enviar mensagem de boas-vindas
		welcomeMsg := server.Message{
			Type: "system",
			Data: json.RawMessage(fmt.Sprintf(`{
				"message": "Bem-vindo ao chat, %s! Você está na sala '%s'",
				"nickname": "%s",
				"room": "%s"
			}`, c.Nickname, c.Room, c.Nickname, c.Room)),
		}

		sendMessageToClient(c, &welcomeMsg)
	}

	chatServer.OnDisc = func(c *ChatClient, err error) {
		log.Printf("📤 [%s] Cliente desconectado da sala '%s' - %v mensagens enviadas",
			c.Nickname, c.Room, c.MessagesSent)

		// Notificar outros usuários na mesma sala
		leaveMsg := server.Message{
			Type: "user_left",
			Data: json.RawMessage(fmt.Sprintf(`{
				"nickname": "%s",
				"room": "%s"
			}`, c.Nickname, c.Room)),
		}

		broadcastToRoom(chatServer, c.Room, &leaveMsg, c)
	}

	chatServer.OnMsg = func(c *ChatClient, msg *server.Message) {
		log.Printf("💬 [%s] Mensagem recebida: %s", c.Nickname, msg.Type)

		switch msg.Type {
		case "join_room":
			handleJoinRoom(chatServer, c, msg)

		case "chat_message":
			handleChatMessage(chatServer, c, msg)

		case "private_message":
			handlePrivateMessage(chatServer, c, msg)

		case "admin_command":
			handleAdminCommand(chatServer, c, msg)

		case "set_nickname":
			handleSetNickname(c, msg)
		}
	}

	chatServer.TickFn = func(s *server.Server[*ChatClient]) {
		// A cada 30 segundos, enviar estatísticas
		clients := s.GetClients()
		if len(clients) > 0 && time.Now().Second()%30 == 0 {
			stats := map[string]int{}
			for _, client := range clients {
				stats[client.Room]++
			}

			log.Printf("📊 Estatísticas: %v clients online, salas: %+v", len(clients), stats)
		}
	}

	log.Println("🚀 Servidor de chat iniciado em localhost:8888")
	chatServer.Start()
	defer chatServer.Stop()

	select {} // Manter o servidor rodando
}

func handleJoinRoom(s *server.Server[*ChatClient], c *ChatClient, msg *server.Message) {
	var data struct {
		Room string `json:"room"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		log.Printf("❌ Erro ao decodificar join_room: %v", err)
		return
	}

	oldRoom := c.Room
	c.Room = data.Room

	log.Printf("🚪 [%s] Mudou da sala '%s' para '%s'", c.Nickname, oldRoom, c.Room)

	// Notificar saída da sala anterior
	leaveMsg := server.Message{
		Type: "user_left",
		Data: json.RawMessage(fmt.Sprintf(`{
			"nickname": "%s",
			"room": "%s"
		}`, c.Nickname, oldRoom)),
	}
	broadcastToRoom(s, oldRoom, &leaveMsg, c)

	// Notificar entrada na nova sala
	joinMsg := server.Message{
		Type: "user_joined",
		Data: json.RawMessage(fmt.Sprintf(`{
			"nickname": "%s",
			"room": "%s"
		}`, c.Nickname, c.Room)),
	}
	broadcastToRoom(s, c.Room, &joinMsg, c)

	// Confirmar mudança para o cliente
	confirmMsg := server.Message{
		Type: "room_changed",
		Data: json.RawMessage(fmt.Sprintf(`{
			"old_room": "%s",
			"new_room": "%s"
		}`, oldRoom, c.Room)),
	}
	sendMessageToClient(c, &confirmMsg)
}

func handleChatMessage(s *server.Server[*ChatClient], c *ChatClient, msg *server.Message) {
	if !c.CanSendMessage() {
		errorMsg := server.Message{
			Type: "error",
			Data: json.RawMessage(`{"message": "Você está mutado e não pode enviar mensagens"}`),
		}
		sendMessageToClient(c, &errorMsg)
		return
	}

	var data struct {
		Message string `json:"message"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		log.Printf("❌ Erro ao decodificar chat_message: %v", err)
		return
	}

	c.IncrementMessages()

	// Reenviar mensagem para todos na mesma sala
	chatMsg := server.Message{
		Type: "chat_message",
		Data: json.RawMessage(fmt.Sprintf(`{
			"nickname": "%s",
			"message": "%s",
			"room": "%s",
			"timestamp": "%s"
		}`, c.Nickname, data.Message, c.Room, time.Now().Format(time.RFC3339))),
	}

	broadcastToRoom(s, c.Room, &chatMsg, nil) // nil = incluir o remetente
}

func handlePrivateMessage(s *server.Server[*ChatClient], c *ChatClient, msg *server.Message) {
	var data struct {
		ToNickname string `json:"to_nickname"`
		Message    string `json:"message"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		log.Printf("❌ Erro ao decodificar private_message: %v", err)
		return
	}

	// Encontrar o destinatário
	clients := s.GetClients()
	var target *ChatClient
	for _, client := range clients {
		if client.Nickname == data.ToNickname {
			target = client
			break
		}
	}

	if target == nil {
		errorMsg := server.Message{
			Type: "error",
			Data: json.RawMessage(fmt.Sprintf(`{"message": "Usuário '%s' não encontrado"}`, data.ToNickname)),
		}
		sendMessageToClient(c, &errorMsg)
		return
	}

	// Enviar mensagem privada
	privateMsg := server.Message{
		Type: "private_message",
		Data: json.RawMessage(fmt.Sprintf(`{
			"from_nickname": "%s",
			"message": "%s",
			"timestamp": "%s"
		}`, c.Nickname, data.Message, time.Now().Format(time.RFC3339))),
	}

	sendMessageToClient(target, &privateMsg)

	// Confirmar envio para o remetente
	confirmMsg := server.Message{
		Type: "private_sent",
		Data: json.RawMessage(fmt.Sprintf(`{
			"to_nickname": "%s",
			"message": "%s"
		}`, data.ToNickname, data.Message)),
	}
	sendMessageToClient(c, &confirmMsg)
}

func handleAdminCommand(s *server.Server[*ChatClient], c *ChatClient, msg *server.Message) {
	if !c.IsAdmin {
		errorMsg := server.Message{
			Type: "error",
			Data: json.RawMessage(`{"message": "Você não tem permissões de administrador"}`),
		}
		sendMessageToClient(c, &errorMsg)
		return
	}

	var data struct {
		Command string `json:"command"`
		Target  string `json:"target,omitempty"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		log.Printf("❌ Erro ao decodificar admin_command: %v", err)
		return
	}

	switch data.Command {
	case "mute":
		muteUser(s, data.Target)
	case "unmute":
		unmuteUser(s, data.Target)
	case "kick":
		kickUser(s, data.Target)
	case "list_users":
		listUsers(s, c)
	}
}

func handleSetNickname(c *ChatClient, msg *server.Message) {
	var data struct {
		Nickname string `json:"nickname"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		log.Printf("❌ Erro ao decodificar set_nickname: %v", err)
		return
	}

	oldNickname := c.Nickname
	c.Nickname = data.Nickname

	confirmMsg := server.Message{
		Type: "nickname_changed",
		Data: json.RawMessage(fmt.Sprintf(`{
			"old_nickname": "%s",
			"new_nickname": "%s"
		}`, oldNickname, c.Nickname)),
	}
	sendMessageToClient(c, &confirmMsg)

	log.Printf("👤 Cliente mudou nickname de '%s' para '%s'", oldNickname, c.Nickname)
}

func sendMessageToClient(c *ChatClient, msg *server.Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("❌ Erro ao serializar mensagem: %v", err)
		return
	}

	stream, err := c.GetConn().OpenStream()
	if err != nil {
		log.Printf("❌ Erro ao abrir stream: %v", err)
		return
	}
	defer stream.Close()

	_, err = stream.Write(data)
	if err != nil {
		log.Printf("❌ Erro ao enviar mensagem: %v", err)
	}
}

func broadcastToRoom(s *server.Server[*ChatClient], room string, msg *server.Message, except *ChatClient) {
	clients := s.GetClients()
	for _, client := range clients {
		if client.Room == room && client != except {
			sendMessageToClient(client, msg)
		}
	}
}

func muteUser(s *server.Server[*ChatClient], nickname string) {
	clients := s.GetClients()
	for _, client := range clients {
		if client.Nickname == nickname {
			client.IsMuted = true
			log.Printf("🔇 Usuário '%s' foi mutado", nickname)
			return
		}
	}
}

func unmuteUser(s *server.Server[*ChatClient], nickname string) {
	clients := s.GetClients()
	for _, client := range clients {
		if client.Nickname == nickname {
			client.IsMuted = false
			log.Printf("🔊 Usuário '%s' foi desmutado", nickname)
			return
		}
	}
}

func kickUser(s *server.Server[*ChatClient], nickname string) {
	clients := s.GetClients()
	for _, client := range clients {
		if client.Nickname == nickname {
			client.GetConn().CloseWithError(1000, "Kicked by admin")
			log.Printf("👢 Usuário '%s' foi expulso", nickname)
			return
		}
	}
}

func listUsers(s *server.Server[*ChatClient], adminClient *ChatClient) {
	clients := s.GetClients()
	userList := make([]map[string]interface{}, 0, len(clients))

	for _, client := range clients {
		userList = append(userList, map[string]interface{}{
			"nickname":      client.Nickname,
			"room":          client.Room,
			"joined_at":     client.JoinedAt.Format(time.RFC3339),
			"messages_sent": client.MessagesSent,
			"is_muted":      client.IsMuted,
		})
	}

	data, _ := json.Marshal(userList)
	listMsg := server.Message{
		Type: "user_list",
		Data: json.RawMessage(data),
	}

	sendMessageToClient(adminClient, &listMsg)
}

// Para testar o chat server, descomente a linha abaixo:
// func main() { RunChatServer() }
