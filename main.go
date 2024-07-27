package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	globalfunctions "tfl/functions"
	pages "tfl/handlers"
	calendarhandler "tfl/handlers/calendar"
	chathandler "tfl/handlers/chats"
	cshandler "tfl/handlers/customersupport"
	postshandler "tfl/handlers/posts"
	tchandler "tfl/handlers/timecapsule"
	userdatahandler "tfl/handlers/userdata"
	globalvars "tfl/vars"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	_ "image/png"

	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

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
	http.HandleFunc("/get-events", calendarhandler.GetEventsHandler)
	http.HandleFunc("/get-event-comments", calendarhandler.GetSelectedEventsComments)
	http.HandleFunc("/create-event-comment", calendarhandler.CreateEventCommentHandler)
	http.HandleFunc("/create-event", calendarhandler.CreateEventHandler)
	http.HandleFunc("/update-rsvp-for-event", calendarhandler.UpdateRSVPForEventHandler)
	http.HandleFunc("/get-rsvp-data", calendarhandler.GetEventRSVPHandler)
	http.HandleFunc("/get-rsvp", calendarhandler.GetRSVPNotesHandler)
	http.HandleFunc("/delete-event", calendarhandler.DeleteEventHandler)
	/* Time Capsule handlers */
	http.HandleFunc("/create-new-tc", tchandler.CreateNewTimeCapsuleHandler)
	//http.HandleFunc("/get-my-time-capsules", getMyPurchasedTimeCapsulesHandler)
	http.HandleFunc("/get-my-purchased-time-capsules", tchandler.GetMyPurchasedTimeCapsulesHandler)
	http.HandleFunc("/get-my-notyetpurchased-time-capsules", tchandler.GetMyNotYetPurchasedTimeCapsulesHandler)
	http.HandleFunc("/get-my-available-time-capsules", tchandler.GetMyAvailableTimeCapsulesHandler)
	http.HandleFunc("/available-tc-was-downloaded", tchandler.AvailableTcWasDownloaded)
	http.HandleFunc("/get-my-tc-req-status", tchandler.GetMyTcRequestStatusHandler)
	http.HandleFunc("/initiate-tc-req-for-archive-file", tchandler.InitiateMyTCRestoreHandler)
	http.HandleFunc("/webhook-tc-early-access-payment-complete", tchandler.WixWebhookEarlyAccessPaymentCompleteHandler)
	http.HandleFunc("/webhook-tc-initial-payment-complete", tchandler.WixWebhookTCInitialPurchaseHandler)
	http.HandleFunc("/delete-my-tc", tchandler.DeleteMyTChandler)
	/* User data handlers */
	http.HandleFunc("/get-username-from-session", userdatahandler.GetSessionDataHandler)
	http.HandleFunc("/get-check-if-subscribed", userdatahandler.GetSubscribedHandler)
	http.HandleFunc("/create-subscription", userdatahandler.SubscriptionHandler)
	http.HandleFunc("/update-pfp", userdatahandler.UpdatePfpHandler)
	http.HandleFunc("/update-gchat-bg-theme", userdatahandler.UpdateChatThemeHandler)
	/* Customer Support handlers */
	http.HandleFunc("/create-issue", cshandler.CreateIssueHandler)
	http.HandleFunc("/get-my-customer-support-issues", cshandler.GetCustomerSupportIssuesHandler)
	http.HandleFunc("/get-issues-comments", cshandler.GetGHIssuesComments)
	http.HandleFunc("/create-issue-comment", cshandler.CreateGHIssueCommentHandler)
	/* Games handlers */
	http.HandleFunc("/get-leaderboard", getLeaderboardHandler)
	http.HandleFunc("/update-simpleshades-score", updateSimpleShadesScoreHandler)
	http.HandleFunc("/get-stackerz-leaderboard", getStackerzLeaderboardHandler)
	http.HandleFunc("/update-stackerz-score", updateStackerzScoreHandler)
	http.HandleFunc("/get-catchit-leaderboard", getCatchitLeaderboardHandler)
	http.HandleFunc("/get-my-personal-score-catchit", getPersonalCatchitLeaderboardHandler)
	http.HandleFunc("/update-catchit-score", updateCatchitScoreHandler)
	/* Wix handlers */
	http.HandleFunc("/wix-webhook-pricing-plan-changed", wixWebhookChangePlanHandler)
	http.HandleFunc("/wix-webhook-update-reg-user-paid-plan", regUserPaidForPlanHandler)
	http.HandleFunc("/current-user-wix-subscription", getCurrentUserSubPlan)
	http.HandleFunc("/send-reset-pass-wix-user", sendResetPassOnlyHandler)
	http.HandleFunc("/cancel-current-sub-regular-user", cancelCurrentSubRegUserHandler)
	/* Admin dashboard handlers */
	http.HandleFunc("/admin-list-of-users", adminGetListOfUsersHandler)
	http.HandleFunc("/admin-get-all-time-capsules", adminGetAllTCHandler)
	http.HandleFunc("/admin-get-subscription-package", adminGetSubPackageHandler)
	http.HandleFunc("/admin-delete-user", adminDeleteUserHandler)
	/* Auth handlers */
	http.HandleFunc("/signup", signUpHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/reset-password", getResetPasswordCodeHandler)
	http.HandleFunc("/reset-password-with-code", resetPasswordHandler)
	http.HandleFunc("/update-admin-pass", updateAdminPassHandler)
	http.HandleFunc("/update-fcm-token", updateFCMTokenHandler)
	// NOT USING THIS RIGHT NOW
	//http.HandleFunc("/refresh-token", refreshTokenHandler)
	http.HandleFunc("/delete-jwt", deleteJWTHandler)

	http.HandleFunc("/healthy-me-checky", healthCheckHandler)
	http.HandleFunc("/validate-endpoint-from-wix", validateEndpointForWixHandler)

	http.HandleFunc("/jwt-validation-endpoint", validateJWTHandler)

	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("js"))))
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	log.Fatal(http.ListenAndServe(":80", nil))

}
