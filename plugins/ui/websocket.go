package ui

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/labstack/echo"
	"golang.org/x/net/websocket"
)

type socket struct {
	conn *websocket.Conn
}

var ws socket
var wsMutex sync.Mutex

func (sock socket) send(msg interface{}) {
	payload, err := json.Marshal(msg)
	if err == nil && sock.conn != nil {
		fmt.Fprint(sock.conn, string(payload))
	}
}

func upgrader(c echo.Context) error {

	websocket.Handler(func(conn *websocket.Conn) {
		wsMutex.Lock()
		ws.conn = conn
		wsMutex.Unlock()
		defer conn.Close()
		for {
			msg := ""
			err := websocket.Message.Receive(conn, &msg)
			if err != nil {
				//c.Logger().Error(err)
				break
			}
			fmt.Printf("%s\n", msg)
		}
	}).ServeHTTP(c.Response(), c.Request())

	return nil
}
