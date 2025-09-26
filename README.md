# Go MP Server - Clients e Messages Customizados com Generics

Este projeto permite que usuários criem seus próprios tipos de **client** e **message** personalizados usando **generics** do Go. Você pode definir qualquer estrutura de client e message que implementem as interfaces básicas e usar diretamente nos callbacks do servidor.

## ✨ Características

- **Type Safety**: Compilador garante que você está usando os tipos corretos de client e message
- **Zero Overhead**: Generics são resolvidos em tempo de compilação
- **Flexibilidade Total**: Crie clients e messages com qualquer estrutura que você precisar
- **Reutilização**: Pode usar embedding para estender o client e message padrão
- **Interface Simples**: Apenas alguns métodos básicos são obrigatórios

## 🚀 Como Usar

### 1. Client e Message Padrão (Mais Simples)

```go
func main() {
    // Servidor com client e message padrão
    s, err := server.NewDefaultServer("localhost:8888", 60)
    if err != nil {
        panic(err)
    }

    s.OnConn = func(c *server.Client) {
        println("Client conectado:", c.GetID())
    }

    s.OnMsg = func(c *server.Client, msg *server.Message) {
        // Processar mensagem usando client e message padrão
        log.Printf("Cliente %s enviou mensagem do tipo: %s", c.GetID(), msg.GetType())
    }

    s.Start()
    defer s.Stop()
    select {}
}
```

### 2. Client e Message Customizado (Embedding)

```go
// Defina seu client customizado
type CustomClient struct {
    *server.Client  // Herda funcionalidade básica

    // Seus campos específicos
    Username    string
    Level       int
    Permissions []string
}

// Defina sua message customizada
type CustomMessage struct {
    *server.Message  // Herda funcionalidade básica

    // Seus campos específicos
    Timestamp time.Time
    Priority  int
    Source    string
}

// Factory functions obrigatórias
func NewCustomClient(conn *quic.Conn) *CustomClient {
    return &CustomClient{
        Client:      server.NewClient(conn),
        Username:    "anonymous",
        Level:       1,
        Permissions: []string{"read"},
    }
}

func NewCustomMessage(msg *server.Message) *CustomMessage {
    return &CustomMessage{
        Message:   msg,
        Timestamp: time.Now(),
        Priority:  1,
        Source:    "client",
    }
}

// Métodos específicos do seu domínio
func (c *CustomClient) HasPermission(perm string) bool {
    for _, p := range c.Permissions {
        if p == perm {
            return true
        }
    }
    return false
}

func (m *CustomMessage) IsHighPriority() bool {
    return m.Priority >= 5
}

func main() {
    // Servidor com client e message customizados
    s, err := server.New("localhost:8888", 60, NewCustomClient, NewCustomMessage)
    if err != nil {
        panic(err)
    }

    // Callbacks recebem SEUS tipos customizados!
    s.OnConn = func(c *CustomClient) {
        println("Custom client conectado:", c.GetID())
        c.Username = "player_" + c.GetID()
    }

    s.OnMsg = func(c *CustomClient, msg *CustomMessage) {
        // Acesso direto aos seus campos customizados
        if !c.HasPermission("write") {
            log.Printf("Cliente %s não tem permissão para escrever", c.Username)
            return
        }

        if msg.IsHighPriority() {
            log.Printf("Mensagem de alta prioridade de %s às %v",
                      c.Username, msg.Timestamp)
        }

        switch msg.GetType() {
        case "level_up":
            c.Level++
            if c.Level >= 10 {
                c.Permissions = append(c.Permissions, "admin")
            }
        }
    }

    s.Start()
    defer s.Stop()
    select {}
}
```

### 3. Client e Message Completamente Customizados

