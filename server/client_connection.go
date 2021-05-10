package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

const (
	// Time allowed to write a message to the peer.
	WriteWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	PingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 64 * 1024	// 64KB
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

type ClientConnection struct {
	context *gin.Context
	conn           *websocket.Conn
	onClose        func()
	onMessage      func(message Message)
	data map[string]interface{}
	mu *sync.Mutex
}

func NewClientConnection(context *gin.Context, conn *websocket.Conn) *ClientConnection {
	return &ClientConnection{
		context: context,
		conn: conn,
		mu: &sync.Mutex{},
	}
}

func (c *ClientConnection) read() {
	ticker := time.NewTicker(PingPeriod)

	defer func() {
		ticker.Stop()
		if c.onClose != nil {
			c.onClose()
		}
		_ = c.conn.Close()
	}()

	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Println("ping")
				_ = c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		fmt.Println("pong")
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		var msg Message
		_ = json.Unmarshal(message, &msg)
		if c.onMessage != nil {
			c.onMessage(msg)
		}
	}
}

func (c *ClientConnection) SendMessage(msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(WriteWait))

	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	msgByte, err := json.Marshal(&msg)
	if nil != err {
		return err
	}
	_, err = w.Write(msgByte)
	if nil != err {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

func (c *ClientConnection) GetContext() *gin.Context {
	return c.context
}

func (c *ClientConnection) Close() error {
	return c.conn.Close()
}

func (c *ClientConnection) Set(key string, value interface{}) {
	if c.data == nil {
		c.data = map[string]interface{}{}
	}
	c.data[key] = value
}

func (c *ClientConnection) GetString(key string) (string, bool) {
	if v, ok := c.data[key]; ok {
		if stringValue, ok := v.(string); ok {
			return stringValue, true
		}
	}
	return "", false
}

func (c *ClientConnection) GetInt64(key string) (int64, bool) {
	if v, ok := c.data[key]; ok {
		if int64Value, ok := v.(int64); ok {
			return int64Value, true
		}
	}
	return 0, false
}

func (c *ClientConnection) GetInt(key string) (int, bool) {
	if v, ok := c.data[key]; ok {
		if intValue, ok := v.(int); ok {
			return intValue, true
		}
	}
	return 0, false
}

func (c *ClientConnection) GetBool(key string) (bool, bool) {
	if v, ok := c.data[key]; ok {
		if boolValue, ok := v.(bool); ok {
			return boolValue, true
		}
	}
	return false, false
}
