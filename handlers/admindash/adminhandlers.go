package adminhandler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	globalfunctions "tfl/functions"
	globalvars "tfl/vars"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"go.mongodb.org/mongo-driver/bson"
)

func AdminSendInviteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	var isAdmin bool

	rowRes := globalvars.Db.QueryRow(fmt.Sprintf("select is_admin from tfldata.users where username='%s';", currentUserFromSession))
	rowRes.Scan(&isAdmin)
	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny || !isAdmin {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var hasSent sql.NullString
	row := globalvars.Db.QueryRow(fmt.Sprintf("select hassent from tfldata.invite_sent_requests where from_admin = '%s' and to_user_email = '%s';", currentUserFromSession, r.PostFormValue("emailtosendto")))
	row.Scan(&hasSent)
	if hasSent.Valid {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<p style='color: red'>You have already sent a request to this email</p>"))
		return
	}

	var loopPassDataStr string
	if r.PostFormValue("sendviaemailval") == "yes" {
		loopPassDataStr = fmt.Sprintf("<p id='looppass'> Use the password: %s to signup!</p>", globalvars.OrgId)
	} else {
		loopPassDataStr = ""
	}
	var reachThemDataStr string
	if len(r.PostFormValue("emailtoreachout")) > 0 || len(r.PostFormValue("phoneadmin")) > 0 {
		reachThemDataStr = "You can reach out with questions at: " + r.PostFormValue("emailtoreachout") + " " + r.PostFormValue("phoneadmin")
	}
	_, sendemailerr := globalvars.SesClient.SendEmail(context.TODO(), &ses.SendEmailInput{
		Source: aws.String("tfl-response@the-family-loop.com"),
		Destination: &types.Destination{
			ToAddresses: aws.ToStringSlice(aws.StringSlice([]string{r.PostFormValue("emailtosendto")})),
		},
		Message: &types.Message{
			Body: &types.Body{
				Html: &types.Content{
					Data: aws.String(fmt.Sprintf("<div style='background: radial-gradient(white, #4f4f4f7a); border-radius: 20px 20px 20px 20px; padding: 15px;'> <div style='display: block;'> <div style='width: 100&percnt;'><img src='https://antons.the-family-loop.com/assets/TFLBanner.png' style='width: 23dvw'> <h1 style='text-align: center'>You've been invited!</h1> </div> <div style='text-align: center;'> <div style='display: flex; justify-content: center;'> <p id='usertosend' style='text-align: center; margin: auto'>Hi&nbsp;%s, you are invited to join a family loop.</p> </div> <div style='display: block; text-align: center;'> <p id='someone'>Someone has invited you to their loop!&nbsp; %s </p> </div> <p id='reachthemat'>%s</p> <p>Please follow the <a href='%s'>link</a> to sign up</p> </div> </div> </div>", r.PostFormValue("firstnamesendto"), loopPassDataStr, reachThemDataStr, r.PostFormValue("loopurl"))),
				},
			},
			Subject: &types.Content{
				Data: aws.String("You got an invite!"),
			},
		},
	})
	if sendemailerr == nil {
		_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.invite_sent_requests(from_admin, to_user_email, to_user_first_name, hassent) values('%s', '%s', '%s', true);", currentUserFromSession, r.PostFormValue("emailtosendto"), r.PostFormValue("firstnamesendto")))
		if inserr != nil {
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s', 0, 105), 'Err inserting into invite_sent_requests', now());", inserr.Error()))
		}
	}
}

func AdminGetListOfUsersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	var isAdmin bool

	rowRes := globalvars.Db.QueryRow(fmt.Sprintf("select is_admin from tfldata.users where username='%s';", currentUserFromSession))

	rowRes.Scan(&isAdmin)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny || !isAdmin {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type dataStruct struct {
		username string
		email    string
	}

	output, outerr := globalvars.Db.Query(fmt.Sprintf("select username, email from tfldata.users order by id %s;", r.URL.Query().Get("sortByLastPass")))
	if outerr != nil {
		activityStr := "Gathering listofusers for admin dash"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", outerr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}
	defer output.Close()

	var curDataObj dataStruct
	for output.Next() {
		scnErr := output.Scan(&curDataObj.username, &curDataObj.email)
		if scnErr != nil {
			activityStr := "Scan err on listofusers admin dash"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", outerr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		}
		w.Write([]byte(fmt.Sprintf("<tr><td style='padding-bottom: 0&percnt;'>%s</td><td style='padding-bottom: 0&percnt;'>%s</td><td style='padding-bottom: 0&percnt;;'><p onclick='openDeleteModal(`%s`)' style='color: white; border-radius: 15px / 15px; box-shadow: 1px 1px 6px black; text-align: center; width: 20&percnt;; background: linear-gradient(130deg, #9d9d9d, #f94242f5); margin: auto; margin-bottom: 10&percnt;;'>X</p></td></tr>", curDataObj.username, curDataObj.email, curDataObj.username)))

	}

}
func AdminGetSubPackageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	var isAdmin bool

	rowRes := globalvars.Db.QueryRow(fmt.Sprintf("select is_admin from tfldata.users where username='%s';", currentUserFromSession))

	rowRes.Scan(&isAdmin)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny || !isAdmin {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var countOfUsers int
	var totalUsers int
	rowOfCount := globalvars.Db.QueryRow("select count(*) from tfldata.users;")
	rowOfCount.Scan(&countOfUsers)
	switch globalvars.SubLevel {
	case "supreme":
		totalUsers = 50
	case "extra":
		totalUsers = 20
	case "standard":
		totalUsers = 8
	}

	w.Write([]byte(globalvars.SubLevel + " - " + "Current user count: " + fmt.Sprint(countOfUsers) + "/" + fmt.Sprint(totalUsers)))
}
func AdminGetAllTCHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	var isAdmin bool

	rowRes := globalvars.Db.QueryRow(fmt.Sprintf("select is_admin from tfldata.users where username='%s';", currentUserFromSession))

	rowRes.Scan(&isAdmin)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny || !isAdmin {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type listOfTC struct {
		tcname      string
		createdon   string
		availableOn string
	}

	output, _ := globalvars.Db.Query(fmt.Sprintf("select tcname, createdon, available_on from tfldata.timecapsule where available_on %s now() order by available_on asc;", r.URL.Query().Get("pastorpresent")))

	defer output.Close()

	iter := 0

	for output.Next() {
		bgColor := "white"
		var myTcOut listOfTC
		if iter%2 == 0 {
			bgColor = "white"
		} else {
			bgColor = "#efefefe6"
		}

		output.Scan(&myTcOut.tcname, &myTcOut.createdon, &myTcOut.availableOn)

		w.Write([]byte(fmt.Sprintf("<tr><td style='background-color: %s'>%s</td><td style='background-color: %s'>%s</td><td style='background-color: %s'>%s</td></tr>", bgColor, myTcOut.tcname, bgColor, strings.Split(myTcOut.createdon, "T")[0], bgColor, strings.Split(myTcOut.availableOn, "T")[0])))
		iter++
	}
}
func AdminDeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	var isAdmin bool

	rowRes := globalvars.Db.QueryRow(fmt.Sprintf("select is_admin from tfldata.users where username='%s';", currentUserFromSession))

	rowRes.Scan(&isAdmin)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny || !isAdmin {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	bs, _ := io.ReadAll(r.Body)
	type postBody struct {
		DeleteAllOpt            string `json:"deleteAllData"`
		DeleteChatsOpt          string `json:"deleteDataChatsOptions,omitempty"`
		DeletePostsOpt          string `json:"deleteDataPostsOptions,omitempty"`
		DeleteGameScoresOpt     string `json:"deleteDataGameScoresOptions,omitempty"`
		DeleteCalendarEventsOpt string `json:"deleteDataCalendarEventsOptions,omitempty"`
		SelectedUser            string `json:"user"`
	}
	var postData postBody
	marshErr := json.Unmarshal(bs, &postData)

	if marshErr != nil {
		fmt.Println(marshErr)
	}

	var tcFileToDeleteCreatedon string
	var tcFileToDeleteTcname string
	var pfpName string

	postfileout, postfileouterr := globalvars.Db.Query(fmt.Sprintf("select file_name,file_type from tfldata.postfiles where post_files_key in (select post_files_key from tfldata.posts where author='%s');", postData.SelectedUser))
	if postfileouterr != nil {
		fmt.Println(postfileouterr)
	}
	defer postfileout.Close()

	tcrow := globalvars.Db.QueryRow(fmt.Sprintf("select createdon,tcname from tfldata.timecapsule where username='%s';", postData.SelectedUser))

	scner := tcrow.Scan(&tcFileToDeleteCreatedon, &tcFileToDeleteTcname)
	if scner != nil {
		fmt.Println(scner)
	}

	pfprow := globalvars.Db.QueryRow(fmt.Sprintf("select pfp_name from tfldata.users where username='%s';", postData.SelectedUser))
	pfpscnerr := pfprow.Scan(&pfpName)
	if pfpscnerr != nil {
		fmt.Println(pfpscnerr)
	}
	tcFileToDeleteTcname = strings.Split(tcFileToDeleteCreatedon, "T")[0] + "_" + tcFileToDeleteTcname + "_capsule_" + postData.SelectedUser + ".zip"

	var mongoRecords []bson.M

	cursor, findErr := globalvars.Leaderboardcoll.Find(context.TODO(), bson.D{{Key: "username", Value: postData.SelectedUser}, {Key: "org_id", Value: globalvars.OrgId}})
	if findErr != nil {
		fmt.Println(findErr)
	}

	marsherr := cursor.All(context.TODO(), &mongoRecords)
	if marsherr != nil {
		fmt.Println("here: " + marsherr.Error())
	}

	for _, val := range mongoRecords {
		_, delErr := globalvars.Leaderboardcoll.DeleteOne(context.TODO(), bson.D{{Key: "_id", Value: val["_id"]}})
		if delErr != nil {
			fmt.Println("err: " + delErr.Error())
		}
	}
	go globalfunctions.DeleteFileFromS3(tcFileToDeleteTcname, "timecapsules/")
	globalfunctions.DeleteFileFromS3(pfpName, "pfp/")
	if postData.DeleteAllOpt == "yes" {
		for postfileout.Next() {
			var fileName string
			var fileType string
			scnerr := postfileout.Scan(&fileName, &fileType)
			if scnerr != nil {
				fmt.Println(scnerr)
			}
			if strings.Contains(fileType, "image") {
				globalfunctions.DeleteFileFromS3(fileName, "posts/images/")
			} else {
				go globalfunctions.DeleteFileFromS3(fileName, "posts/videos/")
			}
		}
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.calendar where event_owner='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.comments where author='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.calendar_rsvp where username='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.gchat where thread in (select thread from tfldata.threads where threadauthor = '%s');", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.gchat where author='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.threads where threadauthor='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.users_to_threads where username='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.stack_leaderboard where username='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.ss_leaderboard where username='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.catchitleaderboard where username='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.timecapsule where username='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.posts where author='%s';", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.postfiles where post_files_key in (select post_files_key from tfldata.posts where author='%s');", postData.SelectedUser))
		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.users where username='%s';", postData.SelectedUser))

	} else {
		if postData.DeleteChatsOpt == "on" {
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.gchat where thread in (select thread from tfldata.threads where threadauthor = '%s');", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.gchat where author='%s';", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.threads where threadauthor='%s';", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.users_to_threads where username='%s';", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.users where username='%s';", postData.SelectedUser))
		}
		if postData.DeletePostsOpt == "on" {
			for postfileout.Next() {
				var fileName string
				var fileType string
				scnerr := postfileout.Scan(&fileName, &fileType)
				if scnerr != nil {
					fmt.Println(scnerr)
				}
				if strings.Contains(fileType, "image") {
					globalfunctions.DeleteFileFromS3(fileName, "posts/images/")
				} else {
					go globalfunctions.DeleteFileFromS3(fileName, "posts/videos/")
				}
			}
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.posts where author='%s';", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.postfiles where post_files_key in (select post_files_key from tfldata.posts where author='%s');", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.users where username='%s';", postData.SelectedUser))
		}
		if postData.DeleteGameScoresOpt == "on" {
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.stack_leaderboard where username='%s';", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.ss_leaderboard where username='%s';", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.catchitleaderboard where username='%s';", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.users where username='%s';", postData.SelectedUser))
		}
		if postData.DeleteCalendarEventsOpt == "on" {
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.calendar where event_owner='%s';", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.comments where author='%s';", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.calendar_rsvp where username='%s';", postData.SelectedUser))
			globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.users where username='%s';", postData.SelectedUser))
		}

	}

}
