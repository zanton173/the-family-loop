package wshandlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	globalfunctions "tfl/functions"
	globaltypes "tfl/types"
	globalvars "tfl/vars"
	"time"

	"github.com/gorilla/websocket"
)

var clients = make([]*Client, 0)

type Client struct {
	conn     *websocket.Conn
	username string
	mutex    sync.Mutex
}

type GameState struct {
	player *Client
	gameId string
}

var gamesession = make([]*GameState, 2)

func (c *Client) CloseConn() {

	// Send a WebSocket close message
	deadline := time.Now().Add(time.Minute)
	err := c.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		deadline,
	)
	if err != nil {
		fmt.Println(err)
	}
	for idx, clientval := range clients {
		if clientval == c {
			clients = append(clients[:idx], clients[idx+1:]...)
		}
	}

}

func CreateGameSession(message globaltypes.WebSocketPongMessage, client *Client) {
	fmt.Println(message)
	gamesession = append(gamesession, &GameState{player: client, gameId: message.Data})
}
func GameLoop(playerMessageFor string, msg globaltypes.WebSocketPongMessage) {
	var sendToThem *Client
	for _, clientVal := range clients {
		if clientVal.username == playerMessageFor {
			sendToThem = clientVal
		}
	}
	type stateType struct {
		Id         sql.NullInt64
		PlayerOne  sql.NullString
		PlayerConn sql.NullBool
	}
	var stateResp stateType

	row := globalvars.Db.QueryRow("select id, playerone, playeroneconnected from tfldata.pong_game_state where playertwo is null and playertwoconnected is null limit 1;")

	row.Scan(&stateResp.Id, &stateResp.PlayerOne, &stateResp.PlayerConn)

	if !stateResp.PlayerOne.Valid {
		stateResp.PlayerOne.String = ""
	}
	if !stateResp.PlayerConn.Valid {
		stateResp.PlayerConn.Bool = false
	}
	if !stateResp.Id.Valid {
		stateResp.Id.Int64 = 0
	}
	fmt.Print("state resp: ")
	fmt.Println(stateResp)
	if !stateResp.PlayerConn.Bool || !stateResp.PlayerOne.Valid || !stateResp.Id.Valid {
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.pong_game_state(playerone, playeroneconnected) values ('%s', true);", msg.Player))
		sendmessage := globaltypes.WebSocketPongMessage{
			Data:   "single",
			Player: msg.Player,
			Type:   msg.Player,
		}
		fmt.Print("send message playerone: ")
		fmt.Print(sendmessage)
		fmt.Println(" to: " + sendToThem.username)
		if msgerr := sendToThem.conn.WriteJSON(sendmessage); msgerr != nil {
			fmt.Println(msgerr)
			return
		}
	} else if stateResp.PlayerConn.Bool {
		idToStr := strconv.Itoa(int(stateResp.Id.Int64))
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.pong_game_state set playertwo = '%s', playertwoconnected = true where id = '%s';", msg.Player, idToStr))
		if uperr != nil {
			fmt.Println(uperr)
		}
		sendmessage := globaltypes.WebSocketPongMessage{
			Data:   "two",
			Player: msg.Player,
			Type:   msg.Player,
		}
		fmt.Print("send message playertwo: ")
		fmt.Print(sendmessage)
		fmt.Println(" to: " + sendToThem.username)
		var newClient *Client
		for _, clientSendVal := range clients {
			if clientSendVal.username == stateResp.PlayerOne.String {
				newClient = clientSendVal
			}
		}
		if msgerr := sendToThem.conn.WriteJSON(sendmessage); msgerr != nil {
			fmt.Println(msgerr)
			return
		}
		if msgerr := newClient.conn.WriteJSON(sendmessage); msgerr != nil {
			fmt.Println(msgerr)
			return
		}
	}

	/*for {

	}*/
}
func InitialHandler(w http.ResponseWriter, r *http.Request) {
	conn, connerr := globalvars.Upgrader.Upgrade(w, r, nil)
	if connerr != nil {
		fmt.Printf("connn err: %s", connerr.Error())
		return
	}
	fmt.Println("remote add: " + r.RemoteAddr)

	allowOrDeny, currentUserFromSession, _ := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)
	var client *Client
	if allowOrDeny {
		client = &Client{conn: conn, username: currentUserFromSession}
		clients = append(clients, client)
	}

	defer conn.Close()
	for {

		msgtype, msg, msgerr := conn.ReadMessage()

		if msgerr != nil {
			fmt.Printf("msgerr: %s\n", msgerr)
			client.CloseConn()
			return
		}
		if msgtype == -1 {
			client.CloseConn()
		}
		if msgtype == websocket.CloseMessage {
			client.CloseConn()
			return
		}
		var newMessage globaltypes.WebSocketPongMessage
		json.Unmarshal(msg, &newMessage)

		/*for _, sessionclient := range gamesession{
			if newMessage.Type == "game" {

			}
		}*/

		for _, client := range clients {
			if newMessage.Type == "lobby" {
				//CreateGameSession(newMessage, client)
				if newMessage.Player == currentUserFromSession {
					go GameLoop(newMessage.Player, newMessage)
					break
				}
			} else {
				client.mutex.Lock()
				if msgerr = client.conn.WriteJSON(newMessage); msgerr != nil {
					client.mutex.Unlock()
					return
				}
				client.mutex.Unlock()
			}
		}
	}
}
