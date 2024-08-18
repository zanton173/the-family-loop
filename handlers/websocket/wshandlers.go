package wshandlers

import (
	"fmt"
	"net/http"
	globalvars "tfl/vars"

	"github.com/gorilla/websocket"
)

var clients []*websocket.Conn

func InitialHandler(w http.ResponseWriter, r *http.Request) {
	conn, connerr := globalvars.Upgrader.Upgrade(w, r, nil)
	if connerr != nil {
		fmt.Println(connerr)
		return
	}
	clients = append(clients, conn)

	for {
		msgType, msg, msgerr := conn.ReadMessage()
		if msgerr != nil {
			return
		}
		fmt.Printf("%s send: %s\n", conn.RemoteAddr(), string(msg))
		for _, client := range clients {
			if msgerr = client.WriteMessage(msgType, msg); msgerr != nil {
				return
			}
		}
	}
}
