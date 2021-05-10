package client

import (
	"dichnd/go-socket/server"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type IWsClient interface {
	SendMessage(message server.Message) error
	OnMessage(f func(message server.Message))
}

type WsClient struct {
	Url *url.URL
	Header http.Header
	conn *websocket.Conn
	onMessage func(message server.Message)
	mu sync.Mutex
	isReading bool
}

func NewWsClient(url *url.URL, header http.Header) *WsClient {
	return &WsClient{
		Url: url,
		Header: header,
		mu: sync.Mutex{},
		isReading: false,
	}
}

func (c *WsClient) Connect() error {
	go c.retryConnection()
	return nil
}

func (c *WsClient) SendMessage(message server.Message) error {
	if c.conn == nil {
		return errors.New("websocket connection disconnected")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	msgByte, err := json.Marshal(&message)
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

func (c *WsClient) OnMessage(f func(message server.Message)) {
	c.onMessage = f
}

func (c *WsClient) read() {
	c.isReading = true
	for c.conn != nil {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil && messageType == -1 {
			fmt.Println("socket error, will retry: ", err)
			c.retryConnection()
		}

		if messageType == websocket.PingMessage {
			_ = c.conn.WriteMessage(websocket.PongMessage, nil)
		} else if messageType != -1 {
			var msg server.Message
			_ = json.Unmarshal(message, &msg)
			if c.onMessage != nil {
				c.onMessage(msg)
			}
		}
	}
}

func (c *WsClient) retryConnection()  {
	for {
		conn, _, err := websocket.DefaultDialer.Dial(c.Url.String(), c.Header);
		if err == nil && conn != nil {
			c.conn = conn
			if !c.isReading {
				go c.read()
			}
			return
		}
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(5 * time.Second)
	}
}

func (*WsClient) BindKey() string {
	return "lib.socket_client"
}
