package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	globalfunctions "tfl/functions"
	pages "tfl/handlers"
	chathandler "tfl/handlers/chats"
	postshandler "tfl/handlers/posts"
	globaltypes "tfl/types"
	globalvars "tfl/vars"
	"time"

	"firebase.google.com/go/messaging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	_ "image/png"

	"github.com/google/go-github/github"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type seshStruct struct {
	Username         string
	Pfpname          sql.NullString
	BGtheme          string
	GchatOrderOpt    bool
	CFDomain         string
	Isadmin          bool
	Fcmkey           sql.NullString
	LastViewedPChat  sql.NullString
	LastViewedThread sql.NullString
}

func main() {

	// favicon

	globalfunctions.InitalizeAll()
	faviconHandler := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "assets/favicon.ico")
	}
	http.HandleFunc("/favicon.ico", faviconHandler)
	serviceWorkerHandler := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "firebase-messaging-sw.js")
	}
	http.HandleFunc("/firebase-messaging-sw.js", serviceWorkerHandler)
	// Connect to database

	defer globalvars.Db.Close()
	mongoDb, mongoerr := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb+srv://tfl-user:"+globalvars.MongoDBPass+"@tfl-leaderboard.dg95d1f.mongodb.net/?retryWrites=true&w=majority"))

	if mongoerr != nil {
		activityStr := "mongo Initalize error"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105),substr('%s',0,105),now());", mongoerr.Error(), activityStr))
		return
	}
	defer mongoDb.Disconnect(context.TODO())
	coll := mongoDb.Database("tfl-database").Collection("leaderboards")

	awscfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(globalvars.Awskey, globalvars.Awskeysecret, "")),
	)
	sqsClient := sqs.NewFromConfig(awscfg)

	if err != nil {
		log.Fatal(err)
		os.Exit(4)
	}

	//globalvars.S3Client = s3.NewFromConfig(awscfg)

	updateFCMTokenHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id = null where username = '%s';", r.URL.Query().Get("username")))
	}
	subscriptionHandler := func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		bs, _ := io.ReadAll(r.Body)

		type postBody struct {
			Fcmtoken string `json:"fcm_token"`
		}
		var postData postBody
		psdmae := json.Unmarshal(bs, &postData)
		if psdmae != nil {
			fmt.Print(psdmae)
		}
		seshToken, seshErr := r.Cookie("session_id")
		if seshErr != nil {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		seshVal := strings.Split(seshToken.Value, "session_id=")[0]
		_, inserr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id='%s' where session_token='%s';", postData.Fcmtoken, seshVal))
		if inserr != nil {
			activityStr := "attempt update fcm token where seshtoken is value subHandler"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		}

	}

	signUpHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/form-data")

		if r.PostFormValue("passwordsignup") != r.PostFormValue("confirmpasswordsignup") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.PostFormValue("orgidinput") != globalvars.OrgId {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var countOfUsers int
		userRowCount := globalvars.Db.QueryRow("select count(*) from tfldata.users;")
		userRowCount.Scan(&countOfUsers)
		switch globalvars.SubLevel {
		case "supreme":
			if countOfUsers > 49 {
				w.WriteHeader(http.StatusFailedDependency)
				return
			}
		case "extra":
			if countOfUsers > 19 {
				w.WriteHeader(http.StatusFailedDependency)
				return
			}
		case "standard":
			if countOfUsers > 7 {
				w.WriteHeader(http.StatusFailedDependency)
				return
			}
		}

		upload, filename, errfile := r.FormFile("pfpformfile")
		if errfile != nil {
			activityStr := "uploading pfp during sign up"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errfile.Error(), time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fn := globalfunctions.UploadPfpToS3(upload, filename.Filename, r, "pfpformfile")
		bs := []byte(r.PostFormValue("passwordsignup"))

		bytesOfPass, err := bcrypt.GenerateFromPassword(bs, len(bs))
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		_, errinsert := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.users(\"username\", \"password\", \"pfp_name\", \"email\", \"gchat_bg_theme\", \"gchat_order_option\", \"cf_domain_name\", \"orgid\", \"is_admin\", \"mytz\") values('%s', '%s', '%s', '%s', '%s', %t, '%s', '%s', %t, '%s');", strings.ToLower(r.PostFormValue("usernamesignup")), bytesOfPass, fn, strings.ToLower(r.PostFormValue("emailsignup")), "background: linear-gradient(142deg, #00009f, #3dc9ff 26%)", true, globalvars.Cfdistro, globalvars.OrgId, false, r.PostFormValue("mytz")))

		if errinsert != nil {
			activityStr := "err inserting into users table on sign up"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errinsert, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, errutterr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.users_to_threads(\"username\", \"thread\", \"is_subscribed\") values('%s', 'posts', true),('%s', 'calendar',true), ('%s', 'main', true);", strings.ToLower(r.PostFormValue("usernamesignup")), strings.ToLower(r.PostFormValue("usernamesignup")), strings.ToLower(r.PostFormValue("usernamesignup"))))
		if errutterr != nil {
			activityStr := fmt.Sprintf("user %s will not be subscribed to new posts as of now", strings.ToLower(r.PostFormValue("usernamesignup")))
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,106), substr('%s',0,106), now());", errutterr.Error(), activityStr))
		}
		type memberChildrenObj struct {
			LoginEmail string `json:"loginEmail"`
		}
		type memberObj struct {
			MemChild memberChildrenObj `json:"member"`
		}
		postReqBody := memberObj{
			MemChild: memberChildrenObj{
				LoginEmail: strings.ToLower(r.PostFormValue("emailsignup")),
			},
		}
		jsonMarshed, errMarsh := json.Marshal(&postReqBody)
		if errMarsh != nil {
			activityStr := "error marshing Json body for members sign up"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", errMarsh.Error(), activityStr))
			return
		}

		req, reqerr := http.NewRequest("POST", "https://www.wixapis.com/members/v1/members", bytes.NewReader(jsonMarshed))
		if reqerr != nil {
			activityStr := "error posting to wix members sign up"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", reqerr.Error(), activityStr))
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", globalvars.Wixapikey)
		req.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
		req.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
		client := &http.Client{}
		createresp, clientdoerr := client.Do(req)
		if clientdoerr != nil {
			activityStr := "client error for wix members sign up"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", clientdoerr.Error(), activityStr))
			return
		}

		defer client.CloseIdleConnections()

		type redirectObj struct {
			Url string `json:"url"`
		}

		type sendReset struct {
			Email       string `json:"email"`
			Lang        string `json:"language"`
			RedirectObj redirectObj
		}
		postBody := sendReset{
			Email: strings.ToLower(r.PostFormValue("emailsignup")),
			Lang:  "en",
			RedirectObj: redirectObj{
				Url: "https://the-family-loop.com",
			},
		}
		sendjsonMarshed, senderrMarsh := json.Marshal(&postBody)
		if senderrMarsh != nil {
			activityStr := "error marshing Json body for members sign up"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", senderrMarsh.Error(), activityStr))
			return
		}
		setpassreq, setpassreqerr := http.NewRequest("POST", "https://www.wixapis.com/_api/iam/recovery/v1/send-email", bytes.NewReader(sendjsonMarshed))
		if setpassreqerr != nil {
			activityStr := "error sending set pass wix members sign up"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", setpassreqerr.Error(), activityStr))
			return
		}
		setpassreq.Header.Set("Content-Type", "application/json")
		setpassreq.Header.Set("Authorization", globalvars.Wixapikey)
		setpassreq.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
		setpassreq.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
		sendclient := &http.Client{}
		_, sendclientdoerr := sendclient.Do(setpassreq)
		if sendclientdoerr != nil {
			activityStr := "client error for wix members sign up"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", sendclientdoerr.Error(), activityStr))
			return
		}
		defer sendclient.CloseIdleConnections()

		type memberobj struct {
			Id string `json:"id"`
		}
		type memberResponseObj struct {
			Memberstruct memberobj `json:"member"`
		}
		var responseData memberResponseObj
		bs, bserr := io.ReadAll(createresp.Body)

		if bserr != nil {
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,105), substr('%s',0,105), now());", bserr.Error(), "creating bs for wix create site member response"))
		}

		unmarsherr := json.Unmarshal(bs, &responseData)
		if unmarsherr != nil {
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,105), substr('%s',0,105), now());", unmarsherr.Error(), "unmarshal wix create site member response"))
		}
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set wix_member_id = '%s' where username = '%s';", responseData.Memberstruct.Id, strings.ToLower(r.PostFormValue("usernamesignup"))))
		if uperr != nil {
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,106), substr('%s',0,105), now());", uperr.Error(), "Err updating users table with wix id"))
		}

		defer createresp.Body.Close()
	}

	loginHandler := func(w http.ResponseWriter, r *http.Request) {

		userStr := strings.ToLower(r.PostFormValue("usernamelogin"))

		var password string
		var isAdmin bool
		var emailIn string
		passScan := globalvars.Db.QueryRow(fmt.Sprintf("select is_admin, password, email from tfldata.users where username='%s' or email='%s';", userStr, userStr))

		scnerr := passScan.Scan(&isAdmin, &password, &emailIn)

		if isAdmin {

			if password == r.PostFormValue("passwordlogin") {

				w.Header().Set("HX-Trigger", "changeAdminPassword")
				globalfunctions.SetLoginCookie(w, globalvars.Db, userStr, r.PostFormValue("mytz"))
				globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set last_sign_on='%s' where username='%s';", time.Now().In(globalvars.NyLoc).Format(time.DateTime), userStr))

				globalfunctions.GenerateLoginJWT(userStr, w, globalvars.JwtSignKey)

			} else {
				err := bcrypt.CompareHashAndPassword([]byte(password), []byte(r.PostFormValue("passwordlogin")))

				if err != nil {

					globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values(substr('%s',0,105), '%s');", err, time.Now().In(globalvars.NyLoc).Format(time.DateTime)))
					w.WriteHeader(http.StatusUnauthorized)
					return
				} else {

					globalfunctions.GenerateLoginJWT(userStr, w, globalvars.JwtSignKey)
					globalfunctions.SetLoginCookie(w, globalvars.Db, userStr, r.PostFormValue("mytz"))
					globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set last_sign_on='%s' where username='%s';", time.Now().In(globalvars.NyLoc).Format(time.DateTime), userStr))

					w.Header().Set("HX-Refresh", "true")
				}
			}
		} else {
			if scnerr != nil {
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('this was the scan error %s with globalvars.Dbpassword *** and form user is %s');", scnerr, userStr))
				fmt.Print(scnerr)
			}
			err := bcrypt.CompareHashAndPassword([]byte(password), []byte(r.PostFormValue("passwordlogin")))

			if err != nil {
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values(substr('%s',0,105), '%s');", err, time.Now().In(globalvars.NyLoc).Format(time.DateTime)))
				w.WriteHeader(http.StatusUnauthorized)
				return
			} else {

				globalfunctions.GenerateLoginJWT(userStr, w, globalvars.JwtSignKey)
				globalfunctions.SetLoginCookie(w, globalvars.Db, userStr, r.PostFormValue("mytz"))
				globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set last_sign_on='%s' where username='%s';", time.Now().In(globalvars.NyLoc).Format(time.DateTime), userStr))

				w.Header().Set("HX-Refresh", "true")
			}
		}
	}
	updateAdminPassHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		bs, _ := io.ReadAll(r.Body)
		type postBody struct {
			Username            string `json:"username"`
			Newadminpass        string `json:"newadminpassinput"`
			Confirmnewadminpass string `json:"confirmnewadminpassinput"`
		}
		var postData postBody
		json.Unmarshal(bs, &postData)

		if postData.Newadminpass != postData.Confirmnewadminpass {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		newadminpassbs := []byte(postData.Newadminpass)

		newAdminbytesOfPass, err := bcrypt.GenerateFromPassword(newadminpassbs, len(newadminpassbs))
		if err != nil {
			fmt.Println(err)
			activityStr := "updating admin pass"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values (substr('%s',0,106), substr('%s',0,106), now());", err.Error(), activityStr))
			return
		}
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set password='%s' where username='%s';", newAdminbytesOfPass, postData.Username))
		if uperr != nil {
			fmt.Println(uperr)
			activityStr := "updating admin pass"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values (substr('%s',0,106), substr('%s',0,106), now());", uperr.Error(), activityStr))
			return
		}

		w.Header().Set("HX-Refresh", "true")
	}
	getResetPasswordCodeHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		emailInput := r.Header.Get("HX-Prompt")

		var userEmail string
		var userName string
		var lastPassReset time.Time
		row := globalvars.Db.QueryRow(fmt.Sprintf("select email, username, last_pass_reset from tfldata.users where email='%s' and (last_pass_reset < now() - interval '32 hours' or last_pass_reset is null);", emailInput))
		scnerr := row.Scan(&userEmail, &userName, &lastPassReset)
		if scnerr != nil {

			if strings.Contains(scnerr.Error(), "no rows in result") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		var table = [...]byte{'1', '2', '3', '4', '5', '6', '7', '8', '9', '0'}
		b := make([]byte, 6)
		n, err := io.ReadAtLeast(rand.Reader, b, 6)
		if n != 6 {
			panic(err)
		}
		for i := 0; i < len(b); i++ {
			b[i] = table[int(b[i])%len(table)]
		}

		_, senderr := sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
			QueueUrl:    aws.String("https://sqs.us-east-1.amazonaws.com/529465713677/sendresetcode"),
			MessageBody: aws.String(fmt.Sprintf("{\"user\": \"%s\", \"resetcode\": \"%s\", \"email\": \"%s\", \"username\": \"%s\"}", emailInput, string(b), userEmail, userName)),
		})
		if senderr != nil {
			fmt.Println(senderr)
		}

		w.Write([]byte(fmt.Sprintf("{\"user\":\"%s\", \"code\": \"%s\", \"email\": \"%s\"}", userName, string(b), userEmail)))
	}
	resetPasswordHandler := func(w http.ResponseWriter, r *http.Request) {

		newPass := r.PostFormValue("resetnewpassinput")
		verifyCode := r.PostFormValue("resetCodeInput")
		emailInput := r.PostFormValue("email")
		userInput := r.PostFormValue("user")
		userInStr := userInput
		type messageBody struct {
			Userinput string `json:"user"`
			ResetCode string `json:"resetcode"`
			Useremail string `json:"email"`
			Username  string `json:"username"`
		}
		var messageData messageBody

		out, geterr := sqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
			QueueUrl: aws.String("https://sqs.us-east-1.amazonaws.com/529465713677/sendresetcode"),
			MessageAttributeNames: []string{
				userInStr,
				"secondname",
			},
		})

		if geterr != nil {
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),'reset password getmessage', now());", geterr))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var deleteReceipt string
		for _, val := range out.Messages {

			marsherr := json.Unmarshal([]byte(*val.Body), &messageData)
			if marsherr != nil {
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),'reset password marshaler', now());", marsherr))
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			deleteReceipt = *val.ReceiptHandle
		}

		for emailInput != messageData.Useremail || userInput != messageData.Username {

			out, _ := sqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
				QueueUrl: aws.String("https://sqs.us-east-1.amazonaws.com/529465713677/sendresetcode"),
				MessageAttributeNames: []string{
					userInStr,
					"secondname",
				},
			})
			for _, val := range out.Messages {

				marsherr := json.Unmarshal([]byte(*val.Body), &messageData)
				if marsherr != nil {
					fmt.Print(marsherr)
				}
				deleteReceipt = *val.ReceiptHandle

			}

		}

		if messageData.ResetCode == verifyCode {

			newpassbs := []byte(newPass)

			newPassbytesOfPass, err := bcrypt.GenerateFromPassword(newpassbs, len(newpassbs))
			if err != nil {
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),'reset password generate issue', now());", err))
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set password='%s', last_pass_reset=now() where username='%s' or email='%s';", newPassbytesOfPass, messageData.Username, messageData.Useremail))
			if uperr != nil {
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),'reset password update users table', now());", uperr))
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			_, delErr := sqsClient.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
				QueueUrl:      aws.String("https://sqs.us-east-1.amazonaws.com/529465713677/sendresetcode"),
				ReceiptHandle: aws.String(deleteReceipt),
			})
			if delErr != nil {
				fmt.Print("del err: " + delErr.Error())
			}

		} else {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	createEventCommentHandler := func(w http.ResponseWriter, r *http.Request) {
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
	getSelectedEventsComments := func(w http.ResponseWriter, r *http.Request) {
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

	getEventsHandler := func(w http.ResponseWriter, r *http.Request) {

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

	createEventHandler := func(w http.ResponseWriter, r *http.Request) {
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
	deleteEventHandler := func(w http.ResponseWriter, r *http.Request) {
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
	updateRSVPForEventHandler := func(w http.ResponseWriter, r *http.Request) {
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
	getEventRSVPHandler := func(w http.ResponseWriter, r *http.Request) {

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
	getRSVPNotesHandler := func(w http.ResponseWriter, r *http.Request) {
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

	getSubscribedHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)

		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var fcmRegToken string
		fcmRegRow := globalvars.Db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username='%s';", currentUserFromSession))
		scnerr := fcmRegRow.Scan(&fcmRegToken)

		if scnerr != nil {
			w.WriteHeader(http.StatusAccepted)
			return
			// globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", scnerr, time.Now().In(globalvars.NyLoc).Local().Format(time.DateTime)))
		}
		w.WriteHeader(http.StatusOK)
	}
	getSessionDataHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)

		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			if h == "onRevealedYouHaveNotPurchasedRegularUserSubscriptionPlan" {
				w.Header().Set("HX-Trigger", h)
				w.WriteHeader(http.StatusFound)
				type respBody struct {
					Orgid    string `json:"orgid"`
					Username string `json:"username"`
				}
				var resp respBody
				resp.Orgid = globalvars.OrgId
				resp.Username = currentUserFromSession
				bs, _ := json.Marshal(&resp)
				w.Write(bs)
				return
			}
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var ourSeshStruct seshStruct

		row := globalvars.Db.QueryRow(fmt.Sprintf("select username, gchat_bg_theme, gchat_order_option, is_admin, pfp_name, fcm_registration_id, last_viewed_pchat, last_viewed_gchat from tfldata.users where username='%s';", currentUserFromSession))
		scerr := row.Scan(&ourSeshStruct.Username, &ourSeshStruct.BGtheme, &ourSeshStruct.GchatOrderOpt, &ourSeshStruct.Isadmin, &ourSeshStruct.Pfpname, &ourSeshStruct.Fcmkey, &ourSeshStruct.LastViewedPChat, &ourSeshStruct.LastViewedThread)
		if scerr != nil {
			fmt.Println(scerr)
		}

		if !ourSeshStruct.Fcmkey.Valid {
			ourSeshStruct.Fcmkey.String = ""
		}
		if !ourSeshStruct.Pfpname.Valid {
			ourSeshStruct.Pfpname.String = ""
		}
		if !ourSeshStruct.LastViewedPChat.Valid {
			ourSeshStruct.LastViewedPChat.String = ""
		}
		if !ourSeshStruct.LastViewedThread.Valid {
			ourSeshStruct.LastViewedThread.String = ""
		}

		ourSeshStruct.CFDomain = globalvars.Cfdistro

		data, err := json.Marshal(&ourSeshStruct)
		if err != nil {
			fmt.Println(err)
		}

		w.Write(data)
	}

	updatePfpHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/form-data")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		upload, filename, _ := r.FormFile("changepfp")

		username := r.PostFormValue("usernameinput")

		fn := globalfunctions.UploadPfpToS3(upload, filename.Filename, r, "changepfp")
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set pfp_name='%s' where username='%s';", fn, username))
		if uperr != nil {
			activityStr := fmt.Sprintf("update table users set pfp_name failed for user %s", currentUserFromSession)
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	updateChatThemeHandler := func(w http.ResponseWriter, r *http.Request) {
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
			Theme    string `json:"theme"`
			Username string `json:"username"`
		}
		var postData postBody
		bs, _ := io.ReadAll(r.Body)
		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			fmt.Println(marsherr)
		}
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set gchat_bg_theme='%s' where username='%s';", postData.Theme, postData.Username))
		if uperr != nil {
			activityStr := fmt.Sprintf("updateChatTheme failed for user %s", currentUserFromSession)
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		}
	}

	getGHIssuesComments := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		type commentResp struct {
			Created string `json:"created_at"`
			Body    string `json:"body"`
			Author  struct {
				Username string `json:"login"`
			} `json:"user"`
		}
		type commentList []commentResp
		req, reqerr := http.NewRequest("GET", r.URL.Query().Get("comurl")+"?per_page=100", nil)
		if reqerr != nil {
			fmt.Println(reqerr)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+globalvars.Ghusercommentkey)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		bs, readerr := io.ReadAll(resp.Body)
		if readerr != nil {
			fmt.Println(readerr)
			return
		}
		var listofcomments commentList
		var commentListFull commentList
		marshbodyerr := json.Unmarshal(bs, &listofcomments)

		if marshbodyerr != nil {
			fmt.Println(marshbodyerr)
			return
		}

		commentListFull = append(listofcomments, commentListFull...)
		marsheddata, marsheddataerr := json.Marshal(commentListFull)
		if marsheddataerr != nil {
			fmt.Println("final marsh err")
			return
		}
		w.Write(marsheddata)

	}
	createGHIssueCommentHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		type formValues struct {
			Comment string `json:"bugissuecomment"`
			ComURL  string `json:"comurl"`
		}
		type dataBody struct {
			Body string `json:"body"`
		}
		bs, readerr := io.ReadAll(r.Body)
		if readerr != nil {
			fmt.Println(readerr)
		}
		var formValuesData formValues
		marsherr := json.Unmarshal(bs, &formValuesData)
		if marsherr != nil {
			fmt.Println(marsherr)
		}
		var sendBody dataBody
		sendBody.Body = formValuesData.Comment
		sendToPost, sendToPostErr := json.Marshal(sendBody)
		if sendToPostErr != nil {
			fmt.Println(sendToPostErr)
			return
		}

		req, reqerr := http.NewRequest("POST", formValuesData.ComURL+"?per_page=100", bytes.NewReader(sendToPost))
		if reqerr != nil {
			fmt.Println(reqerr)
			return
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("Authorization", "Bearer "+globalvars.Ghusercommentkey)

		client := &http.Client{}
		_, err := client.Do(req)
		if err != nil {

			fmt.Println(err)
			return
		}

		defer client.CloseIdleConnections()
	}
	getCustomerSupportIssuesHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		type issueResp struct {
			CommentsLink string `json:"comments_url"`
			Issuetitle   string `json:"title"`
			CreatedAt    string `json:"created_at"`
			UpdatedAt    string `json:"updated_at"`
			ClosedAt     string `json:"closed_at,omitempty"`
			Body         string `json:"body"`
		}
		type issueList []issueResp

		req, reqerr := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/zanton173/the-family-loop/issues?labels=%s&per_page=100&sort=created&direction=desc&state=%s", currentUserFromSession+"_"+strings.Split(globalvars.OrgId, "_")[0]+strings.Split(globalvars.OrgId, "_")[1][:3], r.URL.Query().Get("state")), nil)
		if reqerr != nil {
			fmt.Println(reqerr)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+globalvars.Ghusercommentkey)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		bs, readerr := io.ReadAll(resp.Body)
		if readerr != nil {
			fmt.Println(readerr)
			return
		}

		var listOfResp issueList
		var listOfAll issueList

		marshbodyerr := json.Unmarshal(bs, &listOfResp)

		if marshbodyerr != nil {
			fmt.Println("marsh at issue list:" + marshbodyerr.Error())
			return
		}

		listOfAll = append(listOfAll, listOfResp...)

		marsheddata, marsheddataerr := json.Marshal(listOfAll)
		if marsheddataerr != nil {
			fmt.Println("final marsh err")
			return
		}

		w.Write(marsheddata)

	}
	createIssueHandler := func(w http.ResponseWriter, r *http.Request) {
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
		type PostBody struct {
			Issuetitle string   `json:"bugissue"`
			Descdetail []string `json:"bugerrmessages"`
			Label      string   `json:"label"`
		}

		var postData PostBody
		var issueLabel []string

		errmarsh := json.Unmarshal(bs, &postData)
		if errmarsh != nil {
			fmt.Println(errmarsh)
		}
		if postData.Label == "enhancement" {
			issueLabel = []string{"enhancement", currentUserFromSession + "_" + strings.Split(globalvars.OrgId, "_")[0] + strings.Split(globalvars.OrgId, "_")[1][:3]}
		} else if postData.Label == "bug" {
			issueLabel = []string{"bug", currentUserFromSession + "_" + strings.Split(globalvars.OrgId, "_")[0] + strings.Split(globalvars.OrgId, "_")[1][:3]}
		}

		bodyText := fmt.Sprintf("%s on %s page - %s. Orgid: %s", strings.ReplaceAll(postData.Descdetail[1], "-", " "), postData.Descdetail[0], currentUserFromSession, strings.Split(globalvars.OrgId, "_")[0]+strings.Split(globalvars.OrgId, "_")[1][:3])
		issueJson := github.IssueRequest{
			Title: &postData.Issuetitle,
			Body:  &bodyText,
		}

		jsonMarshed, errMarsh := json.Marshal(issueJson)
		if errMarsh != nil {
			fmt.Println(errMarsh)
		}

		req, reqerr := http.NewRequest("POST", "https://api.github.com/repos/zanton173/the-family-loop/issues", bytes.NewReader(jsonMarshed))
		if reqerr != nil {
			fmt.Println(reqerr)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("Authorization", "Bearer "+globalvars.Ghusercommentkey)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
		}
		defer resp.Body.Close()
		type respBody struct {
			Number int `json:"number"`
		}

		var respData respBody
		bsread, _ := io.ReadAll(resp.Body)

		marshtoreaderr := json.Unmarshal(bsread, &respData)
		if marshtoreaderr != nil {
			fmt.Println(marshtoreaderr)
			return
		}

		issueWithLabelJson := github.IssueRequest{
			Labels: &issueLabel,
		}

		jsonMarshedWithLabel, errLabelMarsh := json.Marshal(issueWithLabelJson)
		if errLabelMarsh != nil {
			fmt.Println("Marshing here")
			fmt.Println(errLabelMarsh)
			return
		}

		updatereq, updatereqerr := http.NewRequest("PATCH", "https://api.github.com/repos/zanton173/the-family-loop/issues/"+fmt.Sprint(respData.Number), bytes.NewReader(jsonMarshedWithLabel))
		if updatereqerr != nil {
			fmt.Println(updatereqerr)
			return
		}
		updatereq.Header.Set("Accept", "application/vnd.github+json")
		updatereq.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		updatereq.Header.Set("Authorization", "token "+globalvars.Ghissuetoken)

		_, uperr := client.Do(updatereq)
		if uperr != nil {
			fmt.Println(uperr)
			return
		}
	}
	getStackerzLeaderboardHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("leaderboardType") == "family" {
			output, outerr := globalvars.Db.Query("select substr(username,0,14), bonus_points, level from tfldata.stack_leaderboard order by(bonus_points+level) desc limit 20;")
			if outerr != nil {
				fmt.Println(outerr)
			}
			defer output.Close()
			iter := 1
			for output.Next() {
				var username string
				var bonus_points string
				var level string
				scnerr := output.Scan(&username, &bonus_points, &level)
				if scnerr != nil {
					fmt.Println(scnerr)
				}
				dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 2%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 15%;'>" + username + "</p><p class='px-2 m-0' style='text-align: center; position: relative; left: 25%;'>" + bonus_points + "</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 75%;'>" + level + "</p></div><br/>"
				iter++
				w.Write([]byte(dataStr))
			}
		} else if r.URL.Query().Get("leaderboardType") == "global" {
			eventYearConverted, _ := strconv.Atoi(r.URL.Query().Get("eventYear"))
			eventPeriodConverted, _ := strconv.Atoi(r.URL.Query().Get("eventPeriod"))
			var startPeriodMonth int
			var endPeriodMonth int
			switch eventPeriodConverted {
			case 1:
				startPeriodMonth = 0
				endPeriodMonth = 4
			case 2:
				startPeriodMonth = 3
				endPeriodMonth = 7
			case 3:
				startPeriodMonth = 6
				endPeriodMonth = 10
			case 4:
				startPeriodMonth = 9
				endPeriodMonth = 13
			}

			out, err := coll.Aggregate(context.TODO(), bson.A{
				bson.D{{Key: "$match", Value: bson.D{{Key: "game", Value: "stackerz"}}}},
				bson.D{
					{Key: "$set",
						Value: bson.D{
							{Key: "score",
								Value: bson.D{
									{Key: "$sum",
										Value: bson.A{
											"$bonus_points",
											"$level",
										},
									},
								},
							},
						},
					},
				},
				bson.D{
					{Key: "$set",
						Value: bson.D{
							{Key: "year",
								Value: bson.D{
									{Key: "$abs",
										Value: bson.D{
											{Key: "$subtract",
												Value: bson.A{
													2020,
													bson.D{{Key: "$year", Value: "$createdOn"}},
												},
											},
										},
									},
								},
							},
							{Key: "month", Value: bson.D{{Key: "$month", Value: "$createdOn"}}},
							{Key: "day", Value: bson.D{
								{Key: "$subtract",
									Value: bson.A{
										bson.D{{Key: "$dayOfWeek", Value: "$createdOn"}},
										1,
									},
								},
							},
							},
						},
					},
				},
				bson.D{
					{Key: "$match",
						Value: bson.D{
							{Key: "year", Value: eventYearConverted},
							{Key: "month",
								Value: bson.D{
									{Key: "$gt", Value: startPeriodMonth},
									{Key: "$lt", Value: endPeriodMonth},
								},
							},
							{Key: "$and",
								Value: bson.A{
									bson.D{{Key: "day", Value: bson.D{{Key: "$lt", Value: 22}}}},
									bson.D{
										{Key: "month",
											Value: bson.D{
												{Key: "$ne",
													Value: bson.A{
														3,
														6,
														9,
														12,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				bson.D{{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}}}},
				bson.D{{Key: "$limit", Value: 15}},
			})

			if err != nil {
				fmt.Print(err)
			}
			defer out.Close(context.TODO())
			iter := 1

			var results []bson.M

			if err = out.All(context.TODO(), &results); err != nil {
				log.Fatal(err)
			}
			for _, result := range results {
				dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 1%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-1 m-0' style='text-align: center; position: absolute; left: 11%;'>" + result["username"].(string) + "</p><p class='px-2 mx-3' style='text-align: center; position: absolute; left: 41%;'>" + fmt.Sprint(result["bonus_points"].(int32)) + "</p><p class='px-2 mx-3' style='text-align: center; position: absolute; left: 56%;'>" + fmt.Sprint(result["level"].(int32)) + "</p><p class='px-2 mx-3' style='text-align: center; position: absolute; left: 76%;'>" + strings.Split(result["org_id"].(string), "_")[0] + "</p></div><br/>"
				iter++
				w.Write([]byte(dataStr))
				if iter == 20 {
					return
				}
			}
		}
	}
	updateStackerzScoreHandler := func(w http.ResponseWriter, r *http.Request) {
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
			Username    string `json:"username"`
			BonusPoints int    `json:"bonus_points"`
			Level       int    `json:"level"`
		}
		var postData postBody
		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			fmt.Println(marsherr)
		}
		_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.stack_leaderboard(\"username\", \"bonus_points\", \"level\") values('%s', %d, %d)", postData.Username, postData.BonusPoints, postData.Level))
		if inserr != nil {
			activityStr := fmt.Sprintf("could not update stackerz leaderboard for %s", currentUserFromSession)
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		}
		coll.InsertOne(context.TODO(), bson.M{"org_id": globalvars.OrgId, "game": "stackerz", "bonus_points": postData.BonusPoints, "level": postData.Level, "username": postData.Username, "createdOn": time.Now()})
	}
	getPersonalCatchitLeaderboardHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		output, outerr := globalvars.Db.Query(fmt.Sprintf("select username, score from tfldata.catchitleaderboard where username='%s' order by score desc limit 20;", currentUserFromSession))
		if outerr != nil {
			fmt.Println(outerr)
		}
		defer output.Close()
		iter := 1
		for output.Next() {
			var username string
			var score string
			scnerr := output.Scan(&username, &score)
			if scnerr != nil {
				fmt.Println(scnerr)
			}
			dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 2%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 15%;'>" + username + "</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 65%;'>" + score + "</p></div><br/>"
			iter++
			w.Write([]byte(dataStr))
		}
	}
	getCatchitLeaderboardHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("leaderboardType") == "family" {
			output, outerr := globalvars.Db.Query("select username, score from tfldata.catchitleaderboard order by score desc limit 20;")
			if outerr != nil {
				fmt.Println(outerr)
			}
			defer output.Close()
			iter := 1
			for output.Next() {
				var username string
				var score string
				scnerr := output.Scan(&username, &score)
				if scnerr != nil {
					fmt.Println(scnerr)
				}
				dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 2%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 15%;'>" + username + "</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 65%;'>" + score + "</p></div><br/>"
				iter++
				w.Write([]byte(dataStr))
			}
		} else if r.URL.Query().Get("leaderboardType") == "global" {
			eventYearConverted, _ := strconv.Atoi(r.URL.Query().Get("eventYear"))
			eventPeriodConverted, _ := strconv.Atoi(r.URL.Query().Get("eventPeriod"))
			var startPeriodMonth int
			var endPeriodMonth int
			switch eventPeriodConverted {
			case 1:
				startPeriodMonth = 0
				endPeriodMonth = 4
			case 2:
				startPeriodMonth = 3
				endPeriodMonth = 7
			case 3:
				startPeriodMonth = 6
				endPeriodMonth = 10
			case 4:
				startPeriodMonth = 9
				endPeriodMonth = 13
			}
			out, err := coll.Aggregate(context.TODO(), bson.A{
				bson.D{{Key: "$match", Value: bson.D{{Key: "game", Value: "catchit"}}}},
				bson.D{
					{Key: "$set",
						Value: bson.D{
							{Key: "year",
								Value: bson.D{
									{Key: "$abs",
										Value: bson.D{
											{Key: "$subtract",
												Value: bson.A{
													2020,
													bson.D{{Key: "$year", Value: "$createdOn"}},
												},
											},
										},
									},
								},
							},
							{Key: "month", Value: bson.D{{Key: "$month", Value: "$createdOn"}}},
							{Key: "day", Value: bson.D{
								{Key: "$subtract",
									Value: bson.A{
										bson.D{{Key: "$dayOfMonth", Value: "$createdOn"}},
										1,
									},
								},
							},
							},
						},
					},
				},
				bson.D{
					{Key: "$match",
						Value: bson.D{
							{Key: "year", Value: eventYearConverted},
							{Key: "month",
								Value: bson.D{
									{Key: "$gt", Value: startPeriodMonth},
									{Key: "$lt", Value: endPeriodMonth},
								},
							},
							{Key: "$and",
								Value: bson.A{
									bson.D{{Key: "day", Value: bson.D{{Key: "$lt", Value: 22}}}},
									bson.D{
										{Key: "month",
											Value: bson.D{
												{Key: "$ne",
													Value: bson.A{
														3,
														6,
														9,
														12,
													},
												},
											},
										},
									},
								}},
						},
					},
				},
				bson.D{{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}}}},
				bson.D{{Key: "$limit", Value: 15}},
			})

			if err != nil {
				fmt.Print(err)
			}
			defer out.Close(context.TODO())
			iter := 1

			var results []bson.M

			if err = out.All(context.TODO(), &results); err != nil {
				log.Fatal(err)
			}
			for _, result := range results {
				dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 1%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-1 m-0' style='text-align: center; position: absolute; left: 13%;'>" + result["username"].(string) + "</p><p class='px-2 mx-5' style='text-align: center; position: absolute; left: 40%;'>" + fmt.Sprint(result["score"].(int32)) + "</p><p class='px-2 mx-5' style='text-align: center; position: absolute; left: 65%;'>" + strings.Split(result["org_id"].(string), "_")[0] + "</p></div><br/>"
				iter++
				w.Write([]byte(dataStr))
			}

		}
	}
	updateCatchitScoreHandler := func(w http.ResponseWriter, r *http.Request) {
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
			Score    int    `json:"score"`
		}
		var postData postBody
		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			fmt.Println(marsherr)
		}

		_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.catchitleaderboard(\"username\", \"score\", \"createdon\") values('%s', '%d', now());", postData.Username, postData.Score))
		if inserr != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		coll.InsertOne(context.TODO(), bson.M{"org_id": globalvars.OrgId, "game": "catchit", "score": postData.Score, "username": postData.Username, "createdOn": time.Now()})

	}
	getLeaderboardHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("leaderboardType") == "family" {
			output, outerr := globalvars.Db.Query("select username, score from tfldata.ss_leaderboard order by score desc limit 20;")
			if outerr != nil {
				fmt.Println(outerr)
			}
			defer output.Close()
			iter := 1
			for output.Next() {
				var username string
				var score string
				scnerr := output.Scan(&username, &score)
				if scnerr != nil {
					fmt.Println(scnerr)
				}
				// dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0'>" + fmt.Sprintf("%d", iter) + "</p><p class='px-2 m-0' style='text-align: center;'>" + username + " - " + score + "</p></div><br/>"
				dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 2%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 15%;'>" + username + "</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 65%;'>" + score + "</p></div><br/>"
				iter++
				w.Write([]byte(dataStr))
			}
		} else if r.URL.Query().Get("leaderboardType") == "global" {
			eventYearConverted, _ := strconv.Atoi(r.URL.Query().Get("eventYear"))
			eventPeriodConverted, _ := strconv.Atoi(r.URL.Query().Get("eventPeriod"))
			var startPeriodMonth int
			var endPeriodMonth int
			switch eventPeriodConverted {
			case 1:
				startPeriodMonth = 0
				endPeriodMonth = 4
			case 2:
				startPeriodMonth = 3
				endPeriodMonth = 7
			case 3:
				startPeriodMonth = 6
				endPeriodMonth = 10
			case 4:
				startPeriodMonth = 9
				endPeriodMonth = 13
			}
			out, err := coll.Aggregate(context.TODO(), bson.A{
				bson.D{{Key: "$match", Value: bson.D{{Key: "game", Value: "simple_shades"}}}},
				bson.D{
					{Key: "$set",
						Value: bson.D{
							{Key: "year",
								Value: bson.D{
									{Key: "$abs",
										Value: bson.D{
											{Key: "$subtract",
												Value: bson.A{
													2020,
													bson.D{{Key: "$year", Value: "$createdOn"}},
												},
											},
										},
									},
								},
							},
							{Key: "month", Value: bson.D{{Key: "$month", Value: "$createdOn"}}},
							{Key: "day", Value: bson.D{
								{Key: "$subtract",
									Value: bson.A{
										bson.D{{Key: "$dayOfWeek", Value: "$createdOn"}},
										1,
									},
								},
							},
							},
						},
					},
				},
				bson.D{
					{Key: "$match",
						Value: bson.D{
							{Key: "year", Value: eventYearConverted},
							{Key: "month",
								Value: bson.D{
									{Key: "$gt", Value: startPeriodMonth},
									{Key: "$lt", Value: endPeriodMonth},
								},
							},
							{Key: "$and",
								Value: bson.A{
									bson.D{{Key: "day", Value: bson.D{{Key: "$lt", Value: 22}}}},
									bson.D{
										{Key: "month",
											Value: bson.D{
												{Key: "$ne",
													Value: bson.A{
														3,
														6,
														9,
														12,
													},
												},
											},
										},
									},
								}},
						},
					},
				},
				bson.D{{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}}}},
				bson.D{{Key: "$limit", Value: 15}},
			})

			if err != nil {
				fmt.Print(err)
			}
			defer out.Close(context.TODO())
			iter := 1

			var results []bson.M

			if err = out.All(context.TODO(), &results); err != nil {
				log.Fatal(err)
			}
			for _, result := range results {
				dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 1%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-1 m-0' style='text-align: center; position: absolute; left: 13%;'>" + result["username"].(string) + "</p><p class='px-2 mx-5' style='text-align: center; position: absolute; left: 40%;'>" + fmt.Sprint(result["score"].(int32)) + "</p><p class='px-2 mx-5' style='text-align: center; position: absolute; left: 55%;'>" + strings.Split(result["org_id"].(string), "_")[0] + "</p></div><br/>"
				iter++
				w.Write([]byte(dataStr))
			}
		}
	}
	updateSimpleShadesScoreHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		bs, _ := io.ReadAll(r.Body)
		type postBody struct {
			Username string `json:"username"`
			Score    int    `json:"score"`
		}
		var postData postBody
		errmarsh := json.Unmarshal(bs, &postData)
		if errmarsh != nil {
			fmt.Println(errmarsh)
		}

		_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.ss_leaderboard(\"username\", \"score\") values('%s', '%d');", postData.Username, postData.Score))
		if inserr != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		coll.InsertOne(context.TODO(), bson.M{"org_id": globalvars.OrgId, "game": "simple_shades", "score": postData.Score, "username": postData.Username, "createdOn": time.Now()})

	}

	createNewTimeCapsuleHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/form-data")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var expiresOn string
		var curAmountOfStoredCapsules int
		var nameExists string

		searchForName := globalvars.Db.QueryRow(fmt.Sprintf("select tcname from tfldata.timecapsule where tcname='%s' and username='%s' limit 1;", r.PostFormValue("tcName"), currentUserFromSession))

		searchForName.Scan(&nameExists)

		if len(nameExists) > 0 {
			w.WriteHeader(http.StatusNotAcceptable)
			w.Write([]byte("Please use a unique name."))
			return
		}

		row := globalvars.Db.QueryRow(fmt.Sprintf("select count(*) from tfldata.timecapsule where username='%s' and available_on > now();", currentUserFromSession))
		row.Scan(&curAmountOfStoredCapsules)

		if curAmountOfStoredCapsules >= 5 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("You currently have the maximum amount of time capsules (5). You can delete ones you no longer want to store."))
			return
		}

		curDate := time.Now().Format(time.DateOnly)
		tcFileName := curDate + "_" + r.PostFormValue("tcName") + "_capsule_" + currentUserFromSession + ".zip"
		tcFile, err := os.Create(tcFileName)
		if err != nil {
			activityStr := "Failed to create zip file in tccreatehandler"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
			return
		}
		var yearsfordb int
		if r.PostFormValue("yearsToStore") == "one_year" {
			expiresOn = time.Now().Add(time.Hour * 8760).Format(time.DateOnly)
			yearsfordb = 1
		} else if r.PostFormValue("yearsToStore") == "three_years" {
			expiresOn = time.Now().Add(time.Hour * 8760 * 3).Format(time.DateOnly)
			yearsfordb = 3
		} else if r.PostFormValue("yearsToStore") == "seven_years" {
			expiresOn = time.Now().Add(time.Hour * 8760 * 7).Format(time.DateOnly)
			yearsfordb = 7
		}
		zipWriter := zip.NewWriter(tcFile)
		parseerr := r.ParseMultipartForm(10 << 20)
		if parseerr != nil {
			activityStr := "Failed to parse multipart form create tc"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", parseerr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
			return
		}
		totalFilesSize := 0
		for _, fh := range r.MultipartForm.File["tcfileinputname"] {

			f, openErr := fh.Open()
			if openErr != nil {
				activityStr := "failed to open multipart file tc create"
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", openErr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
				return
			}

			w1, createerr := zipWriter.Create("timecapsule/" + fh.Filename)
			if createerr != nil {
				activityStr := "Err creating file to place in zip tccreate handler"
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", createerr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
				return
			}
			_, copyerr := io.Copy(w1, f)
			if copyerr != nil {
				activityStr := "Err copying file to zip create tc handler"
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", copyerr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
				return
			}
			totalFilesSize += int(fh.Size / 1024 / 1024)
			/*if err != nil {
				activityStr := fmt.Sprintf("Open multipart file in createtimecapsulehandler - %s", currentUserFromSession)
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
				w.WriteHeader(http.StatusUnsupportedMediaType)
				return
			}*/
			f.Close()
		}
		zipWriter.Close()

		_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.timecapsule(\"username\", \"available_on\", \"tcname\", \"tcfilename\", \"createdon\", waspurchased, wasearlyaccesspurchased, yearstostore, wasrequested, wasdownloaded) values('%s', '%s'::date + INTERVAL '2 days', '%s', '%s', '%s', false, false, %d, false, false);", currentUserFromSession, expiresOn, r.PostFormValue("tcName"), tcFileName, curDate, yearsfordb))

		if inserr != nil {
			activityStr := "Failed to add time capsule to DB"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
			return
		}

		go globalfunctions.UploadTimeCapsuleToS3(tcFile, tcFileName, fmt.Sprint(yearsfordb))
	}
	initiateMyTCRestoreHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
			Tcfilename string `json:"tcfilename"`
		}
		var postData postBody
		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			fmt.Println(marsherr)
		}
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.timecapsule set wasrequested = true where tcfilename='%s';", postData.Tcfilename))
		if uperr != nil {
			activityStr := "Update tc after requested"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values('%s', '%s', now());", uperr.Error(), activityStr))
			return
		}

		_, reserr := globalvars.S3Client.RestoreObject(context.TODO(), &s3.RestoreObjectInput{
			Bucket: &globalvars.S3Domain,
			Key:    aws.String("timecapsules/" + postData.Tcfilename),
			RestoreRequest: &types.RestoreRequest{
				Days: aws.Int32(7),
				GlacierJobParameters: &types.GlacierJobParameters{
					Tier: types.TierStandard,
				},
			},
		})
		if reserr != nil {
			fmt.Println(reserr)
		}
		w.Header().Set("HX-Retarget", "document.body")
	}
	getMyTcRequestStatusHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		returnobj, returnerr := globalvars.S3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
			Bucket: &globalvars.S3Domain,
			Key:    aws.String("timecapsules/" + r.URL.Query().Get("tcfilename")),
		})
		if returnerr != nil {
			fmt.Print("something went wrong: ")
			fmt.Println(returnerr)
			w.WriteHeader(http.StatusAccepted)
			return
		}
		type postBody struct {
			RestoreStatus bool `json:"status"`
		}

		var postData postBody
		if returnobj.Restore != nil {

			if strings.Contains(*returnobj.Restore, "true") {
				// copy s3 obj
				postData = postBody{
					RestoreStatus: false,
				}

			} else {
				postData = postBody{
					RestoreStatus: true,
				}
				_, cperr := globalvars.S3Client.CopyObject(context.TODO(), &s3.CopyObjectInput{
					Bucket:       &globalvars.S3Domain,
					CopySource:   aws.String(globalvars.S3Domain + "/timecapsules/" + r.URL.Query().Get("tcfilename")),
					Key:          aws.String("timecapsules/restored/" + r.URL.Query().Get("tcfilename")),
					StorageClass: types.StorageClassStandard,
				})

				if cperr != nil {
					fmt.Println(cperr)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			}
			bs, marsherr := json.Marshal(&postData)
			if marsherr != nil {
				fmt.Println("err marshing")
				return
			}

			w.Write(bs)
		} else {
			postData = postBody{
				RestoreStatus: true,
			}
			bs, marsherr := json.Marshal(&postData)
			if marsherr != nil {
				fmt.Println("err marshing")
				return
			}
			w.Write(bs)
		}

	}
	availableTcWasDownloaded := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
			Tcfilename string `json:"tcfilename"`
		}
		var postData postBody
		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			fmt.Println(marsherr)
			return
		}
		globalvars.Db.Exec(fmt.Sprintf("update tfldata.timecapsule set wasdownloaded = true where tcfilename='%s';", postData.Tcfilename))
	}
	getMyAvailableTimeCapsulesHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		type listOfTC struct {
			tcname        string
			createdon     string
			tcfilename    string
			wasrequested  sql.NullBool
			wasdownloaded sql.NullBool
		}

		output, _ := globalvars.Db.Query(fmt.Sprintf("select tcname, tcfilename, createdon, wasrequested, wasdownloaded from tfldata.timecapsule where username='%s' and available_on <= now() + interval '1 day' and waspurchased = true order by available_on asc;", currentUserFromSession))

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
			showReqStatusDataStr := ""
			showDownLinkDataStr := ""
			output.Scan(&myTcOut.tcname, &myTcOut.tcfilename, &myTcOut.createdon, &myTcOut.wasrequested, &myTcOut.wasdownloaded)
			if !myTcOut.wasrequested.Valid {
				myTcOut.wasrequested.Bool = false
			}
			if !myTcOut.wasdownloaded.Valid {
				myTcOut.wasdownloaded.Bool = false
			}

			if myTcOut.wasrequested.Bool {
				showReqStatusDataStr = fmt.Sprintf("<i hx-ext='json-enc' hx-get='/get-my-tc-req-status?tcfilename=%s' hx-target='this' hx-swap='none' hx-on::after-request='alertStatus(event)' hx-trigger='click' class='bi bi-arrow-clockwise toggleArrows'></i>", myTcOut.tcfilename)
				showDownLinkDataStr = fmt.Sprintf("<div hx-ext='json-enc' hx-post='/available-tc-was-downloaded' hx-vals='js:{\"tcfilename\": \"%s\"}' hx-swap='none' hx-target='this'><a target='_blank' href='https://%s/timecapsules/restored/%s'>download</a></div>", myTcOut.tcfilename, globalvars.Cfdistro, myTcOut.tcfilename)
			} else {
				showReqStatusDataStr = fmt.Sprintf("<button class='btn' style='border-width: thin; border-color: black; border-radius: 15px / 15px; padding-top: 1&percnt;; padding-bottom: 1&percnt;; box-shadow: 3px 3px 4px;' hx-post='/initiate-tc-req-for-archive-file' hx-ext='json-enc' hx-swap='none' hx-trigger='click' hx-vals='js:{\"tcfilename\": \"%s\"}' hx-on::after-request='initiateRestoreResp(event)'>Get file</button>", myTcOut.tcfilename)
			}

			w.Write([]byte(fmt.Sprintf("<tr><td style='background-color: %s'>%s</td><td style='background-color: %s'>%s</td><td style='background-color: %s; text-align: center'>%s</td><td  style='background-color: %s; text-align: center;'>%s</td></tr>", bgColor, myTcOut.tcname, bgColor, strings.Split(myTcOut.createdon, "T")[0], bgColor, showReqStatusDataStr, bgColor, showDownLinkDataStr)))
			iter++
		}
	}
	getMyNotYetPurchasedTimeCapsulesHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		type listOfTC struct {
			tcname      string
			createdon   string
			availableOn string
			tcfilename  string
		}

		output, _ := globalvars.Db.Query(fmt.Sprintf("select tcname, createdon, available_on, tcfilename from tfldata.timecapsule where username='%s' and waspurchased = false order by available_on asc;", currentUserFromSession))

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

			output.Scan(&myTcOut.tcname, &myTcOut.createdon, &myTcOut.availableOn, &myTcOut.tcfilename)

			w.Write([]byte(fmt.Sprintf("<tr><td class='toggleArrows' onclick='openInStore(`%s`, `%s`, `%s`, `b4c9da54-cdd2-b747-a2bf-2db7bb015cd2`, `notyetpurchased`)' style='background-color: %s'>%s&nbsp;&nbsp;<span class='glyphicon glyphicon-new-window'></span></td><td style='background-color: %s; text-align: center'>%s</td><td style='background-color: %s; text-align: center'>%s</td><td  style='background-color: %s; text-align: center; font-size: larger; color: red;' class='toggleArrows' hx-swap='none' hx-post='/delete-my-tc' hx-ext='json-enc' hx-vals='{%s: %s}' hx-confirm='This will delete the time capsule forever and it will be unretrievable. Are you sure you want to continue?'>X</td></tr>", myTcOut.tcfilename, globalvars.OrgId, strings.Split(globalvars.OrgId, "_")[0], bgColor, myTcOut.tcname, bgColor, strings.Split(myTcOut.createdon, "T")[0], bgColor, strings.Split(myTcOut.availableOn, "T")[0], bgColor, "\"myTCName\"", "\""+myTcOut.tcname+"\"")))
			iter++
		}
	}

	getMyPurchasedTimeCapsulesHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		type listOfTC struct {
			tcname                  string
			createdon               string
			availableOn             string
			tcfilename              string
			wasearlyaccesspurchased *sql.NullBool
		}

		output, _ := globalvars.Db.Query(fmt.Sprintf("select tcname, createdon, available_on, tcfilename, wasearlyaccesspurchased from tfldata.timecapsule where username='%s' and available_on %s now() and waspurchased = true order by available_on asc;", currentUserFromSession, r.URL.Query().Get("pastorpresent")))

		defer output.Close()

		iter := 0

		for output.Next() {
			bgColor := "white"
			var myTcOut listOfTC
			var eabool bool
			var openinstorestr string
			var openinnewwindowstr string
			if iter%2 == 0 {
				bgColor = "white"
			} else {
				bgColor = "#efefefe6"
			}

			output.Scan(&myTcOut.tcname, &myTcOut.createdon, &myTcOut.availableOn, &myTcOut.tcfilename, &myTcOut.wasearlyaccesspurchased)

			if myTcOut.wasearlyaccesspurchased.Valid {
				eabool = myTcOut.wasearlyaccesspurchased.Bool
			} else {
				eabool = false
			}

			if eabool || r.URL.Query().Get("pastorpresent") == "<" {
				openinstorestr = ""
				openinnewwindowstr = ""
			} else {
				openinstorestr = fmt.Sprintf("class='toggleArrows' onclick='openInStore(`%s`, `%s`, `%s`, `3452d556-4cc6-b5ba-9d8d-e5382a7c97b1`, `purchasedAndWantEarly`)'", myTcOut.tcfilename, globalvars.OrgId, strings.Split(globalvars.OrgId, "_")[0])
				openinnewwindowstr = "&nbsp;&nbsp;<span class='glyphicon glyphicon-new-window'></span>"
			}
			w.Write([]byte(fmt.Sprintf("<tr><td %s style='background-color: %s'>%s%s</td><td style='background-color: %s; text-align: center'>%s</td><td style='background-color: %s; text-align: center'>%s</td><td class='toggleArrows' style='background-color: %s; text-align: center; font-size: larger; color: red;' hx-swap='none' hx-post='/delete-my-tc' hx-ext='json-enc' hx-vals='{%s: %s}' hx-confirm='This will delete the time capsule forever and it will be unretrievable. Are you sure you want to continue?'>X</td></tr>", openinstorestr, bgColor, myTcOut.tcname, openinnewwindowstr, bgColor, strings.Split(myTcOut.createdon, "T")[0], bgColor, strings.Split(myTcOut.availableOn, "T")[0], bgColor, "\"myTCName\"", "\""+myTcOut.tcname+"\"")))
			iter++
		}
	}
	wixWebhookChangePlanHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := globalfunctions.ValidateWebhookJWTToken(globalvars.JwtSignKey, r)
		if !validBool {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		bs, _ := io.ReadAll(r.Body)

		type postBody struct {
			Plan string `json:"plan-name"`
		}
		var postData postBody

		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			fmt.Println("Some marsh err at wixWebhookearlyaccess")
			return
		}

		envvar, filereaderr := os.ReadFile(".env")
		if filereaderr != nil {
			fmt.Println(filereaderr)
			return
		}

		rewrite := bytes.ReplaceAll(envvar, []byte("SUB_PACKAGE="+globalvars.SubLevel), []byte("SUB_PACKAGE="+strings.ToLower(postData.Plan)))
		writeerr := os.WriteFile(".env", rewrite, 0644)
		if writeerr != nil {
			fmt.Println(writeerr)
			return
		}
		globalvars.SubLevel = strings.ToLower(postData.Plan)
	}
	getCurrentUserSubPlan := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var wixMemberId sql.NullString
		var userEmail string
		sqlrow := globalvars.Db.QueryRow(fmt.Sprintf("select wix_member_id, email from tfldata.users where username='%s';", currentUserFromSession))
		sqlrow.Scan(&wixMemberId, &userEmail)
		if !wixMemberId.Valid {
			w.WriteHeader(http.StatusFailedDependency)
			w.Write([]byte(userEmail))
			return
		}
		type operatorDataType string
		type titleObj struct {
			Oper operatorDataType `json:"$eq"`
		}
		type orgidObj struct {
			Oper operatorDataType `json:"$eq"`
		}
		type usernameObj struct {
			Oper operatorDataType `json:"$eq"`
		}

		type filterTitleObj struct {
			Title    titleObj    `json:"title"`
			OrgId    orgidObj    `json:"orgid"`
			Username usernameObj `json:"username"`
		}

		type postReqQueryObj struct {
			Filter filterTitleObj `json:"filter"`
			Fields []string       `json:"fields"`
		}
		type postReqObj struct {
			DataCollectionId string          `json:"dataCollectionId"`
			Query            postReqQueryObj `json:"query"`
		}
		postReqBody := postReqObj{
			DataCollectionId: "regular-user-subscriptions",
			Query: postReqQueryObj{
				Filter: filterTitleObj{
					Title: titleObj{
						Oper: operatorDataType(wixMemberId.String),
					},
					OrgId: orgidObj{
						Oper: operatorDataType(globalvars.OrgId),
					},
					Username: usernameObj{
						Oper: operatorDataType(currentUserFromSession),
					},
				},
				Fields: []string{"orderid"},
			},
		}
		type collBody struct {
			Orderid string `json:"orderid,omitempty"`
		}
		type dataItems []struct {
			Id           string   `json:"id,omitempty"`
			CollBodyData collBody `json:"data"`
		}

		type clientListStruct struct {
			Dataitems dataItems `json:"dataItems"`
		}
		var respObj clientListStruct

		jsonMarshed, errMarsh := json.Marshal(&postReqBody)
		if errMarsh != nil {
			fmt.Println(errMarsh)
		}

		req, reqerr := http.NewRequest("POST", "https://www.wixapis.com/wix-data/v2/items/query", bytes.NewReader(jsonMarshed))
		if reqerr != nil {
			fmt.Println(reqerr)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", globalvars.Wixapikey)
		req.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
		req.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		bs, readerr := io.ReadAll(resp.Body)
		if readerr != nil {
			fmt.Println(readerr)
			return
		}

		marsherr := json.Unmarshal(bs, &respObj)
		if marsherr != nil {
			fmt.Println(marsherr)
			return
		}
		if len(respObj.Dataitems) > 0 {
			orderID := respObj.Dataitems[0].CollBodyData.Orderid

			orderreq, orderreqerr := http.NewRequest("GET", "https://www.wixapis.com/pricing-plans/v2/orders/"+orderID, nil)
			orderreq.Header.Set("Content-Type", "application/json")
			orderreq.Header.Set("Authorization", globalvars.Wixapikey)
			orderreq.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
			orderreq.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
			if orderreqerr != nil {
				fmt.Println(orderreqerr)
				return
			}
			orderdataResp, orderDataRespErr := client.Do(orderreq)
			if orderDataRespErr != nil {
				fmt.Println(orderDataRespErr)
				return
			}
			type innerCycleObj struct {
				Index       int    `json:"index"`
				StartedDate string `json:"startedDate"`
				EndedDate   string `json:"endedDate"`
			}

			type cycleDurObj struct {
				Count int    `json:"count"`
				Unit  string `json:"unit"`
			}
			type cancellationObj struct {
				Cause       string `json:"cause"`
				EffectiveAt string `json:"effectiveAt"`
			}
			type subscObj struct {
				CycleDuration cycleDurObj `json:"cycleDuration"`
				CycleCount    int         `json:"cycleCount"`
			}
			type pricingObj struct {
				Subscription subscObj `json:"subscription"`
			}
			type buyerObj struct {
				MemberId  string `json:"memberId"`
				ContactId string `json:"contactId"`
			}
			type orderObj struct {
				Id                   string          `json:"id"`
				PlanId               string          `json:"planId"`
				SubscriptionId       string          `json:"subscriptionId"`
				Buyer                buyerObj        `json:"buyer"`
				Status               string          `json:"status"`
				StatusNew            string          `json:"statusNew"`
				StartDate            string          `json:"startDate"`
				PlanName             string          `json:"planName"`
				PlanDesc             string          `json:"planDescription"`
				PlanPrice            string          `json:"planPrice"`
				WixPayOrderId        string          `json:"wixPayOrderId"`
				PricingData          pricingObj      `json:"pricing"`
				CurrentCycle         innerCycleObj   `json:"currentCycle"`
				Cancellation         cancellationObj `json:"cancellation"`
				AutoRenewedCancelled bool            `json:"autoRenewCanceled"`
			}

			type outerRespObj struct {
				Order orderObj `json:"order"`
			}
			var currentOrderData outerRespObj
			orderRespBody, readerr := io.ReadAll(orderdataResp.Body)
			if readerr != nil {
				fmt.Println(readerr.Error())
				return
			}

			unmarsherr := json.Unmarshal(orderRespBody, &currentOrderData)
			if unmarsherr != nil {
				fmt.Println(unmarsherr.Error())
				return
			}

			marshedObj, marsherr := json.Marshal(currentOrderData)
			if marsherr != nil {
				fmt.Println(marsherr)
				return
			}

			w.Write(marshedObj)
			defer req.Body.Close()
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	cancelCurrentSubRegUserHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		type postBody struct {
			Orderid     string `json:"orderid"`
			CancelNow   bool   `json:"radionow"`
			CancelLater bool   `json:"radiolater"`
		}
		var postData postBody
		bs, _ := io.ReadAll(r.Body)

		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			fmt.Println(marsherr)

		}
		var effectiveAt string
		if postData.CancelLater && !postData.CancelNow {
			effectiveAt = "NEXT_PAYMENT_DATE"
		} else if !postData.CancelLater && postData.CancelNow {
			effectiveAt = "IMMEDIATELY"
		}
		type sendPostBody struct {
			EffectiveAt string `json:"effectiveAt"`
		}
		var sendPostData sendPostBody
		sendPostData.EffectiveAt = effectiveAt
		jsonMarshed, marshlingerr := json.Marshal(sendPostData)
		if marshlingerr != nil {
			fmt.Println(marshlingerr)
			return
		}

		req, reqerr := http.NewRequest("POST", "https://www.wixapis.com/pricing-plans/v2/orders/"+postData.Orderid+"/cancel", bytes.NewReader(jsonMarshed))
		if reqerr != nil {
			activityStr := "error posting to wix cancel order handler"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", reqerr.Error(), activityStr))
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", globalvars.Wixapikey)
		req.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
		req.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
		client := &http.Client{}
		_, clientdoerr := client.Do(req)
		if clientdoerr != nil {
			activityStr := "client error for wix members reset pass only"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", clientdoerr.Error(), activityStr))
			return
		}

		defer client.CloseIdleConnections()

	}
	sendResetPassOnlyHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		bs, _ := io.ReadAll(r.Body)
		type formPostBody struct {
			Email string `json:"currentemail"`
		}
		var formData formPostBody
		json.Unmarshal(bs, &formData)
		type memberChildrenObj struct {
			LoginEmail string `json:"loginEmail"`
		}
		type memberObj struct {
			MemChild memberChildrenObj `json:"member"`
		}

		postReqBody := memberObj{
			MemChild: memberChildrenObj{
				LoginEmail: strings.ToLower(formData.Email),
			},
		}
		jsonMarshed, errMarsh := json.Marshal(&postReqBody)
		if errMarsh != nil {
			activityStr := "error marshing Json body for members reset pass only"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", errMarsh.Error(), activityStr))
			return
		}

		req, reqerr := http.NewRequest("POST", "https://www.wixapis.com/members/v1/members", bytes.NewReader(jsonMarshed))
		if reqerr != nil {
			activityStr := "error posting to wix members sign up / reset pass only handler"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", reqerr.Error(), activityStr))
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", globalvars.Wixapikey)
		req.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
		req.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
		client := &http.Client{}
		createresp, clientdoerr := client.Do(req)
		if clientdoerr != nil {
			activityStr := "client error for wix members reset pass only"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", clientdoerr.Error(), activityStr))
			return
		}

		defer client.CloseIdleConnections()
		type redirectObj struct {
			Url string `json:"url"`
		}

		type sendReset struct {
			Email       string `json:"email"`
			Lang        string `json:"language"`
			RedirectObj redirectObj
		}
		postBody := sendReset{
			Email: strings.ToLower(formData.Email),
			Lang:  "en",
			RedirectObj: redirectObj{
				Url: "https://the-family-loop.com",
			},
		}
		sendjsonMarshed, senderrMarsh := json.Marshal(&postBody)
		if senderrMarsh != nil {
			activityStr := "error marshing Json body for members sign up reset pass only"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", senderrMarsh.Error(), activityStr))
			return
		}
		setpassreq, setpassreqerr := http.NewRequest("POST", "https://www.wixapis.com/_api/iam/recovery/v1/send-email", bytes.NewReader(sendjsonMarshed))
		if setpassreqerr != nil {
			activityStr := "error sending set pass wix members sign up reset pass only"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", setpassreqerr.Error(), activityStr))
			return
		}
		setpassreq.Header.Set("Content-Type", "application/json")
		setpassreq.Header.Set("Authorization", globalvars.Wixapikey)
		setpassreq.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
		setpassreq.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
		sendclient := &http.Client{}
		_, sendclientdoerr := sendclient.Do(setpassreq)
		if sendclientdoerr != nil {
			activityStr := "client error for wix members sign up"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", sendclientdoerr.Error(), activityStr))
			return
		}
		defer sendclient.CloseIdleConnections()
		if createresp.StatusCode == 200 {
			type memberobj struct {
				Id string `json:"id"`
			}
			type memberResponseObj struct {
				Memberstruct memberobj `json:"member"`
			}
			var responseData memberResponseObj
			bs, bserr := io.ReadAll(createresp.Body)

			if bserr != nil {
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,105), substr('%s',0,105), now());", bserr.Error(), "creating bs for wix create site member response"))
			}

			unmarsherr := json.Unmarshal(bs, &responseData)
			if unmarsherr != nil {
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,105), substr('%s',0,105), now());", unmarsherr.Error(), "unmarshal wix create site member response"))
			}
			_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set wix_member_id = '%s' where username = '%s';", responseData.Memberstruct.Id, currentUserFromSession))
			if uperr != nil {
				globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,106), substr('%s',0,105), now());", uperr.Error(), "Err updating users table with wix id"))
			}
		}
		defer createresp.Body.Close()
	}
	regUserPaidForPlanHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := globalfunctions.ValidateWebhookJWTToken(globalvars.JwtSignKey, r)
		if !validBool {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var setStatus = false
		var currentstatus sql.NullBool
		res := globalvars.Db.QueryRow(fmt.Sprintf("select is_paying_subscriber from tfldata.users where username='%s';", r.URL.Query().Get("username")))
		res.Scan(&currentstatus)
		if !currentstatus.Valid {
			currentstatus.Bool = false
		}
		if currentstatus.Bool {
			setStatus = false
		} else {
			setStatus = true
		}
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set is_paying_subscriber = %t where username = '%s';", setStatus, r.URL.Query().Get("username")))
		if uperr != nil {
			activityStr := "updating user is now paying wix webhook"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,105), substr('%s',0,105), now());", uperr.Error(), activityStr))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	wixWebhookTCInitialPurchaseHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := globalfunctions.ValidateWebhookJWTToken(globalvars.JwtSignKey, r)
		if !validBool {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		type postBody struct {
			/*Family      string `json:"family"`
			Orgcode     string `json:"orgcode"`*/
			Capsulename string `json:"capsule"`
			Route       string `json:"route"`
		}
		bs, _ := io.ReadAll(r.Body)
		var postData postBody

		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			fmt.Println("Some marsh err at wixWebhookearlyaccess")
			return
		}
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.timecapsule set waspurchased=true where tcfilename='%s';", postData.Capsulename))
		if uperr != nil {
			fmt.Println("something went wrong")
		}
		/*var yearstostore string
		row := globalvars.Db.QueryRow(fmt.Sprintf("select yearstostore from tfldata.timecapsule where tcfilename='%s';", postData.Capsulename))
		sner := row.Scan(&yearstostore)
		if sner != nil {
			fmt.Println("scan err")
			return
		}
		if yearstostore == "1" {
			yearstostore = "one_year"
		} else if yearstostore == "3" {
			yearstostore = "three_years"
		} else if yearstostore == "7" {
			yearstostore = "seven_years"
		}
		globalvars.S3Client.PutObjectTagging(context.TODO(), &s3.PutObjectTaggingInput{
			Bucket: &globalvars.S3Domain,
			Key:    aws.String("timecapsules/" + postData.Capsulename),
			Tagging: &types.Tagging{
				TagSet: []types.Tag{
					{
						Key:   aws.String("YearsToStore"),
						Value: &yearstostore,
					},
				},
			},
		})*/
	}
	wixWebhookEarlyAccessPaymentCompleteHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := globalfunctions.ValidateWebhookJWTToken(globalvars.JwtSignKey, r)
		if !validBool {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		type postBody struct {
			/*Family      string `json:"family"`
			Orgcode     string `json:"orgcode"`*/
			Capsulename string `json:"capsule"`
			Route       string `json:"route"`
		}
		bs, _ := io.ReadAll(r.Body)
		var postData postBody

		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			fmt.Println("Some marsh err at wixWebhookearlyaccess")
			return
		}
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.timecapsule set wasearlyaccesspurchased=true,available_on=now() where tcfilename='%s';", postData.Capsulename))
		if uperr != nil {
			activityStr := "Failed attempt purchase early access from wix"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,createdon,activity) values(substr('%s',0,240), now(), substr('%s',0,105);", uperr.Error(), activityStr))
			return
		}
		fmt.Println(postData)

	}
	validateEndpointForWixHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := globalfunctions.ValidateWebhookJWTToken(globalvars.JwtSignKey, r)
		if !validBool {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if globalvars.OrgId != r.URL.Query().Get("orgid") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte("true"))
	}
	deleteMyTChandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		type postBody struct {
			MyTCName string `json:"myTCName"`
		}
		type tcData struct {
			username  string
			createdon string
			tcname    string
		}
		var postData postBody
		var selectedTc tcData
		bs, _ := io.ReadAll(r.Body)

		json.Unmarshal(bs, &postData)

		tcrow := globalvars.Db.QueryRow(fmt.Sprintf("select username,createdon,tcname from tfldata.timecapsule where username='%s' and tcname='%s';", currentUserFromSession, postData.MyTCName))

		tcrow.Scan(&selectedTc.username, &selectedTc.createdon, &selectedTc.tcname)

		_, delerr := globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.timecapsule where username='%s' and tcname='%s';", currentUserFromSession, postData.MyTCName))
		if delerr != nil {
			activityStr := "Failed to delete time capsule from DB"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", delerr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
			return
		}
		deletename := strings.Split(selectedTc.createdon, "T")[0] + "_" + selectedTc.tcname + "_capsule_" + selectedTc.username + ".zip"
		go globalfunctions.DeleteFileFromS3(deletename, "timecapsules/")
	}

	validateJWTHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

	}
	/* NOT USING THIS RIGHT NOW */
	/*refreshTokenHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h :=  globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		jwt.Parse(jwtCookie.Value, func(jwtToken *jwt.Token) (interface{}, error) {
			timeTilExp, _ := jwtToken.Claims.GetExpirationTime()
			if time.Until(timeTilExp.Time) < 24*time.Hour {
				globalfunctions.GenerateLoginJWT(r.URL.Query().Get("usersession"), w, r, jwtCookie.Value)

			}
			return []byte(globalvars.JwtSignKey), nil
		}, jwt.WithValidMethods([]string{"HS256"}))

	}*/

	deleteJWTHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		http.SetCookie(w, &http.Cookie{
			Name:     "backendauth",
			Value:    "",
			MaxAge:   0,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
		})
		//globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id=null where username='%s';", r.URL.Query().Get("user")))
	}

	adminGetListOfUsersHandler := func(w http.ResponseWriter, r *http.Request) {
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
	adminGetSubPackageHandler := func(w http.ResponseWriter, r *http.Request) {
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
	adminGetAllTCHandler := func(w http.ResponseWriter, r *http.Request) {
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
	adminDeleteUserHandler := func(w http.ResponseWriter, r *http.Request) {
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

		cursor, findErr := coll.Find(context.TODO(), bson.D{{Key: "username", Value: postData.SelectedUser}, {Key: "org_id", Value: globalvars.OrgId}})
		if findErr != nil {
			fmt.Println(findErr)
		}

		marsherr := cursor.All(context.TODO(), &mongoRecords)
		if marsherr != nil {
			fmt.Println("here: " + marsherr.Error())
		}

		for _, val := range mongoRecords {
			_, delErr := coll.DeleteOne(context.TODO(), bson.D{{Key: "_id", Value: val["_id"]}})
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
	healthCheckHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("true"))
	}

	http.HandleFunc("/", pages.PagesHandler)
	/* posts handlers */
	http.HandleFunc("/create-post", postshandler.CreatePostHandler)
	http.HandleFunc("/create-reaction-to-post", postshandler.CreatePostReactionHandler)
	http.HandleFunc("/create-comment", postshandler.CreateCommentHandler)
	http.HandleFunc("/get-posts", postshandler.GetPostsHandler)
	http.HandleFunc("/delete-this-post", postshandler.DeleteThisPostHandler)
	http.HandleFunc("/get-selected-post", postshandler.GetSelectedPostsComments)
	http.HandleFunc("/get-posts-reactions", postshandler.GetPostsReactionsHandler)
	http.HandleFunc("/get-post-images", postshandler.GetPostImagesHandler)
	/* chat handlers */
	http.HandleFunc("/get-selected-chat", chathandler.GetSelectedChatHandler)
	http.HandleFunc("/get-selected-pchat", chathandler.GetSelectedPChatHandler)
	http.HandleFunc("/group-chat-messages", chathandler.GetGroupChatMessagesHandler)
	http.HandleFunc("/create-a-group-chat-message", chathandler.CreateGroupChatMessageHandler)
	http.HandleFunc("/del-thread", chathandler.DelThreadHandler)
	http.HandleFunc("/get-all-users-to-tag", chathandler.GetUsernamesToTagHandler)
	http.HandleFunc("/change-gchat-order-opt", chathandler.ChangeGchatOrderOptHandler)
	http.HandleFunc("/private-chat-messages", chathandler.GetPrivateChatMessagesHandler)
	http.HandleFunc("/create-a-private-chat-message", chathandler.CreatePrivatePChatMessageHandler)
	http.HandleFunc("/update-last-viewed-direct", chathandler.UpdateLastViewedPChatHandler)
	http.HandleFunc("/update-last-viewed-thread", chathandler.UpdateLastViewedThreadHandler)
	http.HandleFunc("/update-pchat-reaction", chathandler.UpdatePChatReactionHandler)
	http.HandleFunc("/current-pchat-reaction", chathandler.GetCurrentPChatReactionHandler)
	http.HandleFunc("/update-selected-chat", chathandler.UpdateSelectedChatHandler)
	http.HandleFunc("/delete-selected-chat", chathandler.DeleteSelectedChatHandler)
	http.HandleFunc("/update-selected-pchat", chathandler.UpdateSelectedPChatHandler)
	http.HandleFunc("/delete-selected-pchat", chathandler.DeleteSelectedPChatHandler)
	http.HandleFunc("/get-open-threads", chathandler.GetOpenThreadsHandler)
	http.HandleFunc("/get-users-chat", chathandler.GetUsersToChatToHandler)
	http.HandleFunc("/get-users-subscribed-threads", chathandler.GetUsersSubscribedThreadsHandler)
	http.HandleFunc("/change-if-notified-for-thread", chathandler.ChangeUserSubscriptionToThreadHandler)
	/* calendar handlers */
	http.HandleFunc("/get-events", getEventsHandler)
	http.HandleFunc("/get-event-comments", getSelectedEventsComments)
	http.HandleFunc("/create-event-comment", createEventCommentHandler)
	http.HandleFunc("/create-event", createEventHandler)
	http.HandleFunc("/update-rsvp-for-event", updateRSVPForEventHandler)
	http.HandleFunc("/get-rsvp-data", getEventRSVPHandler)
	http.HandleFunc("/get-rsvp", getRSVPNotesHandler)
	http.HandleFunc("/delete-event", deleteEventHandler)

	http.HandleFunc("/get-username-from-session", getSessionDataHandler)
	http.HandleFunc("/get-check-if-subscribed", getSubscribedHandler)

	http.HandleFunc("/create-subscription", subscriptionHandler)

	http.HandleFunc("/update-pfp", updatePfpHandler)
	http.HandleFunc("/update-gchat-bg-theme", updateChatThemeHandler)

	http.HandleFunc("/create-issue", createIssueHandler)
	http.HandleFunc("/get-my-customer-support-issues", getCustomerSupportIssuesHandler)
	http.HandleFunc("/get-issues-comments", getGHIssuesComments)
	http.HandleFunc("/create-issue-comment", createGHIssueCommentHandler)

	http.HandleFunc("/get-leaderboard", getLeaderboardHandler)
	http.HandleFunc("/update-simpleshades-score", updateSimpleShadesScoreHandler)

	http.HandleFunc("/get-stackerz-leaderboard", getStackerzLeaderboardHandler)
	http.HandleFunc("/update-stackerz-score", updateStackerzScoreHandler)

	http.HandleFunc("/get-catchit-leaderboard", getCatchitLeaderboardHandler)
	http.HandleFunc("/get-my-personal-score-catchit", getPersonalCatchitLeaderboardHandler)
	http.HandleFunc("/update-catchit-score", updateCatchitScoreHandler)

	http.HandleFunc("/create-new-tc", createNewTimeCapsuleHandler)
	http.HandleFunc("/get-my-time-capsules", getMyPurchasedTimeCapsulesHandler)
	http.HandleFunc("/get-my-purchased-time-capsules", getMyPurchasedTimeCapsulesHandler)
	http.HandleFunc("/get-my-notyetpurchased-time-capsules", getMyNotYetPurchasedTimeCapsulesHandler)
	http.HandleFunc("/get-my-available-time-capsules", getMyAvailableTimeCapsulesHandler)

	http.HandleFunc("/available-tc-was-downloaded", availableTcWasDownloaded)

	http.HandleFunc("/get-my-tc-req-status", getMyTcRequestStatusHandler)
	http.HandleFunc("/initiate-tc-req-for-archive-file", initiateMyTCRestoreHandler)

	http.HandleFunc("/webhook-tc-early-access-payment-complete", wixWebhookEarlyAccessPaymentCompleteHandler)
	http.HandleFunc("/webhook-tc-initial-payment-complete", wixWebhookTCInitialPurchaseHandler)
	http.HandleFunc("/wix-webhook-pricing-plan-changed", wixWebhookChangePlanHandler)

	http.HandleFunc("/wix-webhook-update-reg-user-paid-plan", regUserPaidForPlanHandler)

	http.HandleFunc("/current-user-wix-subscription", getCurrentUserSubPlan)
	http.HandleFunc("/send-reset-pass-wix-user", sendResetPassOnlyHandler)
	http.HandleFunc("/cancel-current-sub-regular-user", cancelCurrentSubRegUserHandler)

	http.HandleFunc("/delete-my-tc", deleteMyTChandler)

	http.HandleFunc("/admin-list-of-users", adminGetListOfUsersHandler)
	http.HandleFunc("/admin-get-all-time-capsules", adminGetAllTCHandler)
	http.HandleFunc("/admin-get-subscription-package", adminGetSubPackageHandler)
	http.HandleFunc("/admin-delete-user", adminDeleteUserHandler)

	http.HandleFunc("/signup", signUpHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/reset-password", getResetPasswordCodeHandler)
	http.HandleFunc("/reset-password-with-code", resetPasswordHandler)
	http.HandleFunc("/update-admin-pass", updateAdminPassHandler)
	http.HandleFunc("/update-fcm-token", updateFCMTokenHandler)

	http.HandleFunc("/healthy-me-checky", healthCheckHandler)
	http.HandleFunc("/validate-endpoint-from-wix", validateEndpointForWixHandler)

	http.HandleFunc("/jwt-validation-endpoint", validateJWTHandler)
	// NOT USING THIS RIGHT NOW
	//http.HandleFunc("/refresh-token", refreshTokenHandler)
	http.HandleFunc("/delete-jwt", deleteJWTHandler)

	// http.HandleFunc("/upload-file", h3)
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("js"))))
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	log.Fatal(http.ListenAndServe(":80", nil))

}
