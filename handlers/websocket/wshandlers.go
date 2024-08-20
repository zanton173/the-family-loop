package wshandlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	globalfunctions "tfl/functions"
	globaltypes "tfl/types"
	globalvars "tfl/vars"

	"github.com/gorilla/websocket"
)

var clients = make([]*Client, 0)

type Client struct {
	conn  *websocket.Conn
	mutex sync.Mutex
}

func InitialHandler(w http.ResponseWriter, r *http.Request) {
	conn, connerr := globalvars.Upgrader.Upgrade(w, r, nil)
	if connerr != nil {
		fmt.Println(connerr)
		return
	}
	fmt.Println(r.RemoteAddr)

	allowOrDeny, _, _ := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	if allowOrDeny {
		client := &Client{conn: conn}
		clients = append(clients, client)
	}
	defer conn.Close()
	for {

		msgtype, msg, msgerr := conn.ReadMessage()
		if msgerr != nil {
			fmt.Println(msgerr)
			return
		}
		if msgtype == websocket.CloseMessage {
			return
		}
		var newMessage globaltypes.WebSocketPongMessage
		json.Unmarshal(msg, &newMessage)
		fmt.Println(newMessage)

		for _, client := range clients {
			client.mutex.Lock()
			if msgerr = client.conn.WriteJSON(newMessage); msgerr != nil {
				client.mutex.Unlock()
				return
			}
			client.mutex.Unlock()
		}
	}
}
func CloseConn(w http.ResponseWriter, r *http.Request) {

	// Send a WebSocket close message
	/*for _, client := range Clients {

		deadline := time.Now().Add(time.Minute)
		err := client.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			deadline,
		)
		if err != nil {
			fmt.Println(err)
		}
	}*/
}
