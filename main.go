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

	imagego "image"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"

	"image/jpeg"
	_ "image/png"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/github"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/option"
)

type Postsrow struct {
	Id           int
	Title        string
	Description  string
	Author       string
	Postfileskey string
	Createdon    string
}
type Postjoin struct {
	Filename     string
	Filetype     string
	Postfileskey string
}
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
type notificationOpts struct {
	notificationPage  string
	extraPayloadKey   string
	extraPayloadVal   string
	notificationTitle string
	notificationBody  string
	isTagged          bool
}

var awskey string
var awskeysecret string
var ghissuetoken string
var s3Domain string
var nyLoc *time.Location
var s3Client *s3.Client

func main() {
	replacer := strings.NewReplacer("'", "\\'", "\"", "\\\"")
	nyLoc, _ = time.LoadLocation("America/New_York")
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		os.Exit(1)
	}
	dbpass := os.Getenv("DB_PASS")
	awskey = os.Getenv("AWS_ACCESS_KEY")
	awskeysecret = os.Getenv("AWS_ACCESS_SECRET")
	ghissuetoken = os.Getenv("GH_BEARER")
	cfdistro := os.Getenv("CF_DOMAIN")
	s3Domain = os.Getenv("S3_BUCKET_NAME")
	orgId := os.Getenv("ORG_ID")
	mongoDBPass := os.Getenv("MONGO_PASS")
	subLevel := os.Getenv("SUB_PACKAGE")
	jwtSignKey := os.Getenv("JWT_SIGNING_KEY")
	wixapikey := os.Getenv("WIX_API_KEY")

	opts := []option.ClientOption{option.WithCredentialsFile("the-family-loop-fb0d9-firebase-adminsdk-k6sxl-14c7d4c4f7.json")}

	// Initialize firebase app
	app, err := firebase.NewApp(context.TODO(), nil, opts...)
	if err != nil {
		fmt.Printf("Error in initializing firebase app: %s", err)
	}
	fb_message_client, fbInitErr := app.Messaging(context.TODO())
	if fbInitErr != nil {
		fmt.Println("err intializing fb messaging client")
		os.Exit(3)
	}
	// favicon
	faviconHandler := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "assets/favicon.ico")
	}
	http.HandleFunc("/favicon.ico", faviconHandler)
	serviceWorkerHandler := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "firebase-messaging-sw.js")
	}
	http.HandleFunc("/firebase-messaging-sw.js", serviceWorkerHandler)

	// Connect to database
	connStr := fmt.Sprintf("postgresql://tfldbrole:%s@localhost/tfl?sslmode=disable", dbpass)
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	mongoDb, mongoerr := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb+srv://tfl-user:"+mongoDBPass+"@tfl-leaderboard.dg95d1f.mongodb.net/?retryWrites=true&w=majority"))

	if mongoerr != nil {
		fmt.Println(mongoerr)
	}
	defer mongoDb.Disconnect(context.TODO())
	coll := mongoDb.Database("tfl-database").Collection("leaderboards")

	awscfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awskey, awskeysecret, "")),
	)
	sqsClient := sqs.NewFromConfig(awscfg)

	if err != nil {
		log.Fatal(err)
		os.Exit(4)
	}

	s3Client = s3.NewFromConfig(awscfg)

	var postTmpl *template.Template
	var tmerr error

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
		_, inserr := db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id='%s' where session_token='%s';", postData.Fcmtoken, seshVal))
		if inserr != nil {
			activityStr := "attempt update fcm token where seshtoken is value subHandler"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}

	}

	signUpHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/form-data")

		if r.PostFormValue("passwordsignup") != r.PostFormValue("confirmpasswordsignup") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.PostFormValue("orgidinput") != orgId {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var countOfUsers int
		userRowCount := db.QueryRow("select count(*) from tfldata.users;")
		userRowCount.Scan(&countOfUsers)
		switch subLevel {
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
			fmt.Println(errfile)
			activityStr := "uploading pfp during sign up"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errfile, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fn := uploadPfpToS3(upload, filename.Filename, r, "pfpformfile")
		bs := []byte(r.PostFormValue("passwordsignup"))

		bytesOfPass, err := bcrypt.GenerateFromPassword(bs, len(bs))
		if err != nil {
			fmt.Println(err)
		}

		_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.users(\"username\", \"password\", \"pfp_name\", \"email\", \"gchat_bg_theme\", \"gchat_order_option\", \"cf_domain_name\", \"orgid\", \"is_admin\", \"mytz\") values('%s', '%s', '%s', '%s', '%s', %t, '%s', '%s', %t, '%s');", strings.ToLower(r.PostFormValue("usernamesignup")), bytesOfPass, fn, strings.ToLower(r.PostFormValue("emailsignup")), "background: linear-gradient(142deg, #00009f, #3dc9ff 26%)", true, cfdistro, orgId, false, r.PostFormValue("mytz")))

		type memberChildrenObj struct {
			LoginEmail string `json:"loginEmail"`
		}
		type memberObj struct {
			MemChild memberChildrenObj `json:"member"`
		}

		if err != nil {
			fmt.Println(err)
			return
		}

		if errinsert != nil {
			activityStr := "err inserting into users table on sign up"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errinsert, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, errutterr := db.Exec(fmt.Sprintf("insert into tfldata.users_to_threads(\"username\", \"thread\", \"is_subscribed\") values('%s', 'posts', true),('%s', 'calendar',true), ('%s', 'main', true);", strings.ToLower(r.PostFormValue("usernamesignup")), strings.ToLower(r.PostFormValue("usernamesignup")), strings.ToLower(r.PostFormValue("usernamesignup"))))
		if errutterr != nil {
			activityStr := fmt.Sprintf("user %s will not be subscribed to new posts as of now", strings.ToLower(r.PostFormValue("usernamesignup")))
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,106), substr('%s',0,106), now());", errutterr.Error(), activityStr))
		}
		postReqBody := memberObj{
			MemChild: memberChildrenObj{
				LoginEmail: strings.ToLower(r.PostFormValue("emailsignup")),
			},
		}
		jsonMarshed, errMarsh := json.Marshal(&postReqBody)
		if errMarsh != nil {
			activityStr := "error marshing Json body for members sign up"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", errMarsh.Error(), activityStr))
			return
		}

		req, reqerr := http.NewRequest("POST", "https://www.wixapis.com/members/v1/members", bytes.NewReader(jsonMarshed))
		if reqerr != nil {
			activityStr := "error posting to wix members sign up"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", reqerr.Error(), activityStr))
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", wixapikey)
		req.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
		req.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
		client := &http.Client{}
		createresp, clientdoerr := client.Do(req)
		if clientdoerr != nil {
			activityStr := "client error for wix members sign up"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", clientdoerr.Error(), activityStr))
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", senderrMarsh.Error(), activityStr))
			return
		}
		setpassreq, setpassreqerr := http.NewRequest("POST", "https://www.wixapis.com/_api/iam/recovery/v1/send-email", bytes.NewReader(sendjsonMarshed))
		if setpassreqerr != nil {
			activityStr := "error sending set pass wix members sign up"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", setpassreqerr.Error(), activityStr))
			return
		}
		setpassreq.Header.Set("Content-Type", "application/json")
		setpassreq.Header.Set("Authorization", wixapikey)
		setpassreq.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
		setpassreq.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
		sendclient := &http.Client{}
		_, sendclientdoerr := sendclient.Do(setpassreq)
		if sendclientdoerr != nil {
			activityStr := "client error for wix members sign up"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", sendclientdoerr.Error(), activityStr))
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,105), substr('%s',0,105), now());", bserr.Error(), "creating bs for wix create site member response"))
		}

		unmarsherr := json.Unmarshal(bs, &responseData)
		if unmarsherr != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,105), substr('%s',0,105), now());", unmarsherr.Error(), "unmarshal wix create site member response"))
		}
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set wix_member_id = '%s' where username = '%s';", responseData.Memberstruct.Id, strings.ToLower(r.PostFormValue("usernamesignup"))))
		if uperr != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,106), substr('%s',0,105), now());", uperr.Error(), "Err updating users table with wix id"))
		}

		defer createresp.Body.Close()
	}

	loginHandler := func(w http.ResponseWriter, r *http.Request) {

		userStr := strings.ToLower(r.PostFormValue("usernamelogin"))

		var password string
		var isAdmin bool
		var emailIn string
		passScan := db.QueryRow(fmt.Sprintf("select is_admin, password, email from tfldata.users where username='%s' or email='%s';", userStr, userStr))

		scnerr := passScan.Scan(&isAdmin, &password, &emailIn)

		if isAdmin {

			if password == r.PostFormValue("passwordlogin") {

				w.Header().Set("HX-Trigger", "changeAdminPassword")
				setLoginCookie(w, db, userStr, r.PostFormValue("mytz"))
				db.Exec(fmt.Sprintf("update tfldata.users set last_sign_on='%s' where username='%s';", time.Now().In(nyLoc).Format(time.DateTime), userStr))

				generateLoginJWT(userStr, w, jwtSignKey)

			} else {
				err := bcrypt.CompareHashAndPassword([]byte(password), []byte(r.PostFormValue("passwordlogin")))

				if err != nil {

					db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values(substr('%s',0,105), '%s');", err, time.Now().In(nyLoc).Format(time.DateTime)))
					w.WriteHeader(http.StatusUnauthorized)
					return
				} else {

					generateLoginJWT(userStr, w, jwtSignKey)
					setLoginCookie(w, db, userStr, r.PostFormValue("mytz"))
					db.Exec(fmt.Sprintf("update tfldata.users set last_sign_on='%s' where username='%s';", time.Now().In(nyLoc).Format(time.DateTime), userStr))

					w.Header().Set("HX-Refresh", "true")
				}
			}
		} else {
			if scnerr != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('this was the scan error %s with dbpassword *** and form user is %s');", scnerr, userStr))
				fmt.Print(scnerr)
			}
			err := bcrypt.CompareHashAndPassword([]byte(password), []byte(r.PostFormValue("passwordlogin")))

			if err != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values(substr('%s',0,105), '%s');", err, time.Now().In(nyLoc).Format(time.DateTime)))
				w.WriteHeader(http.StatusUnauthorized)
				return
			} else {

				generateLoginJWT(userStr, w, jwtSignKey)
				setLoginCookie(w, db, userStr, r.PostFormValue("mytz"))
				db.Exec(fmt.Sprintf("update tfldata.users set last_sign_on='%s' where username='%s';", time.Now().In(nyLoc).Format(time.DateTime), userStr))

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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values (substr('%s',0,106), substr('%s',0,106), now());", err.Error(), activityStr))
			return
		}
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set password='%s' where username='%s';", newAdminbytesOfPass, postData.Username))
		if uperr != nil {
			fmt.Println(uperr)
			activityStr := "updating admin pass"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values (substr('%s',0,106), substr('%s',0,106), now());", uperr.Error(), activityStr))
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
		row := db.QueryRow(fmt.Sprintf("select email, username, last_pass_reset from tfldata.users where email='%s' and (last_pass_reset < now() - interval '32 hours' or last_pass_reset is null);", emailInput))
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
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var deleteReceipt string
		for _, val := range out.Messages {

			marsherr := json.Unmarshal([]byte(*val.Body), &messageData)
			if marsherr != nil {
				fmt.Print(marsherr)
			}
			deleteReceipt = *val.ReceiptHandle
			// fmt.Println(val.MessageAttributes)
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
				fmt.Println(err)
			}
			_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set password='%s', last_pass_reset=now() where username='%s' or email='%s';", newPassbytesOfPass, messageData.Username, messageData.Useremail))
			if uperr != nil {
				fmt.Println(uperr)
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
	pagesHandler := func(w http.ResponseWriter, r *http.Request) {

		bs, _ := os.ReadFile("navigation.html")
		navtmple := template.New("Navt")
		tm, _ := navtmple.Parse(string(bs))

		switch r.URL.Path {
		case "/groupchat":
			tmpl := template.Must(template.ParseFiles("groupchat.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		case "/home":
			tmpl := template.Must(template.ParseFiles("index.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		case "/calendar":
			tmpl := template.Must(template.ParseFiles("calendar.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		case "/time-capsule":
			tmpl := template.Must(template.ParseFiles("timecapsule.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		case "/bugreport":
			tmpl := template.Must(template.ParseFiles("bugreport.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		case "/games/simple-shades":
			tmpl := template.Must(template.ParseFiles("simpleshades.html"))
			tmpl.Execute(w, nil)
		case "/games/stackerz":
			tmpl := template.Must(template.ParseFiles("stackerz.html"))
			tmpl.Execute(w, nil)
		case "/games/catchit":
			tmpl := template.Must(template.ParseFiles("catchit.html"))
			tmpl.Execute(w, nil)
		case "/admin-dashboard":
			tmpl := template.Must(template.ParseFiles("admindash.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		default:
			tmpl := template.Must(template.ParseFiles("index.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		}

	}
	getPostsHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)
		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var reactionBtn string
		if currentUserFromSession < " " {
			currentUserFromSession = "Guest"
		}

		var output *sql.Rows
		if r.URL.Query().Get("page") == "null" {
			output, err = db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key, createdon at time zone (select mytz from tfldata.users where username='%s') from tfldata.posts where title ilike '%s' or description ilike '%s' or author ilike '%s' order by createdon DESC limit 2;", currentUserFromSession, "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%"))
		} else if r.URL.Query().Get("limit") == "current" {
			w.Header().Set("HX-Reswap", "innerHTML")
			output, err = db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key, createdon at time zone (select mytz from tfldata.users where username='%s') from tfldata.posts where id >= %s and (title ilike '%s' or description ilike '%s' or author ilike '%s') order by createdon DESC;", currentUserFromSession, r.URL.Query().Get("page"), "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%"))
		} else {
			output, err = db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key, createdon at time zone (select mytz from tfldata.users where username='%s') from tfldata.posts where id < %s and (title ilike '%s' or description ilike '%s' or author ilike '%s') order by createdon DESC limit 2;", currentUserFromSession, r.URL.Query().Get("page"), "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%"))
		}

		var dataStr string
		if err != nil {
			// log.Fatal(err)
			fmt.Print(err)
		}

		defer output.Close()
		for output.Next() {

			var postrows Postsrow
			var reaction string
			// if err := output.Scan(&postrows.Id, &postrows.Title, &postrows.Description, &postrows.File_name, &postrows.File_type, &postrows.Author); err != nil {
			if err := output.Scan(&postrows.Id, &postrows.Title, &postrows.Description, &postrows.Author, &postrows.Postfileskey, &postrows.Createdon); err != nil {
				activityStr := "get posts handler scanning posts query"
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(nyLoc).Format(time.DateTime), activityStr))

			}

			reactionRow := db.QueryRow(fmt.Sprintf("select reaction from tfldata.reactions where post_id=%d and author='%s';", postrows.Id, currentUserFromSession))
			reactionRow.Scan(&reaction)

			editElement := ""
			reactionEmojiBeforeComment := ""
			if postrows.Author != currentUserFromSession {
				if reaction > " " {
					reactionEmojiBeforeComment = "<div style='align-content: center; margin-right: 1&percnt;;'>" + reaction + "</div>"
					reactionBtn = fmt.Sprintf("&nbsp;&nbsp;<div onclick='addAReaction(%d)'><i class='bi bi-three-dots'></i></div>", postrows.Id)
				} else {
					reactionBtn = fmt.Sprintf("<button class='btn btn-outline-secondary border-0 px-1' style='margin-left: 2&percnt;;' type='button' onclick='addAReaction(%d)'><i class='bi bi-three-dots-vertical'></i></button>", postrows.Id)
				}
			} else {
				reactionBtn = ""
				editElement = fmt.Sprintf("<i style='position: absolute; background-color: gray; border-radius: 13px / 13px; z-index: 13' class='bi bi-three-dots m-1 px-1 editbtnclass' hx-post='/delete-this-post' hx-swap='none' hx-on::after-request='window.location.reload()' hx-vals=\"js:{'deletionID': %d}\" hx-params='not page, limit, token' hx-ext='json-enc' hx-confirm='Delete this post forever? This cannot be undone'></i>", postrows.Id)
			}
			comment := db.QueryRow(fmt.Sprintf("select count(*) from tfldata.comments where post_id='%d';", postrows.Id))
			var commentCount string
			comment.Scan(&commentCount)
			var countOfImg int
			rowCount := db.QueryRow(fmt.Sprintf("select count(*) from tfldata.postfiles where post_files_key='%s';", postrows.Postfileskey))
			rowCount.Scan(&countOfImg)
			var firstImg struct {
				filename string
				filetype string
			}
			firstRow := db.QueryRow(fmt.Sprintf("select file_name, file_type from tfldata.postfiles where post_files_key='%s' order by id asc limit 1;", postrows.Postfileskey))
			firstRow.Scan(&firstImg.filename, &firstImg.filetype)

			/*if strings.Contains(postrows.Title, "'") {
			  postrows.Title = strings.ReplaceAll(postrows.Title, "'", "")
			  fmt.Println(postrows.Title)
			  }*/

			if strings.Contains(firstImg.filetype, "image") {

				if countOfImg > 1 {
					dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(109 109 109 / .34); border-radius: 20px 20px 20px 20px; box-shadow: 5px 4px 9px 3px rgb(0 0 0 / 52&percnt;);'>%s<img class='img-fluid' id='%s' src='https://%s/posts/images/%s' alt='%s' style='border-radius: 18px 18px;' alt='default' /><p class='createdontime' style='margin-bottom: -6%s; margin-left: 78%s; text-decoration: underline; color: #4e4c4c;'>%s</p><div class='postarrows' style='display: flex; justify-content: space-around;'><i onclick='nextLeftImage(`%s`)' class='bi bi-arrow-90deg-left'></i><i onclick='nextRightImage(`%s`)' class='bi bi-arrow-90deg-right'></i></div><div id='%d' class='card-body' style='text-align: left; padding-left: 1&percnt;;'><b>%s</b><br/><p style='margin-bottom: .2rem'>%s</p><p style='margin-bottom: .2rem' class='card-text'>%s</p><div style='display: flex; justify-content: end'>%s<button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' class='btn btn-primary' hx-swap='innerHTML' style='margin-bottom: -.1rem'>Comments (%s)</button>%s</div></div></div>", editElement, postrows.Postfileskey, cfdistro, firstImg.filename, firstImg.filename, "%", "%", strings.Split(postrows.Createdon, "T")[0], postrows.Postfileskey, postrows.Postfileskey, postrows.Id, postrows.Author, postrows.Title, postrows.Description, reactionEmojiBeforeComment, postrows.Id, postrows.Id, commentCount, reactionBtn)
				} else if countOfImg == 1 {
					dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(109 109 109 / .34); border-radius: 20px 20px 20px 20px; box-shadow: 5px 4px 9px 3px rgb(0 0 0 / 52&percnt;);'>%s<img class='img-fluid' id='%s' src='https://%s/posts/images/%s' alt='%s' style='border-radius: 18px 18px;' alt='default' /><p class='createdontime' style='margin-bottom: -6%s; margin-left: 78%s; text-decoration: underline; color: #4e4c4c;'>%s</p><div id='%d' class='card-body' style='text-align: left; padding-left: 1&percnt;;'><b>%s</b><br/><p style='margin-bottom: .2rem'>%s</p><p style='margin-bottom: .2rem' class='card-text'>%s</p><div style='display: flex; justify-content: end'>%s<button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary' style='margin-bottom: -.1rem'>Comments (%s)</button>%s</div></div></div>", editElement, postrows.Postfileskey, cfdistro, firstImg.filename, firstImg.filename, "%", "%", strings.Split(postrows.Createdon, "T")[0], postrows.Id, postrows.Author, postrows.Title, postrows.Description, reactionEmojiBeforeComment, postrows.Id, postrows.Id, commentCount, reactionBtn)
				}

			} else {

				if countOfImg > 1 {
					dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(109 109 109 / .34); border-radius: 20px 20px 20px 20px; box-shadow: 5px 4px 9px 3px rgb(0 0 0 / 52&percnt;);'>%s<video style='border-radius: 18px 18px; z-index: 4;' muted playsinline controls preload='auto' id='%s'><source src='https://%s/posts/videos/%s'></video><p class='createdontime' style='margin-bottom: -6%s; margin-left: 78%s;text-decoration: underline;color: #4e4c4c;'>%s</p><div class='postarrows' style='display: flex; justify-content: space-around;'><i onclick='nextLeftImage(`%s`)' class='bi bi-arrow-90deg-left'></i><i onclick='nextRightImage(`%s`)' class='bi bi-arrow-90deg-right'></i></div><div id='%d' class='card-body' style='text-align: left; padding-left: 1&percnt;;'><b>%s</b><br/><p style='margin-bottom: .2rem'>%s</p><p style='margin-bottom: .2rem' class='card-text'>%s</p><div style='display: flex; justify-content: end'>%s<button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary' style='margin-bottom: -.1rem'>Comments (%s)</button>%s</div></div></div>", editElement, postrows.Postfileskey, cfdistro, firstImg.filename, "%", "%", strings.Split(postrows.Createdon, "T")[0], postrows.Postfileskey, postrows.Postfileskey, postrows.Id, postrows.Author, postrows.Title, postrows.Description, reactionEmojiBeforeComment, postrows.Id, postrows.Id, commentCount, reactionBtn)
				} else if countOfImg == 1 {
					dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(109 109 109 / .34); border-radius: 20px 20px 20px 20px; box-shadow: 5px 4px 9px 3px rgb(0 0 0 / 52&percnt;);'>%s<video style='border-radius: 18px 18px; z-index: 4;' muted playsinline controls preload='auto' id='%s'><source src='https://%s/posts/videos/%s'></video><p class='createdontime' style='margin-bottom: -6%s; margin-left: 78%s;text-decoration: underline;color: #4e4c4c;'>%s</p><div id='%d' class='card-body' style='text-align: left; padding-left: 1&percnt;;'><b>%s</b><br/><p style='margin-bottom: .2rem'>%s</p><p style='margin-bottom: .2rem' class='card-text'>%s</p><div style='display: flex; justify-content: end'>%s<button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary' style='margin-bottom: -.1rem'>Comments (%s)</button>%s</div></div></div>", editElement, postrows.Postfileskey, cfdistro, firstImg.filename, "%", "%", strings.Split(postrows.Createdon, "T")[0], postrows.Id, postrows.Author, postrows.Title, postrows.Description, reactionEmojiBeforeComment, postrows.Id, postrows.Id, commentCount, reactionBtn)
				}
			}
			postTmpl, tmerr = template.New("tem").Parse(dataStr)
			if tmerr != nil {
				activityStr := "posts handler postTmpl err"
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", tmerr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			}
			postTmpl.Execute(w, nil)

		}

	}
	deleteThisPostHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithDefaultRegion("us-east-1"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awskey, awskeysecret, "")),
		)

		if err != nil {
			w.Write([]byte("<p>This is a delete issue. Please create an issue on the bug report page</p>"))
			return
		}

		client := s3.NewFromConfig(cfg)

		bs, _ := io.ReadAll(r.Body)
		type postBody struct {
			PostID int `json:"deletionID"`
		}
		var postData postBody
		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			fmt.Println(marsherr)
		}
		type workObj struct {
			Filename string
			Filetype string
			Pfilesid int
		}

		output, outerr := db.Query(fmt.Sprintf("select pf.id,pf.file_name,pf.file_type from tfldata.posts as p join tfldata.postfiles as pf on pf.post_files_key = p.post_files_key where p.id=%d;", postData.PostID))
		if outerr != nil {
			fmt.Println(outerr)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("<p>Please report this error at the bugreport page. Title the error with delete post issue</p>"))
			return
		}
		defer output.Close()
		for output.Next() {
			var workData workObj
			output.Scan(&workData.Pfilesid, &workData.Filename, &workData.Filetype)

			if strings.Contains(workData.Filetype, "image") {
				_, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
					Bucket: aws.String(s3Domain),
					Key:    aws.String("posts/images/" + workData.Filename),
				})

				if err != nil {
					fmt.Println("error on image delete")
					fmt.Println(err.Error())
				}
			} else {
				_, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
					Bucket: aws.String(s3Domain),
					Key:    aws.String("posts/videos/" + workData.Filename),
				})

				if err != nil {
					fmt.Println("error on video delete")
					fmt.Println(err.Error())
				}
			}

			db.Exec(fmt.Sprintf("delete from tfldata.postfiles where id=%d", workData.Pfilesid))
		}

		_, delerr := db.Exec(fmt.Sprintf("delete from tfldata.posts where id=%d", postData.PostID))
		if delerr != nil {
			fmt.Println(delerr)
		}
	}

	createPostHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		postFilesKey := uuid.NewString()

		parseerr := r.ParseMultipartForm(10 << 20)
		if parseerr != nil {
			// handle error
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('memory error multi file upload %s');", err))
		}
		// upload, filename, errfile := r.FormFile("file_name")

		//for _, fh := range r.MultipartForm.File["file_name"] {
		for i := 0; i < len(r.MultipartForm.File["file_name"]); i++ {

			fh := r.MultipartForm.File["file_name"][i]
			f, err := fh.Open()
			if err != nil {
				activityStr := fmt.Sprintf("Open multipart file in createPostHandler - %s", currentUserFromSession)
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			tmpFileName := fh.Filename

			var countIfExists int16
			countIfExistsOut := db.QueryRow(fmt.Sprintf("select count(*) from tfldata.postfiles where file_name = '%s';", fh.Filename))

			countIfExistsOut.Scan(&countIfExists)

			if countIfExists > 0 {
				tmpFileName = strings.ReplaceAll(strings.ReplaceAll(time.Now().Format(time.DateTime), " ", "_"), ":", "") + "_" + tmpFileName
				fh.Filename = tmpFileName
			} else {
				tmpFileName = fh.Filename
			}

			if len(tmpFileName) > 55 {
				fh.Filename = tmpFileName[len(tmpFileName)-35:]
			}

			fileContents := make([]byte, fh.Size)

			f.Read(fileContents)

			filetype := http.DetectContentType(fileContents)

			f.Seek(0, 0)

			uploadFileToS3(f, tmpFileName, db, filetype)

			_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.postfiles(\"file_name\", \"file_type\", \"post_files_key\") values('%s', '%s', '%s');", fh.Filename, filetype, postFilesKey))

			if errinsert != nil {
				activityStr := fmt.Sprintf("insert into postfiles table createPostHander - %s", currentUserFromSession)
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errinsert, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			}

			defer f.Close()
		}
		_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"author\", \"post_files_key\", \"createdon\") values(E'%s', E'%s', '%s', '%s', now());", replacer.Replace(r.PostFormValue("title")), replacer.Replace(r.PostFormValue("description")), currentUserFromSession, postFilesKey))

		if errinsert != nil {
			activityStr := "insert into posts table createPostHandler"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errinsert, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var chatMessageNotificationOpts notificationOpts
		chatMessageNotificationOpts.extraPayloadKey = "post"
		chatMessageNotificationOpts.extraPayloadVal = "posts"
		chatMessageNotificationOpts.notificationPage = "posts"

		chatMessageNotificationOpts.notificationTitle = fmt.Sprintf("%s just made a new post!", currentUserFromSession)
		chatMessageNotificationOpts.notificationBody = strings.ReplaceAll(r.PostFormValue("title"), "\\", "")

		go sendNotificationToAllUsers(db, currentUserFromSession, fb_message_client, &chatMessageNotificationOpts)

	}
	createPostReactionHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		type postBody struct {
			Username       string `json:"username"`
			ReactionToPost string `json:"emoji"`
			Postid         int    `json:"selectedPostId"`
		}
		var postData postBody
		bs, _ := io.ReadAll(r.Body)
		marsherr := json.Unmarshal(bs, &postData)
		if marsherr != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s');", marsherr, time.Now().In(nyLoc).Format(time.DateTime)))
			return
		}
		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.reactions(\"post_id\", \"author\", \"reaction\") values(%d, '%s', '%s') on conflict(post_id,author) do update set reaction='%s';", postData.Postid, postData.Username, postData.ReactionToPost, postData.ReactionToPost))
		if inserr != nil {
			activityStr := fmt.Sprintf("insert into reactions createPostReactionHandler - %s", currentUserFromSession)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			w.WriteHeader(http.StatusBadRequest)
		}

	}
	updatePChatReactionHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s');", marsherr, time.Now().In(nyLoc).Format(time.DateTime)))
			return
		}
		var chatreactionfromDB sql.NullString
		curEmojRow := db.QueryRow(fmt.Sprintf("select reaction from tfldata.pchat where id='%s'", postData.Chatid))
		curEmojRow.Scan(&chatreactionfromDB)
		if !chatreactionfromDB.Valid {
			chatreactionfromDB.String = ""
		}

		if chatreactionfromDB.String != postData.ReactionToPost {
			_, uperr := db.Exec(fmt.Sprintf("update tfldata.pchat set reaction = '%s' where id = '%s';", postData.ReactionToPost, postData.Chatid))
			if uperr != nil {
				activityStr := "updating pchat reaction"
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,106),substr('%s',0,105),now());", uperr.Error(), activityStr))
			}
		} else {
			db.Exec(fmt.Sprintf("update tfldata.pchat set reaction = null where id = '%s';", postData.Chatid))
		}

	}
	getCurrentPChatReactionHandler := func(w http.ResponseWriter, r *http.Request) {
		var reactionStr sql.NullString
		reactionRow := db.QueryRow(fmt.Sprintf("select reaction from tfldata.pchat where id='%s';", r.URL.Query().Get("chatid")))
		reactionRow.Scan(&reactionStr)
		if reactionStr.Valid {
			w.Write([]byte(reactionStr.String))
		} else {
			return
		}
	}

	getSelectedPostsComments := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		type postComment struct {
			Comment string
			Author  string
			Pfpname string
		}

		//var commentTmpl *template.Template

		output, err := db.Query(fmt.Sprintf("select c.comment, substr(c.author, 0, 14), u.pfp_name from tfldata.comments as c join tfldata.users as u on c.author = u.username where c.post_id='%s'::integer order by c.id asc;", r.URL.Query().Get("post-id")))

		var dataStr string
		if err != nil {
			activityStr := fmt.Sprintf("getSelectedPostsCommentsHandler select query - %s", currentUserFromSession)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}

		defer output.Close()

		for output.Next() {
			var posts postComment

			if err := output.Scan(&posts.Comment, &posts.Author, &posts.Pfpname); err != nil || len(posts.Pfpname) == 0 {

				posts.Pfpname = "assets/32x32/ZCAN2301 The Family Loop Favicon_B_32 x 32.jpg"
				activityStr := fmt.Sprintf("getSelectedPostsCommentsHandler scan err - %s", currentUserFromSession)
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(nyLoc).Format(time.DateTime), activityStr))

			} else {
				posts.Pfpname = "https://" + cfdistro + "/pfp/" + posts.Pfpname
			}

			dataStr = "<div class='row'><p style='display: flex; align-items: center; padding-right: 0%;' class='m-1 col-7'>" + posts.Comment + "</p><div style='align-items: center; position: relative; display: flex; padding-left: 0%; left: 1%;' class='col my-5'><b style='position: absolute; bottom: 5%'>" + posts.Author + "</b><img width='30px' class='my-1' style='margin-left: 1%; position: absolute; right: 20%; border-style: solid; border-radius: 13px / 13px; box-shadow: 3px 3px 5px; border-width: thin; top: 5%;' src='" + posts.Pfpname + "' alt='tfl pfp' /></div></div>"

			w.Write([]byte(dataStr))
		}

	}
	createEventCommentHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s');", errmarsh, time.Now().In(nyLoc).Format(time.DateTime)))
		}

		_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.comments(\"comment\", \"event_id\", \"author\") values(E'%s', '%d', '%s');", replacer.Replace(postData.Eventcomment), postData.CommentSelectedEventId, currentUserFromSession))
		if inserterr != nil {
			activityStr := fmt.Sprintf("insert into comments table createEventComment - %s", currentUserFromSession)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserterr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}

		dataStr := "<p class='p-2'>" + postData.Eventcomment + " - " + currentUserFromSession + "</p>"

		commentTmpl, _ := template.New("com").Parse(dataStr)

		commentTmpl.Execute(w, nil)

	}
	getSelectedEventsComments := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var commentTmpl *template.Template

		output, err := db.Query(fmt.Sprintf("select comment, author from tfldata.comments where event_id='%s'::integer order by event_id desc;", r.URL.Query().Get("commentSelectedEventID")))

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
	createCommentHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		bs, _ := io.ReadAll(r.Body)
		type postBody struct {
			Comment        string   `json:"comment"`
			SelectedPostId int      `json:"selectedPostId"`
			Taggedusers    []string `json:"taggedUser"`
		}

		var postData postBody
		errmarsh := json.Unmarshal(bs, &postData)
		if errmarsh != nil {
			fmt.Println(errmarsh)
		}

		_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.comments(\"comment\", \"post_id\", \"author\") values(E'%s', '%d', (select username from tfldata.users where username='%s'));", replacer.Replace(postData.Comment), postData.SelectedPostId, currentUserFromSession))
		if inserterr != nil {
			fmt.Println(inserterr)
		}
		var author string
		var pfpname string
		row := db.QueryRow(fmt.Sprintf("select username, pfp_name from tfldata.users where username='%s';", currentUserFromSession))
		userscnerr := row.Scan(&author, &pfpname)

		if userscnerr != nil || len(pfpname) == 0 {
			pfpname = "assets/32x32/ZCAN2301 The Family Loop Favicon_B_32 x 32.jpg"
		} else {
			pfpname = "https://" + cfdistro + "/pfp/" + pfpname
		}
		dataStr := "<div class='row'><p style='display: flex; align-items: center; padding-right: 0%;' class='m-1 col-7'>" + postData.Comment + "</p><div style='align-items: center; position: relative; display: flex; padding-left: 0%; left: 1%;' class='col my-5'><b style='position: absolute; bottom: 5%'>" + author + "</b><img width='30px' class='my-1' style='margin-left: 1%; position: absolute; right: 20%; border-style: solid; border-radius: 13px / 13px; box-shadow: 3px 3px 5px; border-width: thin; top: 5%;' src='" + pfpname + "' alt='tfl pfp' /></div></div>"

		commentTmpl, err := template.New("com").Parse(dataStr)
		if err != nil {
			fmt.Println(err)
		}
		commentTmpl.Execute(w, nil)
		go func() {
			var fcmToken string
			fcmrow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username = (select author from tfldata.posts where id=%d) and username != (select username from tfldata.users where username='%s') and fcm_registration_id is not null;", postData.SelectedPostId, currentUserFromSession))
			scnerr := fcmrow.Scan(&fcmToken)
			if scnerr == nil {

				//fb_message_client, _ := app.Messaging(context.TODO())
				typePayload := make(map[string]string)
				typePayload["type"] = "posts"
				sentRes, sendErr := fb_message_client.Send(context.TODO(), &messaging.Message{
					Token: fcmToken,
					Data:  typePayload,
					Notification: &messaging.Notification{
						Title:    author + " commented on your post!",
						Body:     "\"" + postData.Comment + "\"",
						ImageURL: "/assets/icon-180x180.jpg",
					},

					Webpush: &messaging.WebpushConfig{
						Notification: &messaging.WebpushNotification{
							Title: author + " commented on your post!",
							Body:  "\"" + postData.Comment + "\"",
							Data:  typePayload,
							Image: "/assets/icon-180x180.jpg",
							Icon:  "/assets/icon-96x96.jpg",
							Actions: []*messaging.WebpushNotificationAction{
								{
									Action: typePayload["type"],
									Title:  author + " commented on your post!",
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
				db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sentRes, time.Now().In(nyLoc).Local().Format(time.DateTime)))
			}
			if len(postData.Taggedusers) > 0 {
				var usersPost string
				row := db.QueryRow(fmt.Sprintf("select author from tfldata.posts where id=%d", postData.SelectedPostId))
				row.Scan(&usersPost)

				for _, userTagged := range postData.Taggedusers {
					var fcmToken string
					fcmrow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username = '%s' and username != (select username from tfldata.users where username='%s') and fcm_registration_id is not null;", userTagged, currentUserFromSession))
					scnerr := fcmrow.Scan(&fcmToken)
					if scnerr == nil {

						//fb_message_client, _ := app.Messaging(context.TODO())
						typePayload := make(map[string]string)
						typePayload["type"] = "posts"
						sentRes, sendErr := fb_message_client.Send(context.TODO(), &messaging.Message{
							Token: fcmToken,
							Notification: &messaging.Notification{
								Title:    currentUserFromSession + " tagged you on " + usersPost + "'s post",
								Body:     "\"" + postData.Comment + "\"",
								ImageURL: "/assets/icon-180x180.jpg",
							},

							Webpush: &messaging.WebpushConfig{
								Notification: &messaging.WebpushNotification{
									Title: currentUserFromSession + " tagged you on " + usersPost + "'s post",
									Body:  "\"" + postData.Comment + "\"",
									Data:  typePayload,
									Image: "/assets/icon-180x180.jpg",
									Icon:  "/assets/icon-96x96.jpg",
								},
							},
						})
						if sendErr != nil {
							fmt.Print(sendErr)
						}
						db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sentRes, time.Now().In(nyLoc).Local().Format(time.DateTime)))
					}

				}
			}
		}()
	}
	getEventsHandler := func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		output, err := db.Query("select start_date, event_owner, event_details, event_title, id, end_date from tfldata.calendar;")
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
	getPostsReactionsHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		output, rowerr := db.Query(fmt.Sprintf("select author, reaction from tfldata.reactions where post_id='%s' and author != '%s';", r.URL.Query().Get("selectedPostId"), r.URL.Query().Get("username")))
		if rowerr != nil {
			db.Exec("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", rowerr, time.Now().In(nyLoc).Local().Format(time.DateTime))
		}
		var outReaction string
		var outAuthor string
		defer output.Close()
		for output.Next() {
			scnerr := output.Scan(&outAuthor, &outReaction)
			if scnerr != nil {
				fmt.Println(scnerr)
			}
			w.Write([]byte("&nbsp;&nbsp;" + outAuthor + "&nbsp; - " + outReaction + "<br/>"))
		}
	}
	createEventHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
			_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.calendar(\"start_date\", \"event_owner\", \"event_details\", \"event_title\") values('%s', '%s', E'%s', E'%s');", postData.Startdate, currentUserFromSession, replacer.Replace(postData.Eventdetails), replacer.Replace(postData.Eventtitle)))
			if inserterr != nil {
				fmt.Println(inserterr)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		} else {

			_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.calendar(\"start_date\", \"event_owner\", \"event_details\", \"event_title\", \"end_date\") values('%s', '%s', E'%s', E'%s', '%s');", postData.Startdate, currentUserFromSession, replacer.Replace(postData.Eventdetails), replacer.Replace(postData.Eventtitle), postData.Enddate))
			if inserterr != nil {
				fmt.Println(inserterr)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		var chatMessageNotificationOpts notificationOpts
		// You can use the below key to add onclick features to the notification
		chatMessageNotificationOpts.extraPayloadKey = "calendardata"
		chatMessageNotificationOpts.extraPayloadVal = "calendar"
		chatMessageNotificationOpts.notificationPage = "calendar"
		chatMessageNotificationOpts.notificationTitle = "New event on: " + postData.Startdate
		chatMessageNotificationOpts.notificationBody = strings.ReplaceAll(postData.Eventtitle, "\\", "")

		go sendNotificationToAllUsers(db, currentUserFromSession, fb_message_client, &chatMessageNotificationOpts)

	}
	deleteEventHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		db.Exec(fmt.Sprintf("delete from tfldata.calendar where id=%d;", postData.Eventid))
		db.Exec(fmt.Sprintf("delete from tfldata.calendar_rsvp where event_id=%d;", postData.Eventid))
		db.Exec(fmt.Sprintf("delete from tfldata.comments where event_id=%d;", postData.Eventid))
	}
	updateRSVPForEventHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.calendar_rsvp(\"username\",\"event_id\",\"status\") values('%s',%d,'%s') on conflict(username,event_id) do update set status='%s';", postData.Username, postData.Eventid, postData.Status, postData.Status))
		if inserr != nil {
			db.Exec("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(nyLoc).Local().Format(time.DateTime))
			w.WriteHeader(http.StatusBadRequest)
		}

		var fcmToken string
		fcmrow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username = (select event_owner from tfldata.calendar where id=%d);", postData.Eventid))
		scnerr := fcmrow.Scan(&fcmToken)
		if scnerr != nil {

			if scnerr.Error() == "sql: no rows in result set" {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			db.Exec("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", scnerr, time.Now().In(nyLoc).Local().Format(time.DateTime))
			w.WriteHeader(http.StatusBadRequest)
			return
		} else {

			fb_message_client, _ := app.Messaging(context.TODO())
			typePayload := make(map[string]string)
			typePayload["type"] = "event"
			sentRes, sendErr := fb_message_client.Send(context.TODO(), &messaging.Message{
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
			db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\") values('%s');", sentRes))
		}

	}
	getEventRSVPHandler := func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var status string
		row := db.QueryRow(fmt.Sprintf("select status from tfldata.calendar_rsvp where username='%s' and event_id='%s';", r.URL.Query().Get("username"), r.URL.Query().Get("event_id")))
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
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var status string
		var username string
		output, outerr := db.Query(fmt.Sprintf("select username, status from tfldata.calendar_rsvp where event_id='%s';", r.URL.Query().Get("event_id")))

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
	createGroupChatMessageHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		chatMessage := replacer.Replace(r.PostFormValue("gchatmessage"))

		listOfUsersTagged := strings.Split(r.PostFormValue("taggedUser"), ",")

		threadVal := r.PostFormValue("threadval")
		if threadVal == "" {
			threadVal = "main thread"
		} else if strings.ToLower(threadVal) == "posts" || strings.ToLower(threadVal) == "calendar" {
			w.WriteHeader(http.StatusConflict)
			return
		}

		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.gchat(\"chat\", \"author\", \"createdon\", \"thread\") values(E'%s', '%s', now(), '%s');", chatMessage, currentUserFromSession, threadVal))
		if inserr != nil {
			fmt.Println("error here: " + inserr.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, ttbleerr := db.Exec(fmt.Sprintf("insert into tfldata.threads(\"thread\", \"threadauthor\", \"createdon\") values(E'%s', '%s', now());", threadVal, currentUserFromSession))
		if ttbleerr != nil {
			if strings.Contains(ttbleerr.Error(), "duplicate key") {
				fmt.Println("duplicate thread error can be ignored")
				s := make([]string, 0)
				s = append(s, "insert into tfldata.users_to_threads(username) select distinct(username) from tfldata.users;")
				s = append(s, fmt.Sprintf("update tfldata.users_to_threads set is_subscribed=true, thread='%s' where is_subscribed is null and thread is null;", threadVal))
				upAndInsUTT, txnerr := db.Begin()
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
		var chatMessageNotificationOpts notificationOpts
		chatMessageNotificationOpts.extraPayloadKey = "thread"
		chatMessageNotificationOpts.extraPayloadVal = threadVal
		chatMessageNotificationOpts.notificationPage = "groupchat"
		chatMessageNotificationOpts.notificationTitle = "message in: " + threadVal
		chatMessageNotificationOpts.notificationBody = strings.ReplaceAll(chatMessage, "\\", "")

		var singleUserChatMessageNotificationOpts notificationOpts
		singleUserChatMessageNotificationOpts.extraPayloadKey = "thread"
		singleUserChatMessageNotificationOpts.extraPayloadVal = threadVal
		singleUserChatMessageNotificationOpts.notificationPage = "groupchat"
		singleUserChatMessageNotificationOpts.notificationTitle = currentUserFromSession + " just tagged you in : " + threadVal
		singleUserChatMessageNotificationOpts.notificationBody = chatMessage

		go sendNotificationToAllUsers(db, currentUserFromSession, fb_message_client, &chatMessageNotificationOpts)

		if len(listOfUsersTagged[0]) > 0 {
			for _, taggedUser := range listOfUsersTagged {
				var fcmToken string
				row := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username='%s';", taggedUser))

				scnerr := row.Scan(&fcmToken)
				if scnerr == nil {
					go sendNotificationToSingleUser(db, fb_message_client, fcmToken, singleUserChatMessageNotificationOpts)
				}

			}
		}
		w.Header().Set("HX-Trigger", "success-send")

	}
	delThreadHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		delThreadDataTxn, txnerr := db.Begin()
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
	changeGchatOrderOptHandler := func(w http.ResponseWriter, r *http.Request) {
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
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set gchat_order_option='%t' where username='%s';", postData.Option, postData.Username))
		if uperr != nil {
			activityStr := fmt.Sprintf("%s tried to update gchat_order_option", postData.Username)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}

	}
	getGroupChatMessagesHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		output, err := db.Query(fmt.Sprintf("select id, chat, author, createdon at time zone (select mytz from tfldata.users where username='%s') from (select * from tfldata.gchat where thread='%s' order by createdon DESC limit %d) as tmp order by createdon %s;", currentUserFromSession, r.URL.Query().Get("threadval"), limitVal, orderAscOrDesc))

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

			row := db.QueryRow(fmt.Sprintf("select pfp_name from tfldata.users where username='%s';", author))
			pfpscnerr := row.Scan(&pfpImg)
			if pfpscnerr != nil {
				pfpImg = "assets/96x96/ZCAN2301 The Family Loop Favicon_W_96 x 96.png"
			} else {
				pfpImg = "https://" + cfdistro + "/pfp/" + pfpImg
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
			dataStr := ""
			if author == currentUserFromSession {
				dataStr = "<div class='container gchatmessagecardme' style='width: 95&percnt;;'><div class='row'><b class='col-2 px-1'>" + author + "</b><div class='row'><img style='width: 15%; position: sticky;' class='col-2 px-2 my-1' src='" + pfpImg + "' alt='tfl pfp' /></div><p class='col-10' style='position: relative; left: 13%; margin-bottom: 1%; margin-top: -15%; overflow-wrap: anywhere; padding-right: 0%;'>" + message + "</p></div><div class='row'><p class='col' style='margin-left: 60%; font-size: smaller; margin-bottom: 0%'>" + createdat.Format(formatCreatedatTime) + editDelBtn + "</p></div></div>"
			} else {
				dataStr = "<div class='container gchatmessagecardfrom' style='width: 95&percnt;;'><div class='row'><b class='col-2 px-1'>" + author + "</b><div class='row'><img style='width: 15%; position: sticky;' class='col-2 px-2 my-1' src='" + pfpImg + "' alt='tfl pfp' /></div><p class='col-10' style='position: relative; left: 13%; margin-bottom: 1%; margin-top: -15%; overflow-wrap: anywhere; padding-right: 0%;'>" + message + "</p></div><div class='row'><p class='col' style='margin-left: 60%; font-size: smaller; margin-bottom: 0%'>" + createdat.Format(formatCreatedatTime) + editDelBtn + "</p></div></div>"
			}
			chattmp, tmperr := template.New("gchat").Parse(dataStr)
			if tmperr != nil {
				fmt.Println(tmperr)
			}
			chattmp.Execute(w, nil)

		}
	}
	getPrivateChatMessagesHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		output, err := db.Query(fmt.Sprintf("select id,message,from_user,to_user,reaction,createdon at time zone (select mytz from tfldata.users where username='%s') from (select * from tfldata.pchat where (from_user='%s' and to_user='%s') or (from_user='%s' and to_user='%s') order by createdon DESC limit %d) as tmp order by createdon %s;", currentUserFromSession, r.URL.Query().Get("userToSendTo"), currentUserFromSession, currentUserFromSession, r.URL.Query().Get("userToSendTo"), limitVal, orderAscOrDesc))

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

			pfprow := db.QueryRow(fmt.Sprintf("select pfp_name from tfldata.users where username='%s';", pChatRow.fromUser))

			pfpscnerr := pfprow.Scan(&pfpimg)
			if pfpscnerr != nil {
				pfpimg = "assets/96x96/ZCAN2301 The Family Loop Favicon_W_96 x 96.png"
			} else {
				pfpimg = "https://" + cfdistro + "/pfp/" + pfpimg
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

			if pChatRow.toUser == currentUserFromSession {
				dataStr = "<div id='pchatid_" + fmt.Sprint(pChatRow.id) + "'class='container gchatmessagecardfrom' style='width: 95&percnt;;'><div class='row'><b class='col-2 px-1'>" + pChatRow.fromUser + "</b><div class='row'><img style='width: 15%; position: sticky;' class='col-2 px-2 my-1' src='" + pfpimg + "' alt='tfl pfp' /></div><p class='col-10' style='position: relative; left: 13%; margin-bottom: 1%; margin-top: -15%; overflow-wrap: anywhere; padding-right: 0%;'>" + pChatRow.chatMessage + "</p></div><div class='row'><div class='col' style='position: relative; margin-right: 0&percnt;; width: auto; display: flex; justify-content: flex-start' id='reactionid_" + fmt.Sprint(pChatRow.id) + "'>" + pChatRow.reaction.String + "</div><p class='col' style='margin-left: 60%; font-size: smaller; margin-bottom: 0%'>" + pChatRow.createdOn.Format(pChatRow.formatCreatedOnTime) + editDelBtn + "</p></div></div>"
			} else {
				dataStr = "<div class='container gchatmessagecardme' style='width: 95&percnt;;'><div class='row'><div class='row'><b class='col-2 px-1'>" + pChatRow.fromUser + "</b><div class='row'><img style='width: 15%; position: sticky;' class='col-2 px-2 my-1' src='" + pfpimg + "' alt='tfl pfp' /></div><p class='col-10' style='position: relative; left: 13%; margin-bottom: 1%; margin-top: -15%; overflow-wrap: anywhere; padding-right: 0%;'>" + pChatRow.chatMessage + "</p></div><div class='col' style='position: relative; margin-right: 0&percnt;; width: auto; display: flex; justify-content: flex-start' id='reactionid_" + fmt.Sprint(pChatRow.id) + "'>" + pChatRow.reaction.String + "</div><p class='col' style='margin-left: 60%; font-size: smaller; margin-bottom: 0%'>" + pChatRow.createdOn.Format(pChatRow.formatCreatedOnTime) + editDelBtn + "</p></div></div>"
			}
			chattmp, tmperr := template.New("pchat").Parse(dataStr)
			if tmperr != nil {
				fmt.Println(tmperr)
			}
			chattmp.Execute(w, nil)

		}
	}
	createPrivatePChatMessageHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		userTo := r.PostFormValue("user_to")
		message := r.PostFormValue("privatechatmessage")

		_, dbinserr := db.Exec(fmt.Sprintf("insert into tfldata.pchat(message, from_user, to_user, createdon) values (substr('%s',0,420), '%s', '%s', now());", message, currentUserFromSession, userTo))
		if dbinserr != nil {
			activityStr := "insert into pchat table"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, createdon, activity) values (substr('%s',0,106), now(), substr('%s',0,105));", dbinserr.Error(), activityStr))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var fcmTokenToUser sql.NullString

		fcmRes := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username = '%s';", userTo))

		fcmRes.Scan(&fcmTokenToUser)

		if fcmTokenToUser.Valid {

			var chatMessageNotificationOpts notificationOpts
			chatMessageNotificationOpts.extraPayloadKey = "direct"
			chatMessageNotificationOpts.extraPayloadVal = "groupchat"
			chatMessageNotificationOpts.notificationPage = "groupchat"
			chatMessageNotificationOpts.notificationTitle = "message from: " + currentUserFromSession
			chatMessageNotificationOpts.notificationBody = strings.ReplaceAll(message, "\\", "")

			go sendNotificationToSingleUser(db, fb_message_client, fcmTokenToUser.String, chatMessageNotificationOpts)
		}
	}
	updateLastViewedPChatHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),substr('%s',0,106), now());", marsherr.Error(), activityStr))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set last_viewed_pchat = '%s' where username = '%s';", postData.LastViewed, currentUserFromSession))
		if uperr != nil {
			activityStr := "update error updatelastviewed function"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),substr('%s',0,106), now());", uperr.Error(), activityStr))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	updateLastViewedThreadHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),substr('%s',0,106), now());", marsherr.Error(), activityStr))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set last_viewed_gchat = '%s' where username = '%s';", postData.LastViewed, currentUserFromSession))
		if uperr != nil {
			activityStr := "update error updatelastviewedthread function"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values(substr('%s',0,105),substr('%s',0,106), now());", uperr.Error(), activityStr))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	getUsernamesToTagHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		searchOutput, searchErr := db.Query("select username from tfldata.users where username like '%" + r.URL.Query().Get("user") + "%';")
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
	getPostImagesHandler := func(w http.ResponseWriter, r *http.Request) {

		var imgList []string
		rows, err := db.Query(fmt.Sprintf("select file_name from tfldata.postfiles where post_files_key='%s' order by id asc;", r.URL.Query().Get("id")))
		if err != nil {
			fmt.Println(err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var filename string
			rows.Scan(&filename)
			imgList = append(imgList, filename)

		}
		data, jsonerr := json.Marshal(&imgList)
		if jsonerr != nil {
			fmt.Println(jsonerr)
		}
		w.Write(data)

	}
	getSubscribedHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)

		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var fcmRegToken string
		fcmRegRow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username='%s';", currentUserFromSession))
		scnerr := fcmRegRow.Scan(&fcmRegToken)

		if scnerr != nil {
			w.WriteHeader(http.StatusAccepted)
			return
			// db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", scnerr, time.Now().In(nyLoc).Local().Format(time.DateTime)))
		}
		w.WriteHeader(http.StatusOK)
	}
	getSessionDataHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)

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
				resp.Orgid = orgId
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

		row := db.QueryRow(fmt.Sprintf("select username, gchat_bg_theme, gchat_order_option, is_admin, pfp_name, fcm_registration_id, last_viewed_pchat, last_viewed_gchat from tfldata.users where username='%s';", currentUserFromSession))
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

		ourSeshStruct.CFDomain = cfdistro

		data, err := json.Marshal(&ourSeshStruct)
		if err != nil {
			fmt.Println(err)
		}

		w.Write(data)
	}

	updatePfpHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/form-data")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		upload, filename, _ := r.FormFile("changepfp")

		username := r.PostFormValue("usernameinput")

		fn := uploadPfpToS3(upload, filename.Filename, r, "changepfp")
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set pfp_name='%s' where username='%s';", fn, username))
		if uperr != nil {
			activityStr := fmt.Sprintf("update table users set pfp_name failed for user %s", currentUserFromSession)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	updateChatThemeHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set gchat_bg_theme='%s' where username='%s';", postData.Theme, postData.Username))
		if uperr != nil {
			activityStr := fmt.Sprintf("updateChatTheme failed for user %s", currentUserFromSession)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}
	}
	deleteSelectedChatHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		_, delerr := db.Exec(fmt.Sprintf("delete from tfldata.gchat where id='%s';", postData.SelectedChatId))
		if delerr != nil {
			activityStr := fmt.Sprintf("%s could not deleteSelectedChat", currentUserFromSession)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", delerr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}
	}
	updateSelectedChatHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.gchat set chat='%s' where id='%s';", postData.ChatMessage, postData.SelectedChatId))
		if uperr != nil {
			activityStr := fmt.Sprintf("%s could not edit the chat message %s", currentUserFromSession, postData.SelectedChatId)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}
	}
	deleteSelectedPChatHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		_, delerr := db.Exec(fmt.Sprintf("delete from tfldata.pchat where id='%s';", postData.SelectedChatId))
		if delerr != nil {
			activityStr := fmt.Sprintf("%s could not deleteSelectedPChat", currentUserFromSession)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", delerr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}
	}
	updateSelectedPChatHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.pchat set message='%s' where id='%s';", postData.ChatMessage, postData.SelectedChatId))
		if uperr != nil {
			activityStr := fmt.Sprintf("%s could not edit the Pchat message %s", currentUserFromSession, postData.SelectedChatId)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}
	}
	getSelectedChatHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var ChatVal string
		row := db.QueryRow(fmt.Sprintf("select chat from tfldata.gchat where id='%s';", r.URL.Query().Get("chatid")))
		row.Scan(&ChatVal)
		marshbs, marsherr := json.Marshal(ChatVal)
		if marsherr != nil {
			fmt.Println(marsherr)
		}
		w.Write(marshbs)
	}
	getSelectedPChatHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var ChatVal string
		row := db.QueryRow(fmt.Sprintf("select message from tfldata.pchat where id='%s';", r.URL.Query().Get("chatid")))
		row.Scan(&ChatVal)
		marshbs, marsherr := json.Marshal(ChatVal)
		if marsherr != nil {
			fmt.Println(marsherr)
		}
		w.Write(marshbs)
	}
	createIssueHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, _ := r.Cookie("session_id")
		var username string
		row := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", c.Value))
		row.Scan(&username)
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
			issueLabel = []string{"enhancement"}

		} else if postData.Label == "bug" {
			issueLabel = []string{"bug"}
		}
		bodyText := fmt.Sprintf("%s on %s page - %s. Orgid: %s", postData.Descdetail[1], postData.Descdetail[0], username, strings.Split(orgId, "_")[0]+strings.Split(orgId, "_")[1][:3])
		issueJson := github.IssueRequest{
			Title:  &postData.Issuetitle,
			Body:   &bodyText,
			Labels: &issueLabel,
		}

		jsonMarshed, errMarsh := json.Marshal(issueJson)
		if errMarsh != nil {
			fmt.Println(errMarsh)
		}

		req, reqerr := http.NewRequest("POST", "https://api.github.com/repos/zanton173/the-family-loop/issues", bytes.NewReader(jsonMarshed))
		if reqerr != nil {
			fmt.Println(reqerr)
		}
		req.Header.Set("Authorization", "token "+ghissuetoken)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
		}
		resp.Body.Close()

	}
	getStackerzLeaderboardHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("leaderboardType") == "family" {
			output, outerr := db.Query("select substr(username,0,14), bonus_points, level from tfldata.stack_leaderboard order by(bonus_points+level) desc limit 20;")
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
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.stack_leaderboard(\"username\", \"bonus_points\", \"level\") values('%s', %d, %d)", postData.Username, postData.BonusPoints, postData.Level))
		if inserr != nil {
			activityStr := fmt.Sprintf("could not update stackerz leaderboard for %s", currentUserFromSession)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}
		coll.InsertOne(context.TODO(), bson.M{"org_id": orgId, "game": "stackerz", "bonus_points": postData.BonusPoints, "level": postData.Level, "username": postData.Username, "createdOn": time.Now()})
	}
	getPersonalCatchitLeaderboardHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		output, outerr := db.Query(fmt.Sprintf("select username, score from tfldata.catchitleaderboard where username='%s' order by score desc limit 20;", currentUserFromSession))
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
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("leaderboardType") == "family" {
			output, outerr := db.Query("select username, score from tfldata.catchitleaderboard order by score desc limit 20;")
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
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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

		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.catchitleaderboard(\"username\", \"score\", \"createdon\") values('%s', '%d', now());", postData.Username, postData.Score))
		if inserr != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		coll.InsertOne(context.TODO(), bson.M{"org_id": orgId, "game": "catchit", "score": postData.Score, "username": postData.Username, "createdOn": time.Now()})

	}
	getLeaderboardHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("leaderboardType") == "family" {
			output, outerr := db.Query("select username, score from tfldata.ss_leaderboard order by score desc limit 20;")
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

		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.ss_leaderboard(\"username\", \"score\") values('%s', '%d');", postData.Username, postData.Score))
		if inserr != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		coll.InsertOne(context.TODO(), bson.M{"org_id": orgId, "game": "simple_shades", "score": postData.Score, "username": postData.Username, "createdOn": time.Now()})

	}
	getOpenThreadsHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		distinctThreadsOutput, queryErr := db.Query("select thread,threadauthor from tfldata.threads order by createdon desc;")
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
	getUsersToChatToHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		distinctUsersOutput, queryErr := db.Query(fmt.Sprintf("select distinct(username) from tfldata.users where username != '%s';", currentUserFromSession))
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
	getUsersSubscribedThreadsHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		output, outerr := db.Query(fmt.Sprintf("select thread, is_subscribed::text from tfldata.users_to_threads where username='%s';", r.URL.Query().Get("username")))
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

	changeUserSubscriptionToThreadHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.users_to_threads(\"username\",\"thread\",\"is_subscribed\") values('%s','%s',%t) on conflict(username,thread) do update set is_subscribed=%t;", postData.User, postData.Thread, postData.CurrentlySubbed, postData.CurrentlySubbed))
		if inserr != nil {
			activityStr := fmt.Sprintf("%s could not update sub settings for thread %s", currentUserFromSession, postData.Thread)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}

	}
	createNewTimeCapsuleHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/form-data")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var expiresOn string
		var curAmountOfStoredCapsules int
		var nameExists string

		searchForName := db.QueryRow(fmt.Sprintf("select tcname from tfldata.timecapsule where tcname='%s' and username='%s' limit 1;", r.PostFormValue("tcName"), currentUserFromSession))

		searchForName.Scan(&nameExists)

		if len(nameExists) > 0 {
			w.WriteHeader(http.StatusNotAcceptable)
			w.Write([]byte("Please use a unique name."))
			return
		}

		row := db.QueryRow(fmt.Sprintf("select count(*) from tfldata.timecapsule where username='%s' and available_on > now();", currentUserFromSession))
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", parseerr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			return
		}
		totalFilesSize := 0
		for _, fh := range r.MultipartForm.File["tcfileinputname"] {

			f, openErr := fh.Open()
			if openErr != nil {
				activityStr := "failed to open multipart file tc create"
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", openErr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
				return
			}

			w1, createerr := zipWriter.Create("timecapsule/" + fh.Filename)
			if createerr != nil {
				activityStr := "Err creating file to place in zip tccreate handler"
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", createerr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
				return
			}
			_, copyerr := io.Copy(w1, f)
			if copyerr != nil {
				activityStr := "Err copying file to zip create tc handler"
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", copyerr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
				return
			}
			totalFilesSize += int(fh.Size / 1024 / 1024)
			/*if err != nil {
				activityStr := fmt.Sprintf("Open multipart file in createtimecapsulehandler - %s", currentUserFromSession)
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
				w.WriteHeader(http.StatusUnsupportedMediaType)
				return
			}*/
			f.Close()
		}
		zipWriter.Close()

		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.timecapsule(\"username\", \"available_on\", \"tcname\", \"tcfilename\", \"createdon\", waspurchased, wasearlyaccesspurchased, yearstostore, wasrequested, wasdownloaded) values('%s', '%s'::date + INTERVAL '2 days', '%s', '%s', '%s', false, false, %d, false, false);", currentUserFromSession, expiresOn, r.PostFormValue("tcName"), tcFileName, curDate, yearsfordb))

		if inserr != nil {
			activityStr := "Failed to add time capsule to DB"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			return
		}

		go uploadTimeCapsuleToS3(tcFile, tcFileName)
	}
	initiateMyTCRestoreHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.timecapsule set wasrequested = true where tcfilename='%s';", postData.Tcfilename))
		if uperr != nil {
			activityStr := "Update tc after requested"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values('%s', '%s', now());", uperr.Error(), activityStr))
			return
		}

		_, reserr := s3Client.RestoreObject(context.TODO(), &s3.RestoreObjectInput{
			Bucket: &s3Domain,
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

		returnobj, returnerr := s3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
			Bucket: &s3Domain,
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
				_, cperr := s3Client.CopyObject(context.TODO(), &s3.CopyObjectInput{
					Bucket:       &s3Domain,
					CopySource:   aws.String(s3Domain + "/timecapsules/" + r.URL.Query().Get("tcfilename")),
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
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		db.Exec(fmt.Sprintf("update tfldata.timecapsule set wasdownloaded = true where tcfilename='%s';", postData.Tcfilename))
	}
	getMyAvailableTimeCapsulesHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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

		output, _ := db.Query(fmt.Sprintf("select tcname, tcfilename, createdon, wasrequested, wasdownloaded from tfldata.timecapsule where username='%s' and available_on <= now() + interval '1 day' and waspurchased = true order by available_on asc;", currentUserFromSession))

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
				showDownLinkDataStr = fmt.Sprintf("<div hx-ext='json-enc' hx-post='/available-tc-was-downloaded' hx-vals='js:{\"tcfilename\": \"%s\"}' hx-swap='none' hx-target='this'><a target='_blank' href='https://%s/timecapsules/restored/%s'>download</a></div>", myTcOut.tcfilename, cfdistro, myTcOut.tcfilename)
			} else {
				showReqStatusDataStr = fmt.Sprintf("<button class='btn' style='border-width: thin; border-color: black; border-radius: 15px / 15px; padding-top: 1&percnt;; padding-bottom: 1&percnt;; box-shadow: 3px 3px 4px;' hx-post='/initiate-tc-req-for-archive-file' hx-ext='json-enc' hx-swap='none' hx-trigger='click' hx-vals='js:{\"tcfilename\": \"%s\"}' hx-on::after-request='initiateRestoreResp(event)'>Get file</button>", myTcOut.tcfilename)
			}

			w.Write([]byte(fmt.Sprintf("<tr><td style='background-color: %s'>%s</td><td style='background-color: %s'>%s</td><td style='background-color: %s; text-align: center'>%s</td><td  style='background-color: %s; text-align: center;'>%s</td></tr>", bgColor, myTcOut.tcname, bgColor, strings.Split(myTcOut.createdon, "T")[0], bgColor, showReqStatusDataStr, bgColor, showDownLinkDataStr)))
			iter++
		}
	}
	getMyNotYetPurchasedTimeCapsulesHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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

		output, _ := db.Query(fmt.Sprintf("select tcname, createdon, available_on, tcfilename from tfldata.timecapsule where username='%s' and waspurchased = false order by available_on asc;", currentUserFromSession))

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

			w.Write([]byte(fmt.Sprintf("<tr><td class='toggleArrows' onclick='openInStore(`%s`, `%s`, `%s`, `b4c9da54-cdd2-b747-a2bf-2db7bb015cd2`, `notyetpurchased`)' style='background-color: %s'>%s&nbsp;&nbsp;<span class='glyphicon glyphicon-new-window'></span></td><td style='background-color: %s; text-align: center'>%s</td><td style='background-color: %s; text-align: center'>%s</td><td  style='background-color: %s; text-align: center; font-size: larger; color: red;' class='toggleArrows' hx-swap='none' hx-post='/delete-my-tc' hx-ext='json-enc' hx-vals='{%s: %s}' hx-confirm='This will delete the time capsule forever and it will be unretrievable. Are you sure you want to continue?'>X</td></tr>", myTcOut.tcfilename, orgId, strings.Split(orgId, "_")[0], bgColor, myTcOut.tcname, bgColor, strings.Split(myTcOut.createdon, "T")[0], bgColor, strings.Split(myTcOut.availableOn, "T")[0], bgColor, "\"myTCName\"", "\""+myTcOut.tcname+"\"")))
			iter++
		}
	}

	getMyPurchasedTimeCapsulesHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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

		output, _ := db.Query(fmt.Sprintf("select tcname, createdon, available_on, tcfilename, wasearlyaccesspurchased from tfldata.timecapsule where username='%s' and available_on %s now() and waspurchased = true order by available_on asc;", currentUserFromSession, r.URL.Query().Get("pastorpresent")))

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
				openinstorestr = fmt.Sprintf("class='toggleArrows' onclick='openInStore(`%s`, `%s`, `%s`, `3452d556-4cc6-b5ba-9d8d-e5382a7c97b1`, `purchasedAndWantEarly`)'", myTcOut.tcfilename, orgId, strings.Split(orgId, "_")[0])
				openinnewwindowstr = "&nbsp;&nbsp;<span class='glyphicon glyphicon-new-window'></span>"
			}
			w.Write([]byte(fmt.Sprintf("<tr><td %s style='background-color: %s'>%s%s</td><td style='background-color: %s; text-align: center'>%s</td><td style='background-color: %s; text-align: center'>%s</td><td class='toggleArrows' style='background-color: %s; text-align: center; font-size: larger; color: red;' hx-swap='none' hx-post='/delete-my-tc' hx-ext='json-enc' hx-vals='{%s: %s}' hx-confirm='This will delete the time capsule forever and it will be unretrievable. Are you sure you want to continue?'>X</td></tr>", openinstorestr, bgColor, myTcOut.tcname, openinnewwindowstr, bgColor, strings.Split(myTcOut.createdon, "T")[0], bgColor, strings.Split(myTcOut.availableOn, "T")[0], bgColor, "\"myTCName\"", "\""+myTcOut.tcname+"\"")))
			iter++
		}
	}
	wixWebhookChangePlanHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := validateWebhookJWTToken(jwtSignKey, r)
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

		rewrite := bytes.ReplaceAll(envvar, []byte("SUB_PACKAGE="+subLevel), []byte("SUB_PACKAGE="+strings.ToLower(postData.Plan)))
		writeerr := os.WriteFile(".env", rewrite, 0644)
		if writeerr != nil {
			fmt.Println(writeerr)
			return
		}
		subLevel = strings.ToLower(postData.Plan)
	}
	regUserPaidForPlanHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := validateWebhookJWTToken(jwtSignKey, r)
		if !validBool {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var setStatus = false
		var currentstatus sql.NullBool
		res := db.QueryRow(fmt.Sprintf("select is_paying_subscriber from tfldata.users where username='%s';", r.URL.Query().Get("username")))
		res.Scan(&currentstatus)
		if !currentstatus.Valid {
			currentstatus.Bool = false
		}
		if currentstatus.Bool {
			setStatus = false
		} else {
			setStatus = true
		}
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set is_paying_subscriber = %t where username = '%s';", setStatus, r.URL.Query().Get("username")))
		if uperr != nil {
			activityStr := "updating user is now paying wix webhook"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,105), substr('%s',0,105), now());", uperr.Error(), activityStr))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	wixWebhookTCInitialPurchaseHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := validateWebhookJWTToken(jwtSignKey, r)
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
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.timecapsule set waspurchased=true where tcfilename='%s';", postData.Capsulename))
		if uperr != nil {
			fmt.Println("something went wrong")
		}
		var yearstostore string
		row := db.QueryRow(fmt.Sprintf("select yearstostore from tfldata.timecapsule where tcfilename='%s';", postData.Capsulename))
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
		s3Client.PutObjectTagging(context.TODO(), &s3.PutObjectTaggingInput{
			Bucket: &s3Domain,
			Key:    aws.String("timecapsules/" + postData.Capsulename),
			Tagging: &types.Tagging{
				TagSet: []types.Tag{
					{
						Key:   aws.String("YearsToStore"),
						Value: &yearstostore,
					},
				},
			},
		})
	}
	wixWebhookEarlyAccessPaymentCompleteHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := validateWebhookJWTToken(jwtSignKey, r)
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
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.timecapsule set wasearlyaccesspurchased=true,available_on=now() where tcfilename='%s';", postData.Capsulename))
		if uperr != nil {
			activityStr := "Failed attempt purchase early access from wix"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,createdon,activity) values(substr('%s',0,240), now(), substr('%s',0,105);", uperr.Error(), activityStr))
			return
		}
		fmt.Println(postData)

	}
	validateEndpointForWixHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := validateWebhookJWTToken(jwtSignKey, r)
		if !validBool {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if orgId != r.URL.Query().Get("orgid") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte("true"))
	}
	deleteMyTChandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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

		tcrow := db.QueryRow(fmt.Sprintf("select username,createdon,tcname from tfldata.timecapsule where username='%s' and tcname='%s';", currentUserFromSession, postData.MyTCName))

		tcrow.Scan(&selectedTc.username, &selectedTc.createdon, &selectedTc.tcname)

		_, delerr := db.Exec(fmt.Sprintf("delete from tfldata.timecapsule where username='%s' and tcname='%s';", currentUserFromSession, postData.MyTCName))
		if delerr != nil {
			activityStr := "Failed to delete time capsule from DB"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", delerr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			return
		}
		deletename := strings.Split(selectedTc.createdon, "T")[0] + "_" + selectedTc.tcname + "_capsule_" + selectedTc.username + ".zip"
		go deleteFileFromS3(deletename, "timecapsules/")
	}

	validateJWTHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
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
		allowOrDeny, _, h :=  validateCurrentSessionId(db, r)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		jwt.Parse(jwtCookie.Value, func(jwtToken *jwt.Token) (interface{}, error) {
			timeTilExp, _ := jwtToken.Claims.GetExpirationTime()
			if time.Until(timeTilExp.Time) < 24*time.Hour {
				generateLoginJWT(r.URL.Query().Get("usersession"), w, r, jwtCookie.Value)

			}
			return []byte(jwtSignKey), nil
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
		//db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id=null where username='%s';", r.URL.Query().Get("user")))
	}

	adminGetListOfUsersHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		var isAdmin bool

		rowRes := db.QueryRow(fmt.Sprintf("select is_admin from tfldata.users where username='%s';", currentUserFromSession))

		rowRes.Scan(&isAdmin)

		validBool := validateJWTToken(jwtSignKey, r)
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

		output, outerr := db.Query(fmt.Sprintf("select username, email from tfldata.users order by id %s;", r.URL.Query().Get("sortByLastPass")))
		if outerr != nil {
			activityStr := "Gathering listofusers for admin dash"
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", outerr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
		}
		defer output.Close()

		var curDataObj dataStruct
		for output.Next() {
			scnErr := output.Scan(&curDataObj.username, &curDataObj.email)
			if scnErr != nil {
				activityStr := "Scan err on listofusers admin dash"
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", outerr, time.Now().In(nyLoc).Format(time.DateTime), activityStr))
			}
			w.Write([]byte(fmt.Sprintf("<tr><td style='padding-bottom: 0&percnt;'>%s</td><td style='padding-bottom: 0&percnt;'>%s</td><td style='padding-bottom: 0&percnt;;'><p onclick='openDeleteModal(`%s`)' style='color: white; border-radius: 15px / 15px; box-shadow: 1px 1px 6px black; text-align: center; width: 20&percnt;; background: linear-gradient(130deg, #9d9d9d, #f94242f5); margin: auto; margin-bottom: 10&percnt;;'>X</p></td></tr>", curDataObj.username, curDataObj.email, curDataObj.username)))

		}

	}
	adminGetSubPackageHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		var isAdmin bool

		rowRes := db.QueryRow(fmt.Sprintf("select is_admin from tfldata.users where username='%s';", currentUserFromSession))

		rowRes.Scan(&isAdmin)

		validBool := validateJWTToken(jwtSignKey, r)
		if !validBool || !allowOrDeny || !isAdmin {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var countOfUsers int
		var totalUsers int
		rowOfCount := db.QueryRow("select count(*) from tfldata.users;")
		rowOfCount.Scan(&countOfUsers)
		switch subLevel {
		case "supreme":
			totalUsers = 50
		case "extra":
			totalUsers = 20
		case "standard":
			totalUsers = 8
		}

		w.Write([]byte(subLevel + " - " + "Current user count: " + fmt.Sprint(countOfUsers) + "/" + fmt.Sprint(totalUsers)))
	}
	adminGetAllTCHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		var isAdmin bool

		rowRes := db.QueryRow(fmt.Sprintf("select is_admin from tfldata.users where username='%s';", currentUserFromSession))

		rowRes.Scan(&isAdmin)

		validBool := validateJWTToken(jwtSignKey, r)
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

		output, _ := db.Query(fmt.Sprintf("select tcname, createdon, available_on from tfldata.timecapsule where available_on %s now() order by available_on asc;", r.URL.Query().Get("pastorpresent")))

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
		allowOrDeny, currentUserFromSession, h := validateCurrentSessionId(db, r)

		var isAdmin bool

		rowRes := db.QueryRow(fmt.Sprintf("select is_admin from tfldata.users where username='%s';", currentUserFromSession))

		rowRes.Scan(&isAdmin)

		validBool := validateJWTToken(jwtSignKey, r)
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

		postfileout, postfileouterr := db.Query(fmt.Sprintf("select file_name,file_type from tfldata.postfiles where post_files_key in (select post_files_key from tfldata.posts where author='%s');", postData.SelectedUser))
		if postfileouterr != nil {
			fmt.Println(postfileouterr)
		}
		defer postfileout.Close()

		tcrow := db.QueryRow(fmt.Sprintf("select createdon,tcname from tfldata.timecapsule where username='%s';", postData.SelectedUser))

		scner := tcrow.Scan(&tcFileToDeleteCreatedon, &tcFileToDeleteTcname)
		if scner != nil {
			fmt.Println(scner)
		}

		pfprow := db.QueryRow(fmt.Sprintf("select pfp_name from tfldata.users where username='%s';", postData.SelectedUser))
		pfpscnerr := pfprow.Scan(&pfpName)
		if pfpscnerr != nil {
			fmt.Println(pfpscnerr)
		}
		tcFileToDeleteTcname = strings.Split(tcFileToDeleteCreatedon, "T")[0] + "_" + tcFileToDeleteTcname + "_capsule_" + postData.SelectedUser + ".zip"

		var mongoRecords []bson.M

		cursor, findErr := coll.Find(context.TODO(), bson.D{{Key: "username", Value: postData.SelectedUser}, {Key: "org_id", Value: orgId}})
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
		go deleteFileFromS3(tcFileToDeleteTcname, "timecapsules/")
		deleteFileFromS3(pfpName, "pfp/")
		if postData.DeleteAllOpt == "yes" {
			for postfileout.Next() {
				var fileName string
				var fileType string
				scnerr := postfileout.Scan(&fileName, &fileType)
				if scnerr != nil {
					fmt.Println(scnerr)
				}
				if strings.Contains(fileType, "image") {
					deleteFileFromS3(fileName, "posts/images/")
				} else {
					go deleteFileFromS3(fileName, "posts/videos/")
				}
			}
			db.Exec(fmt.Sprintf("delete from tfldata.calendar where event_owner='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.comments where author='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.calendar_rsvp where username='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.gchat where thread in (select thread from tfldata.threads where threadauthor = '%s');", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.gchat where author='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.threads where threadauthor='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.users_to_threads where username='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.stack_leaderboard where username='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.ss_leaderboard where username='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.catchitleaderboard where username='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.timecapsule where username='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.posts where author='%s';", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.postfiles where post_files_key in (select post_files_key from tfldata.posts where author='%s');", postData.SelectedUser))
			db.Exec(fmt.Sprintf("delete from tfldata.users where username='%s';", postData.SelectedUser))

		} else {
			if postData.DeleteChatsOpt == "on" {
				db.Exec(fmt.Sprintf("delete from tfldata.gchat where thread in (select thread from tfldata.threads where threadauthor = '%s');", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.gchat where author='%s';", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.threads where threadauthor='%s';", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.users_to_threads where username='%s';", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.users where username='%s';", postData.SelectedUser))
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
						deleteFileFromS3(fileName, "posts/images/")
					} else {
						go deleteFileFromS3(fileName, "posts/videos/")
					}
				}
				db.Exec(fmt.Sprintf("delete from tfldata.posts where author='%s';", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.postfiles where post_files_key in (select post_files_key from tfldata.posts where author='%s');", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.users where username='%s';", postData.SelectedUser))
			}
			if postData.DeleteGameScoresOpt == "on" {
				db.Exec(fmt.Sprintf("delete from tfldata.stack_leaderboard where username='%s';", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.ss_leaderboard where username='%s';", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.catchitleaderboard where username='%s';", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.users where username='%s';", postData.SelectedUser))
			}
			if postData.DeleteCalendarEventsOpt == "on" {
				db.Exec(fmt.Sprintf("delete from tfldata.calendar where event_owner='%s';", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.comments where author='%s';", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.calendar_rsvp where username='%s';", postData.SelectedUser))
				db.Exec(fmt.Sprintf("delete from tfldata.users where username='%s';", postData.SelectedUser))
			}

		}

	}
	healthCheckHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("true"))
	}

	http.HandleFunc("/", pagesHandler)
	http.HandleFunc("/create-post", createPostHandler)

	http.HandleFunc("/create-reaction-to-post", createPostReactionHandler)

	http.HandleFunc("/get-posts", getPostsHandler)
	http.HandleFunc("/delete-this-post", deleteThisPostHandler)

	http.HandleFunc("/get-selected-post", getSelectedPostsComments)

	http.HandleFunc("/get-posts-reactions", getPostsReactionsHandler)

	http.HandleFunc("/get-events", getEventsHandler)
	http.HandleFunc("/get-event-comments", getSelectedEventsComments)

	http.HandleFunc("/get-selected-chat", getSelectedChatHandler)
	http.HandleFunc("/get-selected-pchat", getSelectedPChatHandler)

	http.HandleFunc("/get-post-images", getPostImagesHandler)

	http.HandleFunc("/create-comment", createCommentHandler)
	http.HandleFunc("/create-event-comment", createEventCommentHandler)

	http.HandleFunc("/get-username-from-session", getSessionDataHandler)
	http.HandleFunc("/get-check-if-subscribed", getSubscribedHandler)

	http.HandleFunc("/create-event", createEventHandler)
	http.HandleFunc("/update-rsvp-for-event", updateRSVPForEventHandler)
	http.HandleFunc("/get-rsvp-data", getEventRSVPHandler)
	http.HandleFunc("/get-rsvp", getRSVPNotesHandler)
	http.HandleFunc("/delete-event", deleteEventHandler)

	http.HandleFunc("/group-chat-messages", getGroupChatMessagesHandler)
	http.HandleFunc("/create-a-group-chat-message", createGroupChatMessageHandler)
	http.HandleFunc("/del-thread", delThreadHandler)
	http.HandleFunc("/get-all-users-to-tag", getUsernamesToTagHandler)

	http.HandleFunc("/change-gchat-order-opt", changeGchatOrderOptHandler)

	http.HandleFunc("/private-chat-messages", getPrivateChatMessagesHandler)
	http.HandleFunc("/create-a-private-chat-message", createPrivatePChatMessageHandler)
	http.HandleFunc("/update-last-viewed-direct", updateLastViewedPChatHandler)
	http.HandleFunc("/update-last-viewed-thread", updateLastViewedThreadHandler)

	http.HandleFunc("/update-pchat-reaction", updatePChatReactionHandler)
	http.HandleFunc("/current-pchat-reaction", getCurrentPChatReactionHandler)

	http.HandleFunc("/create-subscription", subscriptionHandler)

	http.HandleFunc("/update-pfp", updatePfpHandler)
	http.HandleFunc("/update-gchat-bg-theme", updateChatThemeHandler)

	http.HandleFunc("/update-selected-chat", updateSelectedChatHandler)
	http.HandleFunc("/delete-selected-chat", deleteSelectedChatHandler)

	http.HandleFunc("/update-selected-pchat", updateSelectedPChatHandler)
	http.HandleFunc("/delete-selected-pchat", deleteSelectedPChatHandler)

	http.HandleFunc("/create-issue", createIssueHandler)

	http.HandleFunc("/get-leaderboard", getLeaderboardHandler)
	http.HandleFunc("/update-simpleshades-score", updateSimpleShadesScoreHandler)

	http.HandleFunc("/get-stackerz-leaderboard", getStackerzLeaderboardHandler)
	http.HandleFunc("/update-stackerz-score", updateStackerzScoreHandler)

	http.HandleFunc("/get-catchit-leaderboard", getCatchitLeaderboardHandler)
	http.HandleFunc("/get-my-personal-score-catchit", getPersonalCatchitLeaderboardHandler)
	http.HandleFunc("/update-catchit-score", updateCatchitScoreHandler)

	http.HandleFunc("/get-open-threads", getOpenThreadsHandler)
	http.HandleFunc("/get-users-chat", getUsersToChatToHandler)

	http.HandleFunc("/get-users-subscribed-threads", getUsersSubscribedThreadsHandler)
	http.HandleFunc("/change-if-notified-for-thread", changeUserSubscriptionToThreadHandler)

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
	// For production
	// log.Fatal(http.ListenAndServeTLS(":443", "../tflserver.crt", "../tflserver.key", nil))
}

