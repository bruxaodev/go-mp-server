# Go MP Server - Clients Customizados com Generics

Este projeto agora permite que usuários criem seus próprios tipos de client personalizados usando **generics** do Go. Você pode definir qualquer estrutura de client que implemente a interface básica e usar diretamente nos callbacks do servidor.

## ✨ Características

- **Type Safety**: Compilador garante que você está usando o tipo correto de client
- **Zero Overhead**: Generics são resolvidos em tempo de compilação
- **Flexibilidade Total**: Crie clients com qualquer estrutura que você precisar
- **Reutilização**: Pode usar embedding para estender o client padrão
- **Interface Simples**: Apenas alguns métodos básicos são obrigatórios

## 🚀 Como Usar

### 1. Client Padrão (Mais Simples)

```go
func main() {
    // Servidor com client padrão
    s, err := server.NewDefaultServer("localhost:8888", 60)
    if err != nil {
        panic(err)
    }

    s.OnConn = func(c *server.Client) {
        println("Client conectado:", c.GetID())
    }

    s.OnMsg = func(c *server.Client, msg *server.Message) {
        // Processar mensagem usando client padrão
    }

    s.Start()
    defer s.Stop()
    select {}
}
```

### 2. Client Customizado (Embedding)

```go
// Defina seu client customizado
type CustomClient struct {
    *server.Client  // Herda funcionalidade básica

    // Seus campos específicos
    Username    string
    Level       int
    Permissions []string
}

// Factory function obrigatória
func NewCustomClient(conn *quic.Conn) *CustomClient {
    return &CustomClient{
        Client:      server.NewClient(conn),
        Username:    "anonymous",
        Level:       1,
        Permissions: []string{"read"},
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

func main() {
    // Servidor com client customizado
    s, err := server.New("localhost:8888", 60, NewCustomClient)
    if err != nil {
        panic(err)
    }

    // Callbacks recebem SEU tipo customizado!
    s.OnConn = func(c *CustomClient) {
        println("Custom client conectado:", c.GetID())
        c.Username = "player_" + c.GetID()
    }

    s.OnMsg = func(c *CustomClient, msg *server.Message) {
        // Acesso direto aos seus campos customizados
        if !c.HasPermission("write") {
            // Lógica específica do seu client
            return
        }

        switch msg.Type {
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

### 3. Client Completamente Customizado

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

// Implementar a interface obrigatória
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

// Factory function
func NewGameClient(conn *quic.Conn) *GameClient {
    return &GameClient{
        conn:      conn,
        meta:      make(map[string]interface{}),
        Position:  Point3D{0, 0, 0},
        Health:    100,
        Inventory: make([]Item, 0),
    }
}

// Métodos específicos do jogo
func (g *GameClient) MoveTo(x, y, z float64) {
    g.Position = Point3D{x, y, z}
}

func (g *GameClient) TakeDamage(damage int) {
    g.Health -= damage
}

func main() {
    gameServer, err := server.New("localhost:8888", 60, NewGameClient)
    if err != nil {
        panic(err)
    }

    gameServer.OnMsg = func(c *GameClient, msg *server.Message) {
        switch msg.Type {
        case "move":
            var pos Point3D
            json.Unmarshal(msg.Data, &pos)
            c.MoveTo(pos.X, pos.Y, pos.Z)

        case "attack":
            c.TakeDamage(10)

        case "heal":
            c.Health = min(c.Health + 20, 100)
        }
    }

    gameServer.Start()
    defer gameServer.Stop()
    select {}
}
```

## 📋 Interface Obrigatória

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

## 🛠 Funções Auxiliares do Servidor

```go
// Obter todos os clients conectados
clients := server.GetClients()

// Obter client por conexão
client, exists := server.GetClientByConn(conn)

// Broadcast para todos os clients
server.Broadcast(&server.Message{
    Type: "announcement",
    Data: json.RawMessage(`{"text": "Server announcement"}`),
})
```

## 🔄 Migração da Versão Anterior

### Antes:

```go
s, err := server.New("localhost:8888", 60)
s.OnConn = func(c *server.Client) {
    println("ID:", c.ID)  // Acesso direto
}
```

### Depois:

```go
s, err := server.NewDefaultServer("localhost:8888", 60)
s.OnConn = func(c *server.Client) {
    println("ID:", c.GetID())  // Usando método da interface
}
```

## 📁 Exemplos

Veja `examples/custom_client_usage.go` para exemplos completos de:

- Client padrão
- Client customizado com embedding
- Client de jogo totalmente customizado
- Diferentes padrões de uso e implementação

## ⚡ Vantagens

1. **Tipagem Forte**: O compilador previne erros de tipo
2. **Performance**: Zero overhead - generics são resolvidos em compile time
3. **Flexibilidade**: Qualquer estrutura que atenda a interface funciona
4. **Manutenibilidade**: Código mais limpo e expressivo
5. **Extensibilidade**: Fácil de adicionar novos campos e métodos

Esta implementação resolve completamente o problema de não poder usar tipos customizados nos callbacks do servidor, mantendo type safety e performance!
