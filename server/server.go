package server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WsServer struct {
	onConnection func(connection *ClientConnection)
	onCloseConnection func(connection *ClientConnection)
	onMessage func(message Message, connection *ClientConnection)
	Middlewares []gin.HandlerFunc
}

func (s *WsServer) Start(addr string) error {
	router := gin.Default()
	router.GET("/health", func(context *gin.Context) {
		context.JSON(200, nil)
	})
	var handlers []gin.HandlerFunc
	handlers = append(handlers, s.Middlewares...)
	handlers = append(handlers, func(context *gin.Context) {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
		conn, err := upgrader.Upgrade(context.Writer, context.Request, nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		client := NewClientConnection(context, conn)

		client.onClose = func() {
			if s.onCloseConnection != nil {
				s.onCloseConnection(client)
			}
		}
		client.onMessage = func(message Message) {
			if s.onMessage != nil {
				s.onMessage(message, client)
			}
		}

		go client.read()

		s.onConnection(client)
	})
	router.Any("/ws/socket", handlers...)

	return router.Run(addr)
}

func (s *WsServer) OnConnection(f func(connection *ClientConnection)) {
	s.onConnection = f
}

func (s *WsServer) OnCloseConnection(f func(connection *ClientConnection)) {
	s.onCloseConnection = f
}

func (s *WsServer) OnMessage(f func(message Message, connection *ClientConnection)) {
	s.onMessage = f
}