func setLoginCookie(w http.ResponseWriter, db *sql.DB, userStr string, acceptedTz string) {

	sessionToken := uuid.NewString()
	expiresAt := time.Now().Add(3600 * time.Hour)
	//fmt.Println(expiresAt.Local().Format(time.DateTime))
	//fmt.Println(userStr)
	/*_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.sessions(\"username\", \"session_token\", \"expiry\", \"ip_addr\") values('%s', '%s', '%s', '%s') on conflict(ip_addr) do update set session_token='%s', expiry='%s';", userStr, sessionToken, expiresAt.Format(time.DateTime), strings.Split(r.RemoteAddr, ":")[0], sessionToken, expiresAt.Format(time.DateTime)))
	  if inserterr != nil {
	  db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserterr))
	  fmt.Println(inserterr)
	  }*/
	_, updateerr := db.Exec(fmt.Sprintf("update tfldata.users set session_token='%s', mytz='%s' where username='%s' or email='%s';", sessionToken, acceptedTz, userStr, userStr))
	if updateerr != nil {
		db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s');", updateerr, time.Now().In(nyLoc).Format(time.DateTime)))
		fmt.Printf("err: '%s'", updateerr)
	}
	maxAge := time.Until(expiresAt)

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionToken,
		MaxAge:   int(maxAge.Seconds()),
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

}
func uploadPfpToS3(f multipart.File, fn string, r *http.Request, formInputIdentifier string) string {

	defer f.Close()
	ourfile, fileHeader, errfile := r.FormFile(formInputIdentifier)

	if errfile != nil {
		log.Fatal(errfile)
	}

	fileContents := make([]byte, fileHeader.Size)

	ourfile.Read(fileContents)
	filetype := http.DetectContentType(fileContents)
	f.Seek(0, 0)
	buf := bytes.NewBuffer(nil)
	_, err := io.Copy(buf, f)
	if err != nil {
		os.Exit(2)
	}

	f.Seek(0, 0)

	newimg, _, decerr := imagego.Decode(buf)
	if decerr != nil {
		log.Fatal("dec err: " + decerr.Error())
	}
	var compfile bytes.Buffer
	encerr := jpeg.Encode(&compfile, newimg, &jpeg.Options{
		Quality: 18,
	})
	if encerr != nil {
		fmt.Println(encerr)
	}
	tmpFileName := fn

	getout, geterr := s3Client.GetObjectAttributes(context.TODO(), &s3.GetObjectAttributesInput{
		Bucket: aws.String(s3Domain),
		Key:    aws.String("pfp/" + tmpFileName),
		ObjectAttributes: []types.ObjectAttributes{
			"ObjectSize",
		},
	})

	if geterr != nil {
		fmt.Println("We can ignore this image: " + geterr.Error())

	} else {

		if *getout.ObjectSize > 1 {
			tmpFileName = strings.ReplaceAll(strings.ReplaceAll(time.Now().Format(time.DateTime), " ", "_"), ":", "") + "_" + tmpFileName
			fn = tmpFileName
		}
	}

	if len(tmpFileName) > 55 {
		fn = tmpFileName[len(tmpFileName)-35:]
	}
	defer ourfile.Close()

	_, err4 := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:       aws.String(s3Domain),
		Key:          aws.String("pfp/" + fn),
		Body:         &compfile,
		ContentType:  &filetype,
		CacheControl: aws.String("max-age=31536000"),
	})

	if err4 != nil {
		fmt.Println("error on upload")
		fmt.Println(err4.Error())
	}
	return fn
}
func uploadFileToS3(f multipart.File, fn string, db *sql.DB, filetype string) {

	if strings.Contains(filetype, "image") {

		f.Seek(0, 0)
		var gettagerr error
		var tag *tiff.Tag
		x, exiferr := exif.Decode(f)
		if exiferr != nil {
			fmt.Println("Err decoding exif format")
			gettagerr = exiferr
		} else {
			tag, gettagerr = x.Get(exif.Orientation)
		}
		if gettagerr != nil {
			f.Seek(0, 0)
			buf := bytes.NewBuffer(nil)
			_, err := io.Copy(buf, f)
			if err != nil {
				fmt.Println("Err copying file to buffer")
			}

			newimg, _, decerr := imagego.Decode(buf)
			if decerr != nil {
				fmt.Println("dec err: " + decerr.Error())
				// we can actually exit program here
			}
			var compfile bytes.Buffer
			encerr := jpeg.Encode(&compfile, newimg, &jpeg.Options{
				Quality: 18,
			})
			if encerr != nil {
				fmt.Println(encerr)
			}
			_, err4 := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket:       aws.String(s3Domain),
				Key:          aws.String("posts/images/" + fn),
				Body:         &compfile,
				ContentType:  &filetype,
				CacheControl: aws.String("max-age=31536000"),
			})

			if err4 != nil {
				fmt.Println("error on upload")
				fmt.Println(err4.Error())
			}

		} else {
			f.Seek(0, 0)
			imgtrn, _, err := imagego.Decode(f)
			if err != nil {
				fmt.Println(err)
				fmt.Println("err on imgtrn decoding")
			}
			if tag.Count == 1 && tag.Format() == tiff.IntVal {
				orientation, err := tag.Int(0)
				if err != nil {
					fmt.Println(err)
					fmt.Println("orientation err")
				}
				switch orientation {
				case 3: // rotate 180
					imgtrn = imaging.Rotate180(imgtrn)
				case 6: // rotate 270
					imgtrn = imaging.Rotate270(imgtrn)
				case 8: //rotate 90
					imgtrn = imaging.Rotate90(imgtrn)
				}
			}

			newbuf := bytes.NewBuffer(nil)
			trnencerr := jpeg.Encode(newbuf, imgtrn, nil)

			if trnencerr != nil {
				fmt.Println(trnencerr)
				fmt.Println("error encoding turned image")

			}

			newimg, _, decerr := imagego.Decode(newbuf)
			if decerr != nil {

				activityStr := "error on image decoding for uploadfiletos3"
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values (substr('%s',0,420), substr('%s',0,106), now());", decerr.Error(), activityStr))
				return
			}
			var compfile bytes.Buffer
			encerr := jpeg.Encode(&compfile, newimg, &jpeg.Options{
				Quality: 18,
			})

			if encerr != nil {
				fmt.Println(encerr)
			}
			_, err4 := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket:       aws.String(s3Domain),
				Key:          aws.String("posts/images/" + fn),
				Body:         &compfile,
				ContentType:  &filetype,
				CacheControl: aws.String("max-age=31536000"),
			})

			if err4 != nil {
				fmt.Println("error on upload")
				fmt.Println(err4.Error())
			}

		}
	} else {

		_, err4 := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:       aws.String(s3Domain),
			Key:          aws.String("posts/videos/" + fn),
			Body:         f,
			ContentType:  &filetype,
			CacheControl: aws.String("max-age=31536000"),
		})

		if err4 != nil {
			fmt.Println("error on upload")
			fmt.Println(err4.Error())
		}

	}
}

