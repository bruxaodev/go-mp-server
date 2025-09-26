package main

import (
	"encoding/json"
	"time"

	"github.com/bruxaodev/go-mp-sdk/pkg/server"
	"github.com/quic-go/quic-go"
)

// CustomClient é um exemplo de client customizado que estende o client padrão
type CustomClient struct {
	*server.Client // Embedding do client padrão

	// Campos customizados
	Username    string
	Level       int
	LastSeen    time.Time
	Permissions []string
}

// NewCustomClient é a factory function para criar o client customizado
func NewCustomClient(conn *quic.Conn) *CustomClient {
	return &CustomClient{
		Client:      server.NewClient(conn),
		Username:    "anonymous",
		Level:       1,
		LastSeen:    time.Now(),
		Permissions: []string{"read"},
	}
}

// Métodos específicos do CustomClient
func (c *CustomClient) HasPermission(perm string) bool {
	for _, p := range c.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

func (c *CustomClient) UpdateLastSeen() {
	c.LastSeen = time.Now()
}

func (c *CustomClient) GrantPermission(perm string) {
	if !c.HasPermission(perm) {
		c.Permissions = append(c.Permissions, perm)
	}
}

// Exemplo de client completamente customizado (sem embedding)
type GameClient struct {
	id   string
	conn *quic.Conn
	meta map[string]interface{}

	// Campos específicos do jogo
	Position  Point3D
	Health    int
	Team      string
	Inventory []Item
}

type Point3D struct {
	X, Y, Z float64
}

type Item struct {
	ID       string
	Name     string
	Quantity int
}

// Implementação da interface ClientInterface
func (g *GameClient) GetID() string                   { return g.id }
func (g *GameClient) GetConn() *quic.Conn             { return g.conn }
func (g *GameClient) GetMeta() map[string]interface{} { return g.meta }
func (g *GameClient) SetID(id string)                 { g.id = id }
func (g *GameClient) SetMeta(key string, value interface{}) {
	if g.meta == nil {
		g.meta = make(map[string]interface{})
	}
	g.meta[key] = value
}

// Factory para GameClient
func NewGameClient(conn *quic.Conn) *GameClient {
	return &GameClient{
		conn:      conn,
		meta:      make(map[string]interface{}),
		Position:  Point3D{0, 0, 0},
		Health:    100,
		Team:      "neutral",
		Inventory: make([]Item, 0),
	}
}

// Métodos específicos do jogo
func (g *GameClient) MoveTo(x, y, z float64) {
	g.Position = Point3D{x, y, z}
}

func (g *GameClient) TakeDamage(damage int) {
	g.Health -= damage
	if g.Health < 0 {
		g.Health = 0
	}
}

func (g *GameClient) AddItem(item Item) {
	for i, inv := range g.Inventory {
		if inv.ID == item.ID {
			g.Inventory[i].Quantity += item.Quantity
			return
		}
	}
	g.Inventory = append(g.Inventory, item)
}

func main() {
	// Exemplo 1: Servidor com client padrão
	println("=== Exemplo 1: Client Padrão ===")
	exampleDefaultClient()

	// Exemplo 2: Servidor com client customizado (embedding)
	println("\n=== Exemplo 2: Client Customizado (Embedding) ===")
	exampleCustomClient()

	// Exemplo 3: Servidor com client de jogo (completamente customizado)
	println("\n=== Exemplo 3: Client de Jogo (Completamente Customizado) ===")
	exampleGameClient()
}

func exampleDefaultClient() {
	s, err := server.NewDefaultServer("localhost:8888", 60)
	if err != nil {
		panic(err)
	}

	s.OnConn = func(c *server.Client) {
		println("Default client connected:", c.GetID())
	}

	s.OnMsg = func(c *server.Client, msg *server.Message) {
		println("Default client message:", msg.Type)
	}

	println("Server with default client would start here...")
}

func exampleCustomClient() {
	// Servidor com client customizado usando generics
	s, err := server.New("localhost:8889", 60, NewCustomClient)
	if err != nil {
		panic(err)
	}

	// Agora os callbacks recebem *CustomClient
	s.OnConn = func(c *CustomClient) {
		println("Custom client connected:", c.GetID())
		c.Username = "player_" + c.GetID()
		c.UpdateLastSeen()
	}

	s.OnDisc = func(c *CustomClient, err error) {
		println("Custom client disconnected:", c.Username)
	}

	s.OnMsg = func(c *CustomClient, msg *server.Message) {
		c.UpdateLastSeen()
		println("Message from", c.Username, "level:", c.Level)

		switch msg.Type {
		case "admin_command":
			if !c.HasPermission("admin") {
				response := server.Message{
					Type: "error",
					Data: json.RawMessage(`{"message": "Permission denied"}`),
				}
				data, _ := json.Marshal(response)

				str, err := c.GetConn().OpenStream()
				if err == nil {
					str.Write(data)
					str.Close()
				}
				return
			}
		case "level_up":
			c.Level++
			if c.Level >= 10 {
				c.GrantPermission("admin")
			}
		}
	}

	println("Server with custom client would start here...")
}

func exampleGameClient() {
	// Servidor de jogo com client completamente customizado
	gameServer, err := server.New("localhost:8890", 60, NewGameClient)
	if err != nil {
		panic(err)
	}

	gameServer.OnConn = func(c *GameClient) {
		println("Game client connected:", c.GetID())
		c.SetMeta("connected_at", time.Now())
	}

	gameServer.OnMsg = func(c *GameClient, msg *server.Message) {
		switch msg.Type {
		case "move":
			var moveData struct {
				X, Y, Z float64 `json:"x,y,z"`
			}
			json.Unmarshal(msg.Data, &moveData)
			c.MoveTo(moveData.X, moveData.Y, moveData.Z)
			println("Player moved to:", moveData.X, moveData.Y, moveData.Z)

		case "attack":
			c.TakeDamage(10)
			println("Player took damage, health:", c.Health)

		case "pickup_item":
			item := Item{ID: "sword", Name: "Iron Sword", Quantity: 1}
			c.AddItem(item)
			println("Player picked up:", item.Name)
		}
	}

	println("Game server would start here...")
}
