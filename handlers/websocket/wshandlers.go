package wshandlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

var gameCounter = 0

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
	var playertwodisc globaltypes.WebSocketPongMessage
	for idx, clientval := range clients {
		if clientval == c {
			clients = append(clients[:idx], clients[idx+1:]...)
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.pong_game_state where playerone = '%s';", c.username))
			_, playertwodisconn := globalvars.Db.Exec(fmt.Sprintf("update tfldata.pong_game_state set playertwo = '', playertwoconnected = false where playertwo = '%s';", c.username))
			if playertwodisconn == nil {
				playertwodisc = globaltypes.WebSocketPongMessage{
					Data:   c.username,
					Type:   "playertwodisconnected",
					Player: clientval.username,
				}
			}
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.pong_game_lobby where player_username = '%s';", c.username))
		}
		clientval.conn.WriteJSON(playertwodisc)

	}

}

func CreateGameSession(message globaltypes.WebSocketPongMessage, client *Client) {

	gamesession = append(gamesession, &GameState{player: client, gameId: message.Data})
}
func GameLoop(msg globaltypes.WebSocketPongMessage) {
	var sendToThem *Client
	for _, clientVal := range clients {
		if clientVal.username == msg.Player {
			sendToThem = clientVal
		}
	}
	type stateType struct {
		Id         sql.NullInt64
		PlayerOne  sql.NullString
		PlayerConn sql.NullBool
	}
	var stateResp stateType

	row := globalvars.Db.QueryRow(fmt.Sprintf("select id, playerone, playeroneconnected from tfldata.pong_game_state where ((playertwo is null or playertwo = '') and (playertwoconnected is null or playertwoconnected is false)) and playerone != '%s' limit 1;", msg.Player))

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
	if !stateResp.PlayerConn.Bool || !stateResp.PlayerOne.Valid || !stateResp.Id.Valid {
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.pong_game_state(playerone, playeroneconnected) values ('%s', true);", msg.Player))
		sendmessage := globaltypes.WebSocketPongMessage{
			Data:   "single",
			Player: msg.Player,
			Type:   msg.Player,
		}
		if msgerr := sendToThem.conn.WriteJSON(sendmessage); msgerr != nil {
			fmt.Println(msgerr)
			return
		}
	} else if stateResp.PlayerConn.Bool {
		idToStr := strconv.Itoa(int(stateResp.Id.Int64))
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.pong_game_state set playertwo = '%s', playertwoconnected = true where id = '%s' and playerone != '%s';", msg.Player, idToStr, msg.Player))
		if uperr != nil {
			fmt.Println(uperr)
		}
		sendmessage := globaltypes.WebSocketPongMessage{
			Data:   "two",
			Player: msg.Player,
			Type:   stateResp.PlayerOne.String,
		}

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
}
func sendScore(msg globaltypes.WebSocketPongMessage) {

	var gameid sql.NullInt64
	globalvars.Db.QueryRow(fmt.Sprintf("select id from tfldata.pong_game_state where playerone = '%s' and playertwo = '%s';", strings.Split(msg.Data, ",")[0], strings.Split(msg.Data, ",")[1])).Scan(&gameid)
	if !gameid.Valid {
		fmt.Println("game not found")
		return
	}
	_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.pong_match_history(playeronename, playertwoname, playeronescore, playertwoscore, matchid, createdon) values ('%s', '%s', '%s', '%s', %d, now());", strings.Split(msg.Data, ",")[0], strings.Split(msg.Data, ",")[1], strings.Split(msg.Data, ",")[2], strings.Split(msg.Data, ",")[3], gameid.Int64))
	if inserr != nil {
		fmt.Println(inserr)
		return
	}
	globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.pong_game_state where id = %d;", gameid.Int64))
}
func InitialHandler(w http.ResponseWriter, r *http.Request) {
	conn, connerr := globalvars.Upgrader.Upgrade(w, r, nil)
	if connerr != nil {
		fmt.Printf("connn err: %s", connerr.Error())
		return
	}

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
			return
		}
		if msgtype == websocket.CloseMessage {
			client.CloseConn()
			return
		}

		var newMessage globaltypes.WebSocketPongMessage
		json.Unmarshal(msg, &newMessage)
		if newMessage.Type == "postmatchscore" {
			sendScore(newMessage)
		}
		if newMessage.Type == "setspeed" {

			gameCounter++

			if gameCounter%4 == 0 {
				clients[0].mutex.Lock()
				clients[1].mutex.Lock()
				gameBallUpdateSpeedMsg := globaltypes.WebSocketPongMessage{
					Data:   "",
					Type:   "updatespeed",
					Player: "",
				}

				if msgerr = clients[0].conn.WriteJSON(gameBallUpdateSpeedMsg); msgerr != nil {
					fmt.Println(msgerr)
					clients[0].mutex.Unlock()
					return
				}
				if msgerr = clients[1].conn.WriteJSON(gameBallUpdateSpeedMsg); msgerr != nil {
					fmt.Println(msgerr)
					clients[1].mutex.Unlock()
					return
				}
				clients[0].mutex.Unlock()
				clients[1].mutex.Unlock()
			}
		}
		for _, client := range clients {

			if newMessage.Type == "lobby" {
				if newMessage.Player == currentUserFromSession {
					GameLoop(newMessage)
					break
				}
			} else if newMessage.Type == "game" {
				if client.username != newMessage.Player {
					client.mutex.Lock()
					if msgerr = client.conn.WriteJSON(newMessage); msgerr != nil {
						client.mutex.Unlock()
						return
					}
					client.mutex.Unlock()

				}
			} else if newMessage.Type == "playerpoint" {
				time.Sleep(time.Millisecond * 20)
				client.mutex.Lock()
				gameCounter = 0
				if msgerr = client.conn.WriteJSON(newMessage); msgerr != nil {
					client.mutex.Unlock()
					return
				}
				client.mutex.Unlock()
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