func sendNotificationToSingleUser(db *sql.DB, fb_message_client *messaging.Client, fcmToken string, opts notificationOpts) {
	typePayload := make(map[string]string)
	typePayload["type"] = opts.extraPayloadVal
	sentRes, sendErr := fb_message_client.Send(context.TODO(), &messaging.Message{
		Token: fcmToken,
		Notification: &messaging.Notification{
			Title:    opts.notificationTitle,
			Body:     strings.ReplaceAll(opts.notificationBody, "\\", ""),
			ImageURL: "/assets/icon-180x180.jpg",
		},

		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: opts.notificationTitle,
				Body:  strings.ReplaceAll(opts.notificationBody, "\\", ""),
				Data:  typePayload,
				Image: "/assets/icon-180x180.jpg",
				Icon:  "/assets/icon-96x96.jpg",
				Actions: []*messaging.WebpushNotificationAction{
					{
						Action: typePayload["type"],
						Title:  opts.notificationTitle,
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
	db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sentRes, time.Now().In(nyLoc).Local().Format(time.DateTime)))
}

func sendNotificationToAllUsers(db *sql.DB, curUser string, fb_message_client *messaging.Client, opts *notificationOpts) {

	usersNotInUtT, outperr := db.Query(fmt.Sprintf("select username from tfldata.users where username not in (select username from tfldata.users_to_threads where thread='%s');", opts.extraPayloadVal))
	if outperr != nil {
		activityStr := "Non issue logging on sendnotificationtoallusers first db output"
		db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"activity\", \"createdon\") values ('%s', '%s', now());", outperr, activityStr))
	}

	defer usersNotInUtT.Close()

	for usersNotInUtT.Next() {
		var user string
		usersNotInUtT.Scan(&user)
		db.Exec(fmt.Sprintf("insert into tfldata.users_to_threads(\"username\",\"thread\",\"is_subscribed\") values('%s', '%s', true) on conflict(username,thread) do nothing;", user, opts.extraPayloadVal))
	}

	var output *sql.Rows
	var outerr error
	if opts.isTagged {
		output, outerr = db.Query(fmt.Sprintf("select username from tfldata.users_to_threads where thread='%s' and username != '%s';", opts.extraPayloadVal, curUser))
	} else {
		output, outerr = db.Query(fmt.Sprintf("select username from tfldata.users_to_threads where thread='%s' and username != '%s' and is_subscribed=true;", opts.extraPayloadVal, curUser))
	}
	if outerr != nil {
		activityStr := "Panic on sendnotificationtoallusers second db output"
		db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"activity\", \"createdon\") values ('%s', '%s', now());", outerr, activityStr))
	}

	defer output.Close()

	typePayload := make(map[string]string)
	typePayload["type"] = opts.notificationPage
	typePayload[opts.extraPayloadKey] = opts.extraPayloadVal
	for output.Next() {
		var userToSend string

		usrToSendScnErr := output.Scan(&userToSend)

		if usrToSendScnErr == nil {
			var fcmToken sql.NullString
			var sendErr error
			tokenRow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username='%s';", userToSend))
			scnerr := tokenRow.Scan(&fcmToken)
			if scnerr == nil {
				if fcmToken.Valid {

					_, sendErr = fb_message_client.Send(context.TODO(), &messaging.Message{

						Token: fcmToken.String,
						Data:  typePayload,
						Notification: &messaging.Notification{
							Title:    opts.notificationTitle,
							Body:     opts.notificationBody,
							ImageURL: "/assets/icon-180x180.jpg",
						},
						Webpush: &messaging.WebpushConfig{
							Notification: &messaging.WebpushNotification{
								Title: opts.notificationTitle,
								Body:  opts.notificationBody,
								Data:  typePayload,
								Image: "/assets/icon-180x180.jpg",
								Icon:  "/assets/icon-96x96.png",
								Actions: []*messaging.WebpushNotificationAction{
									{
										Action: typePayload["type"],
										Title:  opts.notificationTitle,
										Icon:   "/assets/icon-96x96.png",
									},
									{
										Action: typePayload[opts.extraPayloadKey],
										Title:  "NA",
										Icon:   "/assets/icon-96x96.png",
									},
								},
							},
						},
						Android: &messaging.AndroidConfig{
							Notification: &messaging.AndroidNotification{
								Title:    opts.notificationTitle,
								Body:     opts.notificationBody,
								ImageURL: "/assets/icon-180x180.jpg",
								Icon:     "/assets/icon-96x96.png",
							},
						},
					})

					if sendErr != nil {
						activityStr := "Error sending notificationtoallusers"
						db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", sendErr.Error(), time.Now().In(nyLoc).Format(time.DateTime), activityStr))
						// fmt.Print(sendErr.Error() + " for user: " + userToSend)
						if strings.Contains(sendErr.Error(), "404") {
							db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id=null where username='%s';", userToSend))
							fmt.Println("updated " + userToSend + "'s fcm token")
						}
					}
				}
			}
		}
		//db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sendRes, time.Now().In(nyLoc).Local().Format(time.DateTime)))

	}
}

