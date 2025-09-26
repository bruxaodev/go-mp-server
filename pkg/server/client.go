package server

type ClientInterface interface {
	GetID() string
	GetConn() *Conn
	GetMeta() map[string]interface{}
	SetID(id string)
	SetMeta(key string, value interface{})
}

type Client struct {
	ID   string
	Conn *Conn
	Meta map[string]interface{}
}

func (c *Client) GetID() string {
	return c.ID
}

func (c *Client) GetConn() *Conn {
	return c.Conn
}

func (c *Client) GetMeta() map[string]interface{} {
	return c.Meta
}

func (c *Client) SetID(id string) {
	c.ID = id
}

func (c *Client) SetMeta(key string, value interface{}) {
	if c.Meta == nil {
		c.Meta = make(map[string]interface{})
	}
	c.Meta[key] = value
}

func NewClient(conn *Conn) *Client {
	return &Client{
		ID:   "",
		Conn: conn,
		Meta: make(map[string]interface{}),
	}
}
