package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/donaldtrieuit/go-socket/server"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const MaxMessageQueueSize = 1000

type IWsClient interface {
	SendMessage(message server.Message) error
	ReadMessage() (server.Message, error)
	Close() error
}

type WsClient struct {
	Url            *url.URL
	Header         http.Header
	conn           *websocket.Conn
	messageChannel chan server.Message
	mu             sync.Mutex
	destructor     sync.Once // shutdown once
	errors         []chan *error
	noNotify       bool
	isReading      bool
	isClosed       bool
}

func NewWsClient(url *url.URL, header http.Header) *WsClient {
	return &WsClient{
		Url:            url,
		Header:         header,
		mu:             sync.Mutex{},
		isReading:      false,
		isClosed:       false,
		noNotify:       false,
		messageChannel: make(chan server.Message, MaxMessageQueueSize),
	}
}

func (c *WsClient) Connect() error {
	go c.retryConnection()
	return nil
}

func (c *WsClient) Close() error {
	if c.conn != nil {
		c.isClosed = true
		return c.conn.Close()
	}
	defer c.shutdown(nil)
	return nil
}

func (c *WsClient) NotifyClose(receiver chan *error) chan *error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.noNotify {
		close(receiver)
	} else {
		c.errors = append(c.errors, receiver)
	}
	return receiver
}

func (c *WsClient) SendMessage(message server.Message) error {
	if c.conn == nil {
		err := errors.New("websocket connection disconnected")
		c.shutdown(&err)
		return err
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

func (c *WsClient) ReadMessage() (server.Message, error) {
	msg, ok := <-c.messageChannel
	if !ok {
		err := errors.New("socket closed")
		c.shutdown(&err)
		return server.Message{}, err
	}
	return msg, nil
}

func (c *WsClient) read() {
	c.isReading = true
	for c.conn != nil && !c.isClosed {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				c.shutdown(&err)
				close(c.messageChannel)
				break
			}
			if messageType == -1 {
				fmt.Println("socket error, will retry: ", err)
				c.retryConnection()
			}
		}

		if messageType == websocket.PingMessage {
			_ = c.conn.WriteMessage(websocket.PongMessage, nil)
		} else if messageType != -1 {
			var msg server.Message
			err := json.Unmarshal(message, &msg)

			if err != nil {
				msg = server.Message{
					Data: string(message),
				}
			}

			if len(c.messageChannel) == MaxMessageQueueSize {
				<-c.messageChannel
			}
			c.messageChannel <- msg
		}
	}
}

func (c *WsClient) retryConnection() {
	for !c.isClosed {
		conn, _, err := websocket.DefaultDialer.Dial(c.Url.String(), c.Header)
		if err == nil && conn != nil {
			c.conn = conn
			if !c.isReading {
				go c.read()
			}

			return
		}
		if err != nil {
			c.shutdown(&err)
			fmt.Println(err, c.Url.String())
		}

		time.Sleep(5 * time.Second)
	}
}

func (c *WsClient) shutdown(err *error) {
	c.destructor.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if err != nil {
			for _, c := range c.errors {
				c <- err
			}
		}
		for _, c := range c.errors {
			close(c)
		}
		c.noNotify = true
	})
}

func (*WsClient) BindKey() string {
	return "lib.socket_client"
}
