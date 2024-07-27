package calendarhandler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"text/template"
	globalfunctions "tfl/functions"
	globaltypes "tfl/types"
	globalvars "tfl/vars"
	"time"

	"firebase.google.com/go/messaging"
)

func CreateEventHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	bs, _ := io.ReadAll(r.Body)
	type PostBody struct {
		Startdate    string `json:"start_date"`
		Eventdetails string `json:"event_details"`
		Eventtitle   string `json:"event_title"`
		Enddate      string `json:"end_date"`
	}

	var postData PostBody
	errmarsh := json.Unmarshal(bs, &postData)
	if errmarsh != nil {
		fmt.Println(errmarsh)
	}
	if postData.Eventdetails == "" || postData.Eventtitle == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(postData.Enddate) < 1 {
		_, inserterr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.calendar(\"start_date\", \"event_owner\", \"event_details\", \"event_title\") values('%s', '%s', E'%s', E'%s');", postData.Startdate, currentUserFromSession, globalvars.Replacer.Replace(postData.Eventdetails), globalvars.Replacer.Replace(postData.Eventtitle)))
		if inserterr != nil {
			fmt.Println(inserterr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {

		_, inserterr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.calendar(\"start_date\", \"event_owner\", \"event_details\", \"event_title\", \"end_date\") values('%s', '%s', E'%s', E'%s', '%s');", postData.Startdate, currentUserFromSession, globalvars.Replacer.Replace(postData.Eventdetails), globalvars.Replacer.Replace(postData.Eventtitle), postData.Enddate))
		if inserterr != nil {
			fmt.Println(inserterr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	var chatMessageNotificationOpts globaltypes.NotificationOpts
	// You can use the below key to add onclick features to the notification
	chatMessageNotificationOpts.ExtraPayloadKey = "calendardata"
	chatMessageNotificationOpts.ExtraPayloadVal = "calendar"
	chatMessageNotificationOpts.NotificationPage = "calendar"
	chatMessageNotificationOpts.NotificationTitle = "New event on: " + postData.Startdate
	chatMessageNotificationOpts.NotificationBody = strings.ReplaceAll(postData.Eventtitle, "\\", "")

	go globalfunctions.SendNotificationToAllUsers(globalvars.Db, currentUserFromSession, globalvars.Fb_message_client, &chatMessageNotificationOpts)

}
func CreateEventCommentHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

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
		Eventcomment           string `json:"eventcomment"`
		CommentSelectedEventId int    `json:"commentSelectedEventID"`
	}
	var postData postBody
	errmarsh := json.Unmarshal(bs, &postData)
	if errmarsh != nil {
		fmt.Println(errmarsh)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s');", errmarsh, time.Now().In(globalvars.NyLoc).Format(time.DateTime)))
	}

	_, inserterr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.comments(\"comment\", \"event_id\", \"author\") values(E'%s', '%d', '%s');", globalvars.Replacer.Replace(postData.Eventcomment), postData.CommentSelectedEventId, currentUserFromSession))
	if inserterr != nil {
		activityStr := fmt.Sprintf("insert into comments table createEventComment - %s", currentUserFromSession)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserterr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}

	dataStr := "<p class='p-2'>" + postData.Eventcomment + " - " + currentUserFromSession + "</p>"

	commentTmpl, _ := template.New("com").Parse(dataStr)

	commentTmpl.Execute(w, nil)

}
func GetEventRSVPHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var status string
	row := globalvars.Db.QueryRow(fmt.Sprintf("select status from tfldata.calendar_rsvp where username='%s' and event_id='%s';", r.URL.Query().Get("username"), r.URL.Query().Get("event_id")))
	scnerr := row.Scan(&status)
	if scnerr != nil {
		if scnerr.Error() == "sql: no rows in result set" {
			w.WriteHeader(http.StatusAccepted)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}
	w.Write([]byte(status))
}
func GetRSVPNotesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var status string
	var username string
	output, outerr := globalvars.Db.Query(fmt.Sprintf("select username, status from tfldata.calendar_rsvp where event_id='%s';", r.URL.Query().Get("event_id")))

	if outerr != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer output.Close()

	for output.Next() {

		output.Scan(&username, &status)

		var fontColor string
		switch status {
		case "maybe":
			fontColor = "darkgoldenrod"
		case "no":
			fontColor = "crimson"
		case "yes":
			fontColor = "green"
		default:
			fontColor = "black"
		}
		dataStr := "<p class='px-3' style='font-size: medium" + "; color: " + fontColor + ";'>" + username + " has responded with a: " + status + "</p>"

		w.Write([]byte(dataStr))
	}

}
func UpdateRSVPForEventHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	bs, _ := io.ReadAll(r.Body)
	type postBody struct {
		Username string `json:"username"`
		Status   string `json:"status"`
		Eventid  int    `json:"event_id"`
	}
	var postData postBody
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.calendar_rsvp(\"username\",\"event_id\",\"status\") values('%s',%d,'%s') on conflict(username,event_id) do update set status='%s';", postData.Username, postData.Eventid, postData.Status, postData.Status))
	if inserr != nil {
		globalvars.Db.Exec("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(globalvars.NyLoc).Local().Format(time.DateTime))
		w.WriteHeader(http.StatusBadRequest)
	}

	var fcmToken string
	fcmrow := globalvars.Db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username = (select event_owner from tfldata.calendar where id=%d);", postData.Eventid))
	scnerr := fcmrow.Scan(&fcmToken)
	if scnerr != nil {

		if scnerr.Error() == "sql: no rows in result set" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		globalvars.Db.Exec("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", scnerr, time.Now().In(globalvars.NyLoc).Local().Format(time.DateTime))
		w.WriteHeader(http.StatusBadRequest)
		return
	} else {

		typePayload := make(map[string]string)
		typePayload["type"] = "event"
		sentRes, sendErr := globalvars.Fb_message_client.Send(context.TODO(), &messaging.Message{
			Token: fcmToken,
			Notification: &messaging.Notification{
				Title:    "Someone RSVPed to your event",
				Body:     postData.Username + " responded to your event",
				ImageURL: "/assets/icon-180x180.jpg",
			},

			Webpush: &messaging.WebpushConfig{
				Notification: &messaging.WebpushNotification{
					Title: "Someone RSVPed to your event",
					Body:  postData.Username + " responded to your event",
					Data:  typePayload,
					Image: "/assets/icon-180x180.jpg",
					Icon:  "/assets/icon-96x96.jpg",
					Actions: []*messaging.WebpushNotificationAction{
						{
							Action: typePayload["type"],
							Title:  postData.Username + " responded to your event",
							Icon:   "/assets/icon-96x96.png",
						},
						{
							Action: typePayload["type"],
							Title:  "NA",
							Icon:   "/assets/icon-96x96.png",
						},
					},
				},
			},
		})
		if sendErr != nil {
			fmt.Print(sendErr)
		}
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\") values('%s');", sentRes))
	}

}
func GetSelectedEventsComments(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var commentTmpl *template.Template

	output, err := globalvars.Db.Query(fmt.Sprintf("select comment, author from tfldata.comments where event_id='%s'::integer order by event_id desc;", r.URL.Query().Get("commentSelectedEventID")))

	var dataStr string
	if err != nil {
		log.Fatal(err)
	}

	defer output.Close()

	for output.Next() {
		var posts struct {
			Comment string
			Author  string
		}

		if err := output.Scan(&posts.Comment, &posts.Author); err != nil {
			log.Fatal(err)

		}
		dataStr = "<p class='p-2'>" + posts.Comment + " - " + posts.Author + "</p>"

		commentTmpl, err = template.New("com").Parse(dataStr)
		if err != nil {
			fmt.Println(err)
		}
		commentTmpl.Execute(w, nil)
	}

}
func GetEventsHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type EventData struct {
		Eventid      int
		Startdate    string
		Enddate      string
		Eventowner   string
		Eventdetails string
		Eventtitle   string
	}

	ourEvents := []EventData{}
	output, err := globalvars.Db.Query("select start_date, event_owner, event_details, event_title, id, end_date from tfldata.calendar;")
	if err != nil {
		fmt.Println(err)
	}
	defer output.Close()
	for output.Next() {
		var tempData EventData
		scnerr := output.Scan(&tempData.Startdate, &tempData.Eventowner, &tempData.Eventdetails, &tempData.Eventtitle, &tempData.Eventid, &tempData.Enddate)
		if scnerr != nil && tempData.Enddate != "" {
			fmt.Println(scnerr)
			w.WriteHeader(http.StatusBadRequest)
		}
		ourEvents = append(ourEvents, tempData)
	}
	data, marshErr := json.Marshal(ourEvents)

	if marshErr != nil {
		fmt.Println(marshErr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Write(data)
}
func DeleteEventHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	bs, _ := io.ReadAll(r.Body)
	type postBody struct {
		Eventid int `json:"commentSelectedEventId"`
	}
	var postData postBody
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.calendar where id=%d;", postData.Eventid))
	globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.calendar_rsvp where event_id=%d;", postData.Eventid))
	globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.comments where event_id=%d;", postData.Eventid))
}