```go
// Client totalmente customizado para um jogo
type GameClient struct {
    id       string
    conn     *quic.Conn
    meta     map[string]interface{}

    // Campos específicos do jogo
    Position  Point3D
    Health    int
    Team      string
    Inventory []Item
}

// Message totalmente customizada para um jogo
type GameMessage struct {
    msgType string
    data    json.RawMessage

    // Campos específicos do jogo
    PlayerId   string
    Action     string
    Coordinates Point3D
    Metadata   map[string]interface{}
}

// Implementar as interfaces obrigatórias
func (g *GameClient) GetID() string { return g.id }
func (g *GameClient) GetConn() *quic.Conn { return g.conn }
func (g *GameClient) GetMeta() map[string]interface{} { return g.meta }
func (g *GameClient) SetID(id string) { g.id = id }
func (g *GameClient) SetMeta(key string, value interface{}) {
    if g.meta == nil {
        g.meta = make(map[string]interface{})
    }
    g.meta[key] = value
}

func (m *GameMessage) GetType() string { return m.msgType }
func (m *GameMessage) GetData() json.RawMessage { return m.data }

// Factory functions
func NewGameClient(conn *quic.Conn) *GameClient {
    return &GameClient{
        conn:      conn,
        meta:      make(map[string]interface{}),
        Position:  Point3D{0, 0, 0},
        Health:    100,
        Inventory: make([]Item, 0),
    }
}

func NewGameMessage(msg *server.Message) *GameMessage {
    gameMsg := &GameMessage{
        msgType:  msg.GetType(),
        data:     msg.GetData(),
        Metadata: make(map[string]interface{}),
    }

    // Parse dados específicos do jogo
    var gameData map[string]interface{}
    if err := json.Unmarshal(msg.GetData(), &gameData); err == nil {
        if playerId, ok := gameData["player_id"].(string); ok {
            gameMsg.PlayerId = playerId
        }
        if action, ok := gameData["action"].(string); ok {
            gameMsg.Action = action
        }
        if coords, ok := gameData["coordinates"].(map[string]interface{}); ok {
            gameMsg.Coordinates = Point3D{
                X: coords["x"].(float64),
                Y: coords["y"].(float64),
                Z: coords["z"].(float64),
            }
        }
    }

    return gameMsg
}

// Métodos específicos do jogo
func (g *GameClient) MoveTo(x, y, z float64) {
    g.Position = Point3D{x, y, z}
}

func (g *GameClient) TakeDamage(damage int) {
    g.Health -= damage
}

func (m *GameMessage) IsMovementAction() bool {
    return m.Action == "move" || m.Action == "teleport"
}

func main() {
    gameServer, err := server.New("localhost:8888", 60, NewGameClient, NewGameMessage)
    if err != nil {
        panic(err)
    }

    gameServer.OnMsg = func(c *GameClient, msg *GameMessage) {
        log.Printf("Player %s executou ação: %s", msg.PlayerId, msg.Action)

        switch msg.Action {
        case "move":
            if msg.IsMovementAction() {
                c.MoveTo(msg.Coordinates.X, msg.Coordinates.Y, msg.Coordinates.Z)
                log.Printf("Player %s moveu para %+v", c.GetID(), c.Position)
            }

        case "attack":
            c.TakeDamage(10)
            log.Printf("Player %s tomou dano. Health: %d", c.GetID(), c.Health)

        case "heal":
            c.Health = min(c.Health + 20, 100)
            log.Printf("Player %s se curou. Health: %d", c.GetID(), c.Health)
        }
    }

    gameServer.Start()
    defer gameServer.Stop()
    select {}
}
```

## 📋 Interfaces Obrigatórias

### ClientInterface

Todo client customizado deve implementar `ClientInterface`:

```go
type ClientInterface interface {
    GetID() string
    GetConn() *quic.Conn
    GetMeta() map[string]interface{}
    SetID(id string)
    SetMeta(key string, value interface{})
}
```

### MessageInterface

Toda message customizada deve implementar `MessageInterface`:

```go
type MessageInterface interface {
    GetType() string
    GetData() json.RawMessage
}
```

### Constraints (Não implementar diretamente)

Os constraints são usados internamente pelo sistema de tipos:

```go
type ClientConstraint[T any] interface {
    *T
    ClientInterface
}

type MessageConstraint[T any] interface {
    *T
    MessageInterface
}
```

## 🛠 Funções Auxiliares do Servidor

```go
// Obter todos os clients conectados
clients := server.GetClients()

// Obter client por conexão
client, exists := server.GetClientByConn(conn)

// Broadcast para todos os clients (usa Message padrão)
server.Broadcast(&server.Message{
    Type: "announcement",
    Data: json.RawMessage(`{"text": "Server announcement"}`),
})

// Criar servidor com tipos padrão
defaultServer, err := server.NewDefaultServer("localhost:8888", 60)

// Criar servidor com tipos customizados
customServer, err := server.New("localhost:8888", 60, NewCustomClient, NewCustomMessage)
```

## 🔄 Migração da Versão Anterior

### Antes (Versão sem Generics):

```go
s, err := server.New("localhost:8888", 60)
s.OnConn = func(c *server.Client) {
    println("ID:", c.ID)  // Acesso direto
}
s.OnMsg = func(c *server.Client, msg *server.Message) {
    println("Tipo:", msg.Type)  // Acesso direto
}
```

### Depois (Versão com Generics):

```go
// Opção 1: Usar servidor padrão (mais fácil para migração)
s, err := server.NewDefaultServer("localhost:8888", 60)
s.OnConn = func(c *server.Client) {
    println("ID:", c.GetID())  // Usando método da interface
}
s.OnMsg = func(c *server.Client, msg *server.Message) {
    println("Tipo:", msg.GetType())  // Usando método da interface
}

// Opção 2: Usar tipos customizados (mais poderoso)
s, err := server.New("localhost:8888", 60, NewCustomClient, NewCustomMessage)
s.OnMsg = func(c *CustomClient, msg *CustomMessage) {
    // Agora você tem acesso a campos e métodos customizados!
    println("Username:", c.Username)
    println("É alta prioridade?", msg.IsHighPriority())
}
```

## 📁 Exemplos

Veja `examples/custom_client_usage.go` para exemplos completos de:

- Client e message padrão
- Client e message customizados com embedding
- Client e message de jogo totalmente customizados
- Diferentes padrões de uso e implementação
- Factory functions para diferentes cenários

## ⚡ Vantagens

1. **Tipagem Forte**: O compilador previne erros de tipo para clients E messages
2. **Performance**: Zero overhead - generics são resolvidos em compile time
3. **Flexibilidade**: Qualquer estrutura que atenda às interfaces funciona
4. **Manutenibilidade**: Código mais limpo e expressivo com types específicos do domínio
5. **Extensibilidade**: Fácil de adicionar novos campos e métodos em clients e messages
6. **Type Safety Completo**: Tanto clients quanto messages são type-safe nos callbacks

Esta implementação resolve completamente o problema de não poder usar tipos customizados nos callbacks do servidor, oferecendo type safety tanto para clients quanto para messages, mantendo performance máxima!
