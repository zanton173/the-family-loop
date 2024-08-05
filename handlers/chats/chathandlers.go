package chathandler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	globalfunctions "tfl/functions"
	globaltypes "tfl/types"
	globalvars "tfl/vars"
	"time"
)

func CreateGroupChatMessageHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	chatMessage := globalvars.Replacer.Replace(r.PostFormValue("gchatmessage"))

	encryptedChatMessage, encrypterr := globalfunctions.Encrypt(chatMessage)
	if encrypterr != nil {
		fmt.Println(encrypterr)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity,createdon) values(substr('%s',0,105),'Err encrypting g chat message' now());", encrypterr.Error()))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	listOfUsersTagged := strings.Split(r.PostFormValue("taggedUser"), ",")

	threadVal := r.PostFormValue("threadval")
	if threadVal == "" {
		threadVal = "main thread"
	} else if strings.ToLower(threadVal) == "posts" || strings.ToLower(threadVal) == "calendar" {
		w.WriteHeader(http.StatusConflict)
		return
	}

	_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.gchat(\"chat\", \"author\", \"createdon\", \"thread\") values(E'%s', '%s', now(), '%s');", encryptedChatMessage, currentUserFromSession, threadVal))
	if inserr != nil {
		fmt.Println("error here: " + inserr.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, ttbleerr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.threads(\"thread\", \"threadauthor\", \"createdon\") values(E'%s', '%s', now());", threadVal, currentUserFromSession))
	if ttbleerr != nil {
		if strings.Contains(ttbleerr.Error(), "duplicate key") {
			fmt.Println("duplicate thread error can be ignored")
			s := make([]string, 0)
			s = append(s, "insert into tfldata.users_to_threads(username) select distinct(username) from tfldata.users;")
			s = append(s, fmt.Sprintf("update tfldata.users_to_threads set is_subscribed=true, thread='%s' where is_subscribed is null and thread is null;", threadVal))
			upAndInsUTT, txnerr := globalvars.Db.Begin()
			if txnerr != nil {
				fmt.Println(upAndInsUTT)
				fmt.Println("This was a transaction error")
			}
			defer func() {
				_ = upAndInsUTT.Rollback()
			}()
			for _, q := range s {
				_, err := upAndInsUTT.Exec(q)

				if err != nil {
					fmt.Println(err)
				}
			}
			upAndInsUTT.Commit()
		} else {
			fmt.Println("We shouldn't ignore this error: " + ttbleerr.Error())
		}
	}
	var chatMessageNotificationOpts globaltypes.NotificationOpts
	chatMessageNotificationOpts.ExtraPayloadKey = "thread"
	chatMessageNotificationOpts.ExtraPayloadVal = threadVal
	chatMessageNotificationOpts.NotificationPage = "groupchat"
	chatMessageNotificationOpts.NotificationTitle = "message in: " + threadVal
	chatMessageNotificationOpts.NotificationBody = strings.ReplaceAll(chatMessage, "\\", "")

	var singleUserChatMessageNotificationOpts globaltypes.NotificationOpts
	singleUserChatMessageNotificationOpts.ExtraPayloadKey = "thread"
	singleUserChatMessageNotificationOpts.ExtraPayloadVal = threadVal
	singleUserChatMessageNotificationOpts.NotificationPage = "groupchat"
	singleUserChatMessageNotificationOpts.NotificationTitle = currentUserFromSession + " just tagged you in : " + threadVal
	singleUserChatMessageNotificationOpts.NotificationBody = chatMessage

	go globalfunctions.SendNotificationToAllUsers(globalvars.Db, currentUserFromSession, globalvars.Fb_message_client, &chatMessageNotificationOpts)

	if len(listOfUsersTagged[0]) > 0 {
		for _, taggedUser := range listOfUsersTagged {
			var fcmToken string
			row := globalvars.Db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username='%s';", taggedUser))

			scnerr := row.Scan(&fcmToken)
			if scnerr == nil {
				go globalfunctions.SendNotificationToSingleUser(globalvars.Db, globalvars.Fb_message_client, fcmToken, singleUserChatMessageNotificationOpts)
			}

		}
	}
	w.Header().Set("HX-Trigger", "success-send")

}
func CreatePrivatePChatMessageHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	userTo := r.PostFormValue("user_to")
	message := r.PostFormValue("privatechatmessage")

	encryptedChatMessage, encrypterr := globalfunctions.Encrypt(message)
	if encrypterr != nil {
		fmt.Println(encrypterr)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity,createdon) values(substr('%s',0,105),'Err encrypting g chat message' now());", encrypterr.Error()))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, dbinserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.pchat(message, from_user, to_user, createdon) values (substr('%s',0,420), '%s', '%s', now());", encryptedChatMessage, currentUserFromSession, userTo))
	if dbinserr != nil {
		activityStr := "insert into pchat table"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, createdon, activity) values (substr('%s',0,106), now(), substr('%s',0,105));", dbinserr.Error(), activityStr))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var fcmTokenToUser sql.NullString

	fcmRes := globalvars.Db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username = '%s';", userTo))

	fcmRes.Scan(&fcmTokenToUser)

	if fcmTokenToUser.Valid {

		var chatMessageNotificationOpts globaltypes.NotificationOpts
		chatMessageNotificationOpts.ExtraPayloadKey = "direct"
		chatMessageNotificationOpts.ExtraPayloadVal = "groupchat"
		chatMessageNotificationOpts.NotificationPage = "groupchat"
		chatMessageNotificationOpts.NotificationTitle = "message from: " + currentUserFromSession
		chatMessageNotificationOpts.NotificationBody = strings.ReplaceAll(message, "\\", "")

		go globalfunctions.SendNotificationToSingleUser(globalvars.Db, globalvars.Fb_message_client, fcmTokenToUser.String, chatMessageNotificationOpts)
	}
}
func GetSelectedChatHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var ChatVal string
	row := globalvars.Db.QueryRow(fmt.Sprintf("select chat from tfldata.gchat where id='%s';", r.URL.Query().Get("chatid")))
	row.Scan(&ChatVal)
	decryptedMessage, decrypterr := globalfunctions.Decrypt(ChatVal)
	if decrypterr != nil {
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105),'failure to decrypt gchat message' now());", decrypterr.Error()))
		decryptedMessage = "Could not decrypt this message"
	}
	marshbs, marsherr := json.Marshal(&decryptedMessage)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	w.Write(marshbs)
}
func GetUsernamesToTagHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	searchOutput, searchErr := globalvars.Db.Query("select username from tfldata.users where username like '%" + r.URL.Query().Get("user") + "%';")
	if searchErr != nil {
		w.Write([]byte("no results found"))
	}
	defer searchOutput.Close()
	var sliceOfResults []string
	var tmpResult string
	for searchOutput.Next() {

		searchOutput.Scan(&tmpResult)
		sliceOfResults = append(sliceOfResults, tmpResult)
	}
	jsonbs, marsherr := json.Marshal(sliceOfResults)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	w.Write(jsonbs)
}
func GetGroupChatMessagesHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if currentUserFromSession < " " {
		currentUserFromSession = "Guest"
	}
	orderAscOrDesc := "asc"
	if r.URL.Query().Get("order_option") == "true" {
		orderAscOrDesc = "asc"
	} else {
		orderAscOrDesc = "desc"
	}
	limitVal, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	output, err := globalvars.Db.Query(fmt.Sprintf("select id, chat, author, createdon at time zone (select mytz from tfldata.users where username='%s') from (select * from tfldata.gchat where thread='%s' order by createdon DESC limit %d) as tmp order by createdon %s;", currentUserFromSession, r.URL.Query().Get("threadval"), limitVal, orderAscOrDesc))

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	defer output.Close()
	for output.Next() {
		var gchatid string
		var message string
		var author string
		var createdat time.Time
		var formatCreatedatTime string
		var pfpImg string

		output.Scan(&gchatid, &message, &author, &createdat)

		row := globalvars.Db.QueryRow(fmt.Sprintf("select pfp_name from tfldata.users where username='%s';", author))
		pfpscnerr := row.Scan(&pfpImg)
		if pfpscnerr != nil {
			pfpImg = "assets/96x96/ZCAN2301 The Family Loop Favicon_W_96 x 96.png"
		} else {
			pfpImg = "https://" + globalvars.Cfdistro + "/pfp/" + pfpImg
		}
		if time.Now().UTC().Sub(createdat) > (72 * time.Hour) {
			formatCreatedatTime = time.DateOnly

		} else if time.Now().UTC().Sub(createdat) > (24 * time.Hour) {
			formatCreatedatTime = time.ANSIC
			formatCreatedatTime = strings.Split(formatCreatedatTime, " ")[0]
		} else {
			formatCreatedatTime = time.Kitchen
		}
		editDelBtn := ""
		if author == currentUserFromSession {
			editDelBtn = "<i class='bi bi-three-dots-vertical px-1' onclick='editOrDeleteChat(`" + gchatid + "`)'></i>"
		}

		decryptedMessage, decrypterr := globalfunctions.Decrypt(message)
		if decrypterr != nil {
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105),'failure to decrypt gchat message' now());", decrypterr.Error()))
			decryptedMessage = "<i>Could not decrypt this message</i>"
		}

		dataStr := ""
		if author == currentUserFromSession {
			dataStr = "<div class='container gchatmessagecardme' style='width: 95&percnt;;'><div class='row'><b class='col-2 px-1'>me</b><div class='row'><img style='position: sticky; width: 15%; align-self: baseline;' class='col-2 px-2 my-1' src='" + pfpImg + "' alt='tfl pfp' /><p class='col-10' style='padding-right: 0'>" + decryptedMessage + "</p></div></div><div class='row'><p class='col' style='margin-left: 2rem; text-align: right;; font-size: smaller; margin-bottom: 0%'>" + createdat.Format(formatCreatedatTime) + editDelBtn + "</p></div></div>"
		} else {
			dataStr = "<div class='container gchatmessagecardfrom' style='width: 95&percnt;;'><div class='row'><b class='col-2 px-1'>" + author + "</b><div class='row'><img style='position: sticky; width: 15%; align-self: baseline;' class='col-2 px-2 my-1' src='" + pfpImg + "' alt='tfl pfp' /><p class='col-10' style='padding-right: 0'>" + decryptedMessage + "</p></div></div><div class='row'><p class='col' style='margin-left: 2rem; text-align: right;; font-size: smaller; margin-bottom: 0%'>" + createdat.Format(formatCreatedatTime) + editDelBtn + "</p></div></div>"
		}
		chattmp, tmperr := template.New("gchat").Parse(dataStr)
		if tmperr != nil {
			fmt.Println(tmperr)
		}
		chattmp.Execute(w, nil)

	}
}

func GetUsersSubscribedThreadsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	output, outerr := globalvars.Db.Query(fmt.Sprintf("select thread, is_subscribed::text from tfldata.users_to_threads where username='%s';", r.URL.Query().Get("username")))
	if outerr != nil {
		fmt.Println(outerr)
	}
	defer output.Close()

	for output.Next() {
		var thread string
		var isSubbed string
		output.Scan(&thread, &isSubbed)
		w.Write([]byte(thread + "," + isSubbed + "\n"))
	}

}
func GetUsersToChatToHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	distinctUsersOutput, queryErr := globalvars.Db.Query(fmt.Sprintf("select distinct(username) from tfldata.users where username != '%s';", currentUserFromSession))
	if queryErr != nil {
		fmt.Println(queryErr)
	}
	defer distinctUsersOutput.Close()
	for distinctUsersOutput.Next() {
		var user string
		scnerr := distinctUsersOutput.Scan(&user)
		if scnerr != nil {
			fmt.Print("scan error: " + scnerr.Error())
		}
		dataStr := fmt.Sprintf("<option value='%s'>%s</option>", user, user)

		w.Write([]byte(dataStr))
	}
}
func GetOpenThreadsHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	distinctThreadsOutput, queryErr := globalvars.Db.Query("select thread,threadauthor from tfldata.threads order by createdon desc;")
	if queryErr != nil {
		fmt.Println(queryErr)
	}
	defer distinctThreadsOutput.Close()
	for distinctThreadsOutput.Next() {
		var thread string
		var threadAuthor string
		scnerr := distinctThreadsOutput.Scan(&thread, &threadAuthor)
		if scnerr != nil {
			fmt.Print("scan error: " + scnerr.Error())
		}
		dataStr := fmt.Sprintf("<option id='%s' value='%s'>%s</option>", threadAuthor, thread, thread)

		w.Write([]byte(dataStr))
	}
}
func GetSelectedPChatHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var ChatVal string
	row := globalvars.Db.QueryRow(fmt.Sprintf("select message from tfldata.pchat where id='%s';", r.URL.Query().Get("chatid")))
	row.Scan(&ChatVal)
	decryptedMessage, decrypterr := globalfunctions.Decrypt(ChatVal)
	if decrypterr != nil {
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105),'failure to decrypt gchat message' now());", decrypterr.Error()))
		decryptedMessage = "Could not decrypt this message"
	}
	marshbs, marsherr := json.Marshal(&decryptedMessage)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	w.Write(marshbs)
}
func GetPrivateChatMessagesHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type pchatResponseStruct struct {
		id                  int
		chatMessage         string
		fromUser            string
		toUser              string
		reaction            sql.NullString
		createdOn           time.Time
		formatCreatedOnTime string
	}
	orderAscOrDesc := "asc"
	if r.URL.Query().Get("order_option") == "true" {
		orderAscOrDesc = "asc"
	} else {
		orderAscOrDesc = "desc"
	}

	limitVal, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	output, err := globalvars.Db.Query(fmt.Sprintf("select id,message,from_user,to_user,reaction,createdon at time zone (select mytz from tfldata.users where username='%s') from (select * from tfldata.pchat where (from_user='%s' and to_user='%s') or (from_user='%s' and to_user='%s') order by createdon DESC limit %d) as tmp order by createdon %s;", currentUserFromSession, r.URL.Query().Get("userToSendTo"), currentUserFromSession, currentUserFromSession, r.URL.Query().Get("userToSendTo"), limitVal, orderAscOrDesc))

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Println(err.Error())
		return
	}

	defer output.Close()
	var pChatRow pchatResponseStruct
	for output.Next() {
		var pfpimg string
		scnerr := output.Scan(&pChatRow.id, &pChatRow.chatMessage, &pChatRow.fromUser, &pChatRow.toUser, &pChatRow.reaction, &pChatRow.createdOn)
		if scnerr != nil {
			fmt.Println(scnerr)
		}
		if !pChatRow.reaction.Valid {
			pChatRow.reaction.String = ""
		}

		pfprow := globalvars.Db.QueryRow(fmt.Sprintf("select pfp_name from tfldata.users where username='%s';", pChatRow.fromUser))

		pfpscnerr := pfprow.Scan(&pfpimg)
		if pfpscnerr != nil {
			pfpimg = "assets/96x96/ZCAN2301 The Family Loop Favicon_W_96 x 96.png"
		} else {
			pfpimg = "https://" + globalvars.Cfdistro + "/pfp/" + pfpimg
		}
		if time.Now().UTC().Sub(pChatRow.createdOn) > (72 * time.Hour) {
			pChatRow.formatCreatedOnTime = time.DateOnly

		} else if time.Now().UTC().Sub(pChatRow.createdOn) > (24 * time.Hour) {
			pChatRow.formatCreatedOnTime = time.ANSIC
			pChatRow.formatCreatedOnTime = strings.Split(pChatRow.formatCreatedOnTime, " ")[0]
		} else {
			pChatRow.formatCreatedOnTime = time.Kitchen
		}
		editDelBtn := ""
		if pChatRow.fromUser == currentUserFromSession {
			editDelBtn = fmt.Sprintf("<i class='bi bi-three-dots-vertical px-1' onclick='editOrDeletePChat(`%d`)'></i>", pChatRow.id)
		}
		dataStr := ""
		decryptedMessage, decrypterr := globalfunctions.Decrypt(pChatRow.chatMessage)
		if decrypterr != nil {
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105),'failure to decrypt gchat message' now());", decrypterr.Error()))
			decryptedMessage = "<i>Could not decrypt this message</i>"
		}
		if pChatRow.toUser == currentUserFromSession {
			dataStr = "<div id='pchatid_" + fmt.Sprint(pChatRow.id) + "'class='container gchatmessagecardfrom' style='width: 95&percnt;;'><div class='row'><b class='col-2 px-1'>" + pChatRow.fromUser + "</b><div class='row'><img style='width: 15%; position: sticky; align-self: baseline;' class='col-2 px-2 my-1' src='" + pfpimg + "' alt='tfl pfp' /></div><p class='col-10' style='position: relative; left: 13%; margin-bottom: 1%; margin-top: -15%; overflow-wrap: anywhere; padding-right: 0%;'>" + decryptedMessage + "</p></div><div class='row'><div class='col' style='position: relative; margin-right: 0&percnt;; width: auto; display: flex; justify-content: flex-start' id='reactionid_" + fmt.Sprint(pChatRow.id) + "'>" + pChatRow.reaction.String + "</div><p class='col' style='margin-left: 2rem; text-align: right;; font-size: smaller; margin-bottom: 0%'>" + pChatRow.createdOn.Format(pChatRow.formatCreatedOnTime) + editDelBtn + "</p></div></div>"
		} else {
			dataStr = "<div class='container gchatmessagecardme' style='width: 95&percnt;;'><div class='row'><div class='row'><b class='col-2 px-1'>me</b><div class='row'><img style='width: 15%; position: sticky; align-self: baseline;' class='col-2 px-2 my-1' src='" + pfpimg + "' alt='tfl pfp' /></div><p class='col-10' style='position: relative; left: 13%; margin-bottom: 1%; margin-top: -15%; overflow-wrap: anywhere; padding-right: 0%;'>" + decryptedMessage + "</p></div><div class='col' style='position: relative; margin-right: 0&percnt;; width: auto; display: flex; justify-content: flex-start' id='reactionid_" + fmt.Sprint(pChatRow.id) + "'>" + pChatRow.reaction.String + "</div><p class='col' style='margin-left: 2rem; text-align: right;; font-size: smaller; margin-bottom: 0%'>" + pChatRow.createdOn.Format(pChatRow.formatCreatedOnTime) + editDelBtn + "</p></div></div>"
		}
		chattmp, tmperr := template.New("pchat").Parse(dataStr)
		if tmperr != nil {
			fmt.Println(tmperr)
		}
		chattmp.Execute(w, nil)

	}
}
func GetCurrentPChatReactionHandler(w http.ResponseWriter, r *http.Request) {
	var reactionStr sql.NullString
	reactionRow := globalvars.Db.QueryRow(fmt.Sprintf("select reaction from tfldata.pchat where id='%s';", r.URL.Query().Get("chatid")))
	reactionRow.Scan(&reactionStr)
	if reactionStr.Valid {
		w.Write([]byte(reactionStr.String))
	} else {
		return
	}
}
func ChangeGchatOrderOptHandler(w http.ResponseWriter, r *http.Request) {
	bs, _ := io.ReadAll(r.Body)
	type postBody struct {
		Username string `json:"username"`
		Option   bool   `json:"order_option"`
	}
	var postData postBody
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set gchat_order_option='%t' where username='%s';", postData.Option, postData.Username))
	if uperr != nil {
		activityStr := fmt.Sprintf("%s tried to update gchat_order_option", postData.Username)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}

}
func UpdateSelectedChatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type postBody struct {
		ChatMessage    string `json:"newMessage"`
		SelectedChatId string `json:"selectedChatId"`
	}
	var postData postBody
	bs, _ := io.ReadAll(r.Body)
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	encryptedChatMessage, encrypterr := globalfunctions.Encrypt(postData.ChatMessage)
	if encrypterr != nil {
		fmt.Println(encrypterr)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity,createdon) values(substr('%s',0,105),'Err encrypting g chat message' now());", encrypterr.Error()))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.gchat set chat='%s' where id='%s';", encryptedChatMessage, postData.SelectedChatId))
	if uperr != nil {
		activityStr := fmt.Sprintf("%s could not edit the chat message %s", currentUserFromSession, postData.SelectedChatId)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}
}
func UpdateLastViewedThreadHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type postBody struct {
		LastViewed string `json:"last_viewed"`
	}
	var postData postBody
	bs, _ := io.ReadAll(r.Body)
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		activityStr := "marsh json updatelastviewedthread function"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),substr('%s',0,106), now());", marsherr.Error(), activityStr))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set last_viewed_gchat = '%s' where username = '%s';", postData.LastViewed, currentUserFromSession))
	if uperr != nil {
		activityStr := "update error updatelastviewedthread function"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),substr('%s',0,106), now());", uperr.Error(), activityStr))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
func UpdateLastViewedPChatHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type postBody struct {
		LastViewed string `json:"last_viewed"`
	}
	var postData postBody
	bs, _ := io.ReadAll(r.Body)
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		activityStr := "marsh json updatelastviewed function"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),substr('%s',0,106), now());", marsherr.Error(), activityStr))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set last_viewed_pchat = '%s' where username = '%s';", postData.LastViewed, currentUserFromSession))
	if uperr != nil {
		activityStr := "update error updatelastviewed function"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),substr('%s',0,106), now());", uperr.Error(), activityStr))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
func ChangeUserSubscriptionToThreadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	bs, _ := io.ReadAll(r.Body)
	type postBody struct {
		User            string `json:"username"`
		CurrentlySubbed bool   `json:"currentlyNotifiedVal"`
		Thread          string `json:"curThread"`
	}
	var postData postBody
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.users_to_threads(\"username\",\"thread\",\"is_subscribed\") values('%s','%s',%t) on conflict(username,thread) do update set is_subscribed=%t;", postData.User, postData.Thread, postData.CurrentlySubbed, postData.CurrentlySubbed))
	if inserr != nil {
		activityStr := fmt.Sprintf("%s could not update sub settings for thread %s", currentUserFromSession, postData.Thread)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}

}
func UpdatePChatReactionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type postBody struct {
		ReactionToPost string `json:"reaction_str"`
		Chatid         string `json:"chat_id"`
	}
	var postData postBody
	bs, _ := io.ReadAll(r.Body)
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s');", marsherr, time.Now().In(globalvars.NyLoc).Format(time.DateTime)))
		return
	}
	var chatreactionfromDB sql.NullString
	curEmojRow := globalvars.Db.QueryRow(fmt.Sprintf("select reaction from tfldata.pchat where id='%s'", postData.Chatid))
	curEmojRow.Scan(&chatreactionfromDB)
	if !chatreactionfromDB.Valid {
		chatreactionfromDB.String = ""
	}

	if chatreactionfromDB.String != postData.ReactionToPost {
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.pchat set reaction = '%s' where id = '%s';", postData.ReactionToPost, postData.Chatid))
		if uperr != nil {
			activityStr := "updating pchat reaction"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,106),substr('%s',0,105),now());", uperr.Error(), activityStr))
		}
	} else {
		globalvars.Db.Exec(fmt.Sprintf("update tfldata.pchat set reaction = null where id = '%s';", postData.Chatid))
	}

}
func UpdateSelectedPChatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type postBody struct {
		ChatMessage    string `json:"newMessage"`
		SelectedChatId string `json:"selectedChatId"`
	}
	var postData postBody
	bs, _ := io.ReadAll(r.Body)
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	encryptedChatMessage, encrypterr := globalfunctions.Encrypt(postData.ChatMessage)
	if encrypterr != nil {
		fmt.Println(encrypterr)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity,createdon) values(substr('%s',0,105),'Err encrypting g chat message' now());", encrypterr.Error()))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.pchat set message='%s' where id='%s';", encryptedChatMessage, postData.SelectedChatId))
	if uperr != nil {
		activityStr := fmt.Sprintf("%s could not edit the Pchat message %s", currentUserFromSession, postData.SelectedChatId)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}
}
func DelThreadHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bs, _ := io.ReadAll(r.Body)
	type postBody struct {
		ThreadToDel string `json:"threadToDel"`
	}
	var postData postBody
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	s := make([]string, 0)
	s = append(s, fmt.Sprintf("delete from tfldata.gchat where thread='%s';", postData.ThreadToDel))
	s = append(s, fmt.Sprintf("delete from tfldata.threads where thread='%s';", postData.ThreadToDel))
	s = append(s, fmt.Sprintf("delete from tfldata.users_to_threads where thread='%s' or thread is null;", postData.ThreadToDel))
	delThreadDataTxn, txnerr := globalvars.Db.Begin()
	if txnerr != nil {
		fmt.Println(delThreadDataTxn)
		fmt.Println("This was a transaction error")
	}
	defer func() {
		_ = delThreadDataTxn.Rollback()
	}()
	for _, q := range s {
		_, err := delThreadDataTxn.Exec(q)

		if err != nil {
			fmt.Println(err)
		}
	}
	delThreadDataTxn.Commit()

}
func DeleteSelectedChatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type postBody struct {
		SelectedChatId string `json:"selectedChatId"`
	}
	var postData postBody
	bs, _ := io.ReadAll(r.Body)
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	_, delerr := globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.gchat where id='%s';", postData.SelectedChatId))
	if delerr != nil {
		activityStr := fmt.Sprintf("%s could not deleteSelectedChat", currentUserFromSession)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", delerr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}
}
func DeleteSelectedPChatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type postBody struct {
		SelectedChatId string `json:"selectedChatId"`
	}
	var postData postBody
	bs, _ := io.ReadAll(r.Body)
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	_, delerr := globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.pchat where id='%s';", postData.SelectedChatId))
	if delerr != nil {
		activityStr := fmt.Sprintf("%s could not deleteSelectedPChat", currentUserFromSession)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", delerr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}
}