func generateLoginJWT(username string, w http.ResponseWriter, jwtKey string) *jwt.Token {
	daysToExp := int64(7)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":  "backend-auth",
		"user": username,
		"exp":  time.Now().Unix() + (24 * 60 * 60 * daysToExp),
	})
	expiresAt := time.Now().Add(24 * time.Duration(daysToExp) * time.Hour)

	signKey, _ := token.SignedString([]byte(jwtKey))
	http.SetCookie(w, &http.Cookie{
		Name:     "backendauth",
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		Value:    signKey,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})
	return token
}
func validateJWTToken(tokenKey string, r *http.Request) bool {
	jwtCookie, cookieErr := r.Cookie("backendauth")
	if cookieErr != nil {
		return false
	}

	jwtToken, jwtValidateErr := jwt.Parse(jwtCookie.Value, func(jwtToken *jwt.Token) (interface{}, error) {
		return []byte(tokenKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if jwtValidateErr != nil {
		return false
	}
	return jwtToken.Valid
}
func validateWebhookJWTToken(tokenKey string, r *http.Request) bool {
	jwtHeaderVal := r.Header.Get("Authorization")
	jwtToken, jwtValidateErr := jwt.Parse(jwtHeaderVal, func(jwtToken *jwt.Token) (interface{}, error) {
		return []byte(tokenKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if jwtValidateErr != nil {
		return false
	}
	return jwtToken.Valid
}
func validateCurrentSessionId(db *sql.DB, r *http.Request) (bool, string, string) {
	var handlerForLogin string
	session_token, seshErr := r.Cookie("session_id")
	if seshErr != nil {
		handlerForLogin = "onUnauthorizedEvent"
		return false, "Please login", handlerForLogin
	}

	var username sql.NullString
	var currentlypaying sql.NullBool
	var currentSessionToken sql.NullString
	row := db.QueryRow(fmt.Sprintf("select username, is_paying_subscriber, session_token from tfldata.users where session_token='%s';", strings.Split(session_token.Value, "session_id=")[0]))
	scnerr := row.Scan(&username, &currentlypaying, &currentSessionToken)
	if scnerr != nil {
		handlerForLogin = "onUnauthorizedEvent"
		return false, "Please login", handlerForLogin
	}

	if currentSessionToken.Valid && currentSessionToken.String != strings.Split(session_token.Value, "session_id=")[0] {
		handlerForLogin = "onUnauthorizedEvent"
		return false, "Please login", handlerForLogin
	}

	if !currentlypaying.Valid {
		currentlypaying.Bool = false
		handlerForLogin = "onRevealedYouHaveNotPurchasedRegularUserSubscriptionPlan"
	}
	if !currentlypaying.Bool {
		handlerForLogin = "onRevealedYouHaveNotPurchasedRegularUserSubscriptionPlan"
		return false, username.String, handlerForLogin
	}

	return scnerr == nil, username.String, handlerForLogin

}
func uploadTimeCapsuleToS3(f *os.File, fn string) {
	f.Seek(0, 0)

	defer f.Close()

	_, s3err := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(s3Domain),
		Key:         aws.String("timecapsules/" + fn),
		ContentType: aws.String("application/octet-stream"),
		Body:        f,
		//StorageClass: types.StorageClassGlacier,
		//Tagging: aws.String("YearsToStore=" + r.PostFormValue("yearsToStore")),
	})

	if s3err != nil {
		fmt.Println("error on upload")
		fmt.Println(s3err.Error())
	}

	defer os.Remove(fn)
}
func deleteFileFromS3(delname string, delPath string) {

	_, err := s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s3Domain),
		Key:    aws.String(delPath + delname),
	})

	if err != nil {
		fmt.Println("error on file delete")
		fmt.Println(err.Error())
	}
}
