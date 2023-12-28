package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
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
}
type Postjoin struct {
	Filename     string
	Filetype     string
	Postfileskey string
}
type seshStruct struct {
	Username      string
	Pfpname       string
	BGtheme       string
	GchatOrderOpt bool
	CFDomain      string
}
type notificationOpts struct {
	notificationPage  string
	extraPayloadKey   string
	extraPayloadVal   string
	notificationTitle string
	notificationBody  string
}

var awskey string
var awskeysecret string
var ghissuetoken string
var s3Domain string
var nyLoc *time.Location

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

	opts := []option.ClientOption{option.WithCredentialsFile("the-family-loop-fb0d9-firebase-adminsdk-k6sxl-14c7d4c4f7.json")}

	// Initialize firebase app
	app, err := firebase.NewApp(context.Background(), nil, opts...)
	if err != nil {
		fmt.Printf("Error in initializing firebase app: %s", err)
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
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		seshVal := strings.Split(seshToken.Value, "session_id=")[0]
		_, inserr := db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id='%s' where session_token='%s';", postData.Fcmtoken, seshVal))
		if inserr != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", inserr, time.Now().In(nyLoc).Format(time.DateTime)))
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", errfile, time.Now().In(nyLoc).Format(time.DateTime)))
			w.WriteHeader(http.StatusBadRequest)
		}
		uploadPfpToS3(awskey, awskeysecret, upload, filename.Filename, r, "pfpformfile")
		bs := []byte(r.PostFormValue("passwordsignup"))

		bytesOfPass, err := bcrypt.GenerateFromPassword(bs, len(bs))
		if err != nil {
			fmt.Println(err)
		}

		_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.users(\"username\", \"password\", \"pfp_name\", \"email\", \"gchat_bg_theme\", \"gchat_order_option\", \"cf_domain_name\", \"orgid\", \"is_admin\") values('%s', '%s', '%s', '%s', '%s', %t, '%s', '%s', %t);", strings.ToLower(r.PostFormValue("usernamesignup")), bytesOfPass, filename.Filename, strings.ToLower(r.PostFormValue("emailsignup")), "background: linear-gradient(142deg, #00009f, #3dc9ff 26%)", true, cfdistro, orgId, false))

		if errinsert != nil {
			fmt.Println(errinsert)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", errinsert, time.Now().In(nyLoc).Format(time.DateTime)))
			w.WriteHeader(http.StatusBadRequest)
		}
		_, errutterr := db.Exec(fmt.Sprintf("insert into tfldata.users_to_threads(\"username\", \"thread\", \"is_subscribed\") values('%s', 'posts', true),('%s', 'calendar',true)", strings.ToLower(r.PostFormValue("usernamesignup")), strings.ToLower(r.PostFormValue("usernamesignup"))))
		if errutterr != nil {
			fmt.Printf("user %s will not be subscribed to new posts as of now", strings.ToLower(r.PostFormValue("usernamesignup")))
		}
	}

	loginHandler := func(w http.ResponseWriter, r *http.Request) {

		userStr := strings.ToLower(r.PostFormValue("usernamelogin"))

		var password string
		var isAdmin bool
		var curFirebaseUid string
		var emailIn string
		passScan := db.QueryRow(fmt.Sprintf("select is_admin, password, email, firebase_user_uid from tfldata.users where username='%s' or email='%s';", userStr, userStr))
		scnerr := passScan.Scan(&isAdmin, &password, &emailIn, &curFirebaseUid)

		if isAdmin {

			if password == r.PostFormValue("passwordlogin") {

				w.Header().Set("HX-Trigger", "changeAdminPassword")

				setLoginCookie(w, db, userStr, r)
				generateLoginJWT(userStr, w, r, jwtSignKey)
			} else {
				err := bcrypt.CompareHashAndPassword([]byte(password), []byte(r.PostFormValue("passwordlogin")))

				if err != nil {
					db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))
					w.Header().Set("HX-Trigger", "loginevent")
				} else if err == nil {

					generateLoginJWT(userStr, w, r, jwtSignKey)

					setLoginCookie(w, db, userStr, r)
					_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set last_sign_on='%s' where username='%s';", time.Now().In(nyLoc).Format(time.DateTime), userStr))
					if uperr != nil {
						fmt.Println(uperr)
					}
					//w.WriteHeader(http.StatusOK)
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
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))
				w.Header().Set("HX-Trigger", "loginevent")
			} else if err == nil {

				generateLoginJWT(userStr, w, r, jwtSignKey)

				setLoginCookie(w, db, userStr, r)

				_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set last_sign_on='%s' where username='%s';", time.Now().In(nyLoc).Format(time.DateTime), userStr))
				if uperr != nil {
					fmt.Println(uperr)
				}

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
		}
		_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set password='%s' where username='%s';", newAdminbytesOfPass, postData.Username))
		if uperr != nil {
			fmt.Println(uperr)
		}

		w.Header().Set("HX-Refresh", "true")
	}
	getResetPasswordCodeHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
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
			//fmt.Println(val.MessageAttributes)
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

		//tmpl := template.Must(template.ParseFiles("index.html"))

		//tmpl.Execute(w, nil)
		bs, _ := os.ReadFile("navigation.html")
		navtmple := template.New("Navt")
		tm, _ := navtmple.Parse(string(bs))

		switch r.URL.Path {
		case "/groupchat":
			tmpl := template.Must(template.ParseFiles("groupchat.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		case "/home":
			//go cookieExpirationCheck(w, r, db)
			tmpl := template.Must(template.ParseFiles("index.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		case "/calendar":
			tmpl := template.Must(template.ParseFiles("calendar.html"))
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
		default:
			tmpl := template.Must(template.ParseFiles("index.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		}

	}
	getPostsHandler := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		seshToken, seshErr := r.Cookie("session_id")
		if seshErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		seshVal := strings.Split(seshToken.Value, "session_id=")[0]
		var reactionBtn string
		//curUser := r.URL.Query().Get("username")

		var curUser string
		row := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", seshVal))
		row.Scan(&curUser)
		if curUser < " " {
			curUser = "Guest"
		}

		var output *sql.Rows
		if r.URL.Query().Get("page") == "null" {
			output, err = db.Query("select id, \"title\", description, author, post_files_key from tfldata.posts order by createdon DESC limit 2;")
		} else if r.URL.Query().Get("limit") == "current" {
			w.Header().Set("HX-Reswap", "innerHTML")
			output, err = db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key from tfldata.posts where id >= %s order by createdon DESC;", r.URL.Query().Get("page")))
		} else {
			output, err = db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key from tfldata.posts where id < %s order by createdon DESC limit 2;", r.URL.Query().Get("page")))
		}

		var dataStr string
		if err != nil {
			//log.Fatal(err)
			fmt.Print(err)
		}

		defer output.Close()
		for output.Next() {

			var postrows Postsrow
			var reaction string
			//if err := output.Scan(&postrows.Id, &postrows.Title, &postrows.Description, &postrows.File_name, &postrows.File_type, &postrows.Author); err != nil {
			if err := output.Scan(&postrows.Id, &postrows.Title, &postrows.Description, &postrows.Author, &postrows.Postfileskey); err != nil {

				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))

			}

			reactionRow := db.QueryRow(fmt.Sprintf("select reaction from tfldata.reactions where post_id=%d and author='%s';", postrows.Id, curUser))
			reactionRow.Scan(&reaction)

			editElement := ""
			if postrows.Author != curUser {
				if reaction > " " {
					reactionBtn = fmt.Sprintf("&nbsp;&nbsp;"+reaction+"<div onclick='addAReaction(%d)'><i class='bi bi-three-dots'></i></div>", postrows.Id)
				} else {
					reactionBtn = fmt.Sprintf("<button class='btn btn-outline-secondary border-0 px-2' type='button' onclick='addAReaction(%d)'><i class='bi bi-three-dots-vertical'></i></button>", postrows.Id)
				}
			} else {
				reactionBtn = ""
				editElement = fmt.Sprintf("<i style='position: absolute; background-color: gray; border-radius: 13px / 13px; left: 87%s; z-index: 3' class='bi bi-three-dots m-1 px-1' hx-post='/delete-this-post' hx-swap='none' hx-vals=\"js:{'deletionID': %d}\" hx-params='not page, limit, token' hx-ext='json-enc' hx-confirm='Delete this post forever? This cannot be undone'></i>", "%", postrows.Id)
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
			firstRow := db.QueryRow(fmt.Sprintf("select file_name, file_type from tfldata.postfiles where post_files_key='%s' order by id desc limit 1;", postrows.Postfileskey))
			firstRow.Scan(&firstImg.filename, &firstImg.filetype)

			/*if strings.Contains(postrows.Title, "'") {
				postrows.Title = strings.ReplaceAll(postrows.Title, "'", "")
				fmt.Println(postrows.Title)
			}*/

			if strings.Contains(firstImg.filetype, "image") {

				if countOfImg > 1 {
					dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(22 30 255 / .42); border-radius: 72px 72px / 67px 67px; box-shadow: 6px 6px 3px 3px rgb(54 141 150 / 42&percnt;);'>%s<img class='img-fluid' id='%s' src='https://%s/posts/images/%s' alt='%s' style='border-radius: 18px 18px;' alt='default' /><div class='p-2' style='display: flex; justify-content: space-around;'><i onclick='nextLeftImage(`%s`)' class='bi bi-arrow-90deg-left'></i><i onclick='nextRightImage(`%s`)' class='bi bi-arrow-90deg-right'></i></div><div id='%d' class='card-body'><b>%s</b><br/><p>%s</p><p class='card-text'>%s</p><button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' class='btn btn-primary' hx-swap='innerHTML'>Comments (%s)</button>%s</div></div>", editElement, postrows.Postfileskey, cfdistro, firstImg.filename, firstImg.filename, postrows.Postfileskey, postrows.Postfileskey, postrows.Id, postrows.Author, postrows.Title, postrows.Description, postrows.Id, postrows.Id, commentCount, reactionBtn)
				} else if countOfImg == 1 {
					dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(22 30 255 / .42); border-radius: 72px 72px / 67px 67px; box-shadow: 6px 6px 3px 3px rgb(54 141 150 / 42&percnt;);'>%s<img class='img-fluid' id='%s' src='https://%s/posts/images/%s' alt='%s' style='border-radius: 18px 18px;' alt='default' /><div class='p-2' style='display: flex; justify-content: space-around;'></div><div id='%d' class='card-body'><b>%s</b><br/><p>%s</p><p class='card-text'>%s</p><button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary'>Comments (%s)</button>%s</div></div>", editElement, postrows.Postfileskey, cfdistro, firstImg.filename, firstImg.filename, postrows.Id, postrows.Author, postrows.Title, postrows.Description, postrows.Id, postrows.Id, commentCount, reactionBtn)
				}

			} else {

				if countOfImg > 1 {
					dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(22 30 255 / .42); border-radius: 72px 72px / 67px 67px; box-shadow: 6px 6px 3px 3px rgb(54 141 150 / 42&percnt;);'>%s<video style='border-radius: 18px 18px; z-index: 6;' muted playsinline controls id='%s'><source src='https://%s/posts/videos/%s'></video><div class='p-2' style='display: flex; justify-content: space-around;'><i onclick='nextLeftImage(`%s`)' class='bi bi-arrow-90deg-left'></i><i onclick='nextRightImage(`%s`)' class='bi bi-arrow-90deg-right'></i></div><div id='%d' class='card-body'><b>%s</b><br/><p>%s</p><p class='card-text'>%s</p><button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary'>Comments (%s)</button>%s</div></div>", editElement, postrows.Postfileskey, cfdistro, firstImg.filename, postrows.Postfileskey, postrows.Postfileskey, postrows.Id, postrows.Author, postrows.Title, postrows.Description, postrows.Id, postrows.Id, commentCount, reactionBtn)
				} else if countOfImg == 1 {
					dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(22 30 255 / .42); border-radius: 72px 72px / 67px 67px; box-shadow: 6px 6px 3px 3px rgb(54 141 150 / 42&percnt;);'>%s<video style='border-radius: 18px 18px; z-index: 6;' muted playsinline controls id='%s'><source src='https://%s/posts/videos/%s'></video><div class='p-2' style='display: flex; justify-content: space-around;'></div><div id='%d' class='card-body'><b>%s</b><br/><p>%s</p><p class='card-text'>%s</p><button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary'>Comments (%s)</button>%s</div></div>", editElement, postrows.Postfileskey, cfdistro, firstImg.filename, postrows.Id, postrows.Author, postrows.Title, postrows.Description, postrows.Id, postrows.Id, commentCount, reactionBtn)
				}
			}

			postTmpl, tmerr = template.New("tem").Parse(dataStr)
			if tmerr != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", tmerr, time.Now().In(nyLoc).Format(time.DateTime)))
			}
			postTmpl.Execute(w, nil)

		}

	}
	deleteThisPostHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, err := r.Cookie("session_id")
		if err != nil {
			if err == http.ErrNoCookie {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithDefaultRegion("us-east-1"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awskey, awskeysecret, "")),
		)

		if err != nil {
			log.Fatal(err)
			os.Exit(4)
		}

		client := s3.NewFromConfig(cfg)
		var username string
		row := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", c.Value))
		row.Scan(&username)

		postFilesKey := uuid.NewString()

		_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"author\", \"post_files_key\", \"createdon\") values(E'%s', E'%s', '%s', '%s', now());", replacer.Replace(r.PostFormValue("title")), replacer.Replace(r.PostFormValue("description")), username, postFilesKey))

		if errinsert != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", errinsert, time.Now().In(nyLoc).Format(time.DateTime)))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var chatMessageNotificationOpts notificationOpts
		chatMessageNotificationOpts.extraPayloadKey = "post"
		chatMessageNotificationOpts.extraPayloadVal = "posts"
		chatMessageNotificationOpts.notificationPage = "posts"

		chatMessageNotificationOpts.notificationTitle = "Somebody just made a new post!"
		chatMessageNotificationOpts.notificationBody = strings.ReplaceAll(r.PostFormValue("title"), "\\", "")
		go sendNotificationToAllUsers(db, *app, username, chatMessageNotificationOpts)
		parseerr := r.ParseMultipartForm(10 << 20)
		if parseerr != nil {
			// handle error
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('memory error multi file upload %s');", err))
		}
		//upload, filename, errfile := r.FormFile("file_name")
		for _, fh := range r.MultipartForm.File["file_name"] {

			f, err := fh.Open()
			if err != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))
				w.WriteHeader(http.StatusBadRequest)
			}

			tmpFileName := fh.Filename

			getout, geterr := client.GetObjectAttributes(context.TODO(), &s3.GetObjectAttributesInput{
				Bucket: aws.String(s3Domain),
				Key:    aws.String("posts/images/" + tmpFileName),
				ObjectAttributes: []types.ObjectAttributes{
					"ObjectSize",
				},
			})

			if geterr != nil {
				fmt.Println("We can ignore this image: " + geterr.Error())
				getvidout, getviderr := client.GetObjectAttributes(context.TODO(), &s3.GetObjectAttributesInput{
					Bucket: aws.String(s3Domain),
					Key:    aws.String("posts/videos/" + tmpFileName),
					ObjectAttributes: []types.ObjectAttributes{
						"ObjectSize",
					},
				})

				if getviderr != nil {
					fmt.Println("We can ignore this video: " + getviderr.Error())
				} else {
					if *getvidout.ObjectSize > 1 {
						tmpFileName = strings.ReplaceAll(strings.ReplaceAll(time.Now().Format(time.DateTime), " ", "_"), ":", "") + "_" + tmpFileName
						fh.Filename = tmpFileName
					}
				}
			} else {

				if *getout.ObjectSize > 1 {
					tmpFileName = strings.ReplaceAll(strings.ReplaceAll(time.Now().Format(time.DateTime), " ", "_"), ":", "") + "_" + tmpFileName
					fh.Filename = tmpFileName
				}
			}

			if len(tmpFileName) > 55 {
				fh.Filename = tmpFileName[len(tmpFileName)-35:]
			}

			filetype := uploadFileToS3(awskey, awskeysecret, false, f, tmpFileName, r)

			_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.postfiles(\"file_name\", \"file_type\", \"post_files_key\") values('%s', '%s', '%s');", fh.Filename, filetype, postFilesKey))

			if errinsert != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", errinsert, time.Now().In(nyLoc).Format(time.DateTime)))
			}

			defer f.Close()
		}
		/*if errfile != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err))
			w.WriteHeader(http.StatusBadRequest)
		}*/
		/*
			// Returning a filetype from the createandupload function
			// somehow gets the right filetype
			filetype := uploadFileToS3(awskey, awskeysecret, false, upload, filename.Filename, r)

			_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"file_name\", \"file_type\", \"author\") values('%s', '%s', '%s', '%s', '%s');", r.PostFormValue("title"), r.PostFormValue("description"), filename.Filename, filetype, username))

			if errinsert != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", errinsert))
			}*/
		//defer upload.Close()

	}
	createPostReactionHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", marsherr, time.Now().In(nyLoc).Format(time.DateTime)))
		}
		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.reactions(\"post_id\", \"author\", \"reaction\") values(%d, '%s', '%s') on conflict(post_id,author) do update set reaction='%s';", postData.Postid, postData.Username, postData.ReactionToPost, postData.ReactionToPost))
		if inserr != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", inserr, time.Now().In(nyLoc).Format(time.DateTime)))
			w.WriteHeader(http.StatusBadRequest)
		}

	}

	getSelectedPostsComments := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))
		}

		defer output.Close()

		for output.Next() {
			var posts postComment

			if err := output.Scan(&posts.Comment, &posts.Author, &posts.Pfpname); err != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))

			}
			dataStr = "<div class='row'><p style='display: flex; align-items: center; padding-right: 0%;' class='m-1 col-7'>" + posts.Comment + "</p><div style='align-items: center; position: relative; display: flex; padding-left: 0%; left: 1%;' class='col my-5'><b style='position: absolute; bottom: 5%'>" + posts.Author + "</b><img width='30px' class='my-1' style='margin-left: 1%; position: absolute; right: 20%; border-style: solid; border-radius: 13px / 13px; box-shadow: 3px 3px 5px; border-width: thin; top: 5%;' src='https://" + cfdistro + "/pfp/" + posts.Pfpname + "' alt='tfl pfp' /></div></div>"

			w.Write([]byte(dataStr))
		}

	}
	createEventCommentHandler := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, err := r.Cookie("session_id")

		if err != nil {
			if err == http.ErrNoCookie {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if c.Valid() != nil {
			fmt.Println("Cook is no longer valid")
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", errmarsh, time.Now().In(nyLoc).Format(time.DateTime)))
		}

		_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.comments(\"comment\", \"event_id\", \"author\") values(E'%s', '%d', (select username from tfldata.users where session_token='%s'));", replacer.Replace(postData.Eventcomment), postData.CommentSelectedEventId, c.Value))
		if inserterr != nil {
			fmt.Println(inserterr)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", inserterr, time.Now().In(nyLoc).Format(time.DateTime)))
		}
		var author string
		row := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", c.Value))
		row.Scan(&author)

		dataStr := "<p class='p-2'>" + postData.Eventcomment + " - " + author + "</p>"

		commentTmpl, err := template.New("com").Parse(dataStr)
		if err != nil {
			fmt.Println(err)
		}
		commentTmpl.Execute(w, nil)

	}
	getSelectedEventsComments := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, err := r.Cookie("session_id")

		if err != nil {
			if err == http.ErrNoCookie {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if c.Valid() != nil {
			fmt.Println("Cook is no longer valid")
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		bs, _ := io.ReadAll(r.Body)
		type postBody struct {
			Comment        string
			SelectedPostId int
		}
		var postData postBody
		errmarsh := json.Unmarshal(bs, &postData)
		if errmarsh != nil {
			fmt.Println(errmarsh)
		}

		_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.comments(\"comment\", \"post_id\", \"author\") values(E'%s', '%d', (select username from tfldata.users where session_token='%s'));", replacer.Replace(postData.Comment), postData.SelectedPostId, c.Value))
		if inserterr != nil {
			fmt.Println(inserterr)
		}
		var author string
		row := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", c.Value))
		row.Scan(&author)
		// https://stackoverflow.com/questions/2944297/postgresql-function-for-last-inserted-id
		// For adding like / dislike button
		dataStr := "<p class='p-2'>" + postData.Comment + " - " + author + "</p>"

		commentTmpl, err := template.New("com").Parse(dataStr)
		if err != nil {
			fmt.Println(err)
		}
		commentTmpl.Execute(w, nil)

		var fcmToken string
		fcmrow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username = (select author from tfldata.posts where id=%d) and username != (select username from tfldata.users where session_token='%s');", postData.SelectedPostId, c.Value))
		scnerr := fcmrow.Scan(&fcmToken)
		if scnerr != nil {

			if scnerr.Error() == "sql: no rows in result set" {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			db.Exec("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", scnerr, time.Now().In(nyLoc).Local().Format(time.DateTime))

			return
		} else {

			fb_message_client, _ := app.Messaging(context.TODO())
			typePayload := make(map[string]string)
			typePayload["type"] = "posts"
			sentRes, sendErr := fb_message_client.Send(context.TODO(), &messaging.Message{
				Token: fcmToken,
				Notification: &messaging.Notification{
					Title: author + " commented on your post!",
					Body:  "\"" + postData.Comment + "\"",
				},

				Webpush: &messaging.WebpushConfig{
					Notification: &messaging.WebpushNotification{
						Title: author + " commented on your post!",
						Body:  "\"" + postData.Comment + "\"",
						Data:  typePayload,
					},
				},
			})
			if sendErr != nil {
				fmt.Print(sendErr)
			}
			db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sentRes, time.Now().In(nyLoc).Local().Format(time.DateTime)))
		}
	}
	getEventsHandler := func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		type EventData struct {
			Eventid      int
			Startdate    string
			Eventowner   string
			Eventdetails string
			Eventtitle   string
		}

		ourEvents := []EventData{}
		output, err := db.Query("select start_date, event_owner, event_details, event_title, id from tfldata.calendar;")
		if err != nil {
			fmt.Println(err)
		}
		defer output.Close()
		for output.Next() {
			var tempData EventData
			scnerr := output.Scan(&tempData.Startdate, &tempData.Eventowner, &tempData.Eventdetails, &tempData.Eventtitle, &tempData.Eventid)
			if scnerr != nil {
				fmt.Println(scnerr)
				w.WriteHeader(http.StatusBadRequest)
			}
			ourEvents = append(ourEvents, tempData)
		}
		data, marshErr := json.Marshal(ourEvents)

		if marshErr != nil {
			fmt.Println(marshErr)
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Write(data)
	}
	getPostsReactionsHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		output, rowerr := db.Query(fmt.Sprintf("select author, reaction from tfldata.reactions where post_id='%s' and author != '%s';", r.URL.Query().Get("selectedPostId"), r.URL.Query().Get("username")))
		if rowerr != nil {
			db.Exec("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", rowerr, time.Now().In(nyLoc).Local().Format(time.DateTime))
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, err := r.Cookie("session_id")

		if err != nil {
			if err == http.ErrNoCookie {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if c.Valid() != nil {
			fmt.Println("Cook is no longer valid")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		bs, _ := io.ReadAll(r.Body)
		type PostBody struct {
			Startdate    string `json:"start_date"`
			Eventdetails string `json:"event_details"`
			Eventtitle   string `json:"event_title"`
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
		var username string
		row := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", c.Value))
		row.Scan(&username)
		_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.calendar(\"start_date\", \"event_owner\", \"event_details\", \"event_title\") values('%s', '%s', E'%s', E'%s');", postData.Startdate, username, replacer.Replace(postData.Eventdetails), replacer.Replace(postData.Eventtitle)))
		if inserterr != nil {
			fmt.Println(inserterr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var chatMessageNotificationOpts notificationOpts
		// You can use the below key to add onclick features to the notification
		chatMessageNotificationOpts.extraPayloadKey = "calendardata"
		chatMessageNotificationOpts.extraPayloadVal = "calendar"
		chatMessageNotificationOpts.notificationPage = "calendar"
		chatMessageNotificationOpts.notificationTitle = "New event on: " + postData.Startdate
		chatMessageNotificationOpts.notificationBody = strings.ReplaceAll(postData.Eventtitle, "\\", "")
		go sendNotificationToAllUsers(db, *app, username, chatMessageNotificationOpts)

	}
	updateRSVPForEventHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
			db.Exec("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", inserr, time.Now().In(nyLoc).Local().Format(time.DateTime))
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
			db.Exec("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", scnerr, time.Now().In(nyLoc).Local().Format(time.DateTime))
			w.WriteHeader(http.StatusBadRequest)
			return
		} else {

			fb_message_client, _ := app.Messaging(context.TODO())
			typePayload := make(map[string]string)
			typePayload["type"] = "event"
			sentRes, sendErr := fb_message_client.Send(context.TODO(), &messaging.Message{
				Token: fcmToken,
				Notification: &messaging.Notification{
					Title: "Someone RSVPed to your event",
					Body:  postData.Username + " responded to your event",
				},

				Webpush: &messaging.WebpushConfig{
					Notification: &messaging.WebpushNotification{
						Title: "Someone RSVPed to your event",
						Body:  postData.Username + " responded to your event",
						Data:  typePayload,
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, err := r.Cookie("session_id")

		if err != nil {
			if err == http.ErrNoCookie {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if c.Valid() != nil {
			fmt.Println("Cook is no longer valid")
			return
		}

		chatMessage := replacer.Replace(r.PostFormValue("gchatmessage"))

		listOfUsersTagged := strings.Split(r.PostFormValue("taggedUser"), ",")

		var userName string
		userNameRow := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", c.Value))
		userNameRow.Scan(&userName)
		threadVal := r.PostFormValue("threadval")
		if threadVal == "" {
			threadVal = "main thread"
		} else if strings.ToLower(threadVal) == "posts" || strings.ToLower(threadVal) == "calendar" {
			w.WriteHeader(http.StatusConflict)
			return
		}
		var fcmRegToken string
		var chatMessageNotificationOpts notificationOpts
		chatMessageNotificationOpts.extraPayloadKey = "thread"
		chatMessageNotificationOpts.extraPayloadVal = threadVal
		chatMessageNotificationOpts.notificationPage = "groupchat"
		chatMessageNotificationOpts.notificationTitle = "message in: " + threadVal
		chatMessageNotificationOpts.notificationBody = strings.ReplaceAll(chatMessage, "\\", "")
		go sendNotificationToAllUsers(db, *app, userName, chatMessageNotificationOpts)
		if len(listOfUsersTagged) > 0 {
			for _, val := range listOfUsersTagged {
				fcmRegRow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username='%s' and username != '%s';", val, userName))
				scner := fcmRegRow.Scan(&fcmRegToken)
				if scner == nil {
					sendNotificationToTaggedUser(w, r, fcmRegToken, db, strings.ReplaceAll(chatMessage, "\\", ""), app)
				}
			}
		}

		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.gchat(\"chat\", \"author\", \"createdon\", \"thread\") values(E'%s', '%s', '%s', '%s');", chatMessage, userName, time.Now().In(nyLoc).Format(time.DateTime), threadVal))
		if inserr != nil {
			fmt.Println("error here: " + inserr.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, ttbleerr := db.Exec(fmt.Sprintf("insert into tfldata.threads(\"thread\", \"threadauthor\", \"createdon\") values(E'%s', '%s', '%s');", threadVal, userName, time.Now().In(nyLoc).Format(time.DateTime)))
		if ttbleerr != nil {
			fmt.Println("We can ignore this error: " + ttbleerr.Error())
		} else {
			db.Exec("insert into tfldata.users_to_threads(\"username\") select distinct(username) from tfldata.users;")
			db.Exec(fmt.Sprintf("update tfldata.users_to_threads set is_subscribed=true, thread='%s' where is_subscribed is null and thread is null;", threadVal))
		}
		w.Header().Set("HX-Trigger", "success-send")

	}
	delThreadHandler := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
		db.Exec(fmt.Sprintf("delete from tfldata.gchat where thread='%s';", postData.ThreadToDel))
		db.Exec(fmt.Sprintf("delete from tfldata.threads where thread='%s';", postData.ThreadToDel))
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s');", uperr, time.Now().In(nyLoc).Format(time.DateTime)))
		}

	}
	getGroupChatMessagesHandler := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, err := r.Cookie("session_id")
		var curUser string
		//var orderAscOrDesc string
		if err != nil {
			if err == http.ErrNoCookie {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err != nil {
			curUser = "Guest"
		}
		orderAscOrDesc := "asc"
		if r.URL.Query().Get("order_option") == "true" {
			orderAscOrDesc = "asc"
		} else {
			orderAscOrDesc = "desc"
		}
		limitVal, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		output, err := db.Query(fmt.Sprintf("select id, chat, author, createdon from (select * from tfldata.gchat where thread='%s' order by createdon DESC limit %d) as tmp order by createdon %s;", r.URL.Query().Get("threadval"), limitVal, orderAscOrDesc))
		//output, err := db.Query(fmt.Sprintf("select id, chat, author, createdon from tfldata.gchat where thread='%s' order by createdon %s limit %d;", r.URL.Query().Get("threadval"), orderAscOrDesc, limitVal))

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		row := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", c.Value))
		row.Scan(&curUser)
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
			if author == curUser {
				editDelBtn = "<i class='bi bi-three-dots-vertical px-1' onclick='editOrDeleteChat(`" + gchatid + "`)'></i>"
			}
			dataStr := "<div style='max-width: 100%; background-color: rgb(22 53 255 / 13%); border-width: thin; border-style: solid; box-shadow: 4px 4px 5px; border-radius: 16px 5px 23px 3px; padding-bottom: 3%' class='container my-2'><div class='row'><b class='col-2 px-1'>" + author + "</b><div class='row'><img style='width: 15%; position: sticky;' class='col-2 px-2 my-1' src='" + pfpImg + "' alt='tfl pfp' /></div><p class='col-10' style='position: relative; left: 10%; margin-bottom: 1%; margin-top: -15%;'>" + message + "</p></div><div class='row'><p class='col' style='margin-left: 60%; font-size: smaller; margin-bottom: 0%'>" + createdat.Format(formatCreatedatTime) + editDelBtn + "</p></div></div>"
			chattmp, tmperr := template.New("gchat").Parse(dataStr)
			if tmperr != nil {
				fmt.Println(tmperr)
			}
			chattmp.Execute(w, nil)

		}
	}
	getUsernamesToTagHandler := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var imgList []string
		rows, err := db.Query(fmt.Sprintf("select file_name from tfldata.postfiles where post_files_key='%s';", r.URL.Query().Get("id")))
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
		seshToken, seshErr := r.Cookie("session_id")
		if seshErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		seshVal := strings.Split(seshToken.Value, "session_id=")[0]

		var fcmRegToken string
		fcmRegRow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where session_token='%s';", seshVal))
		scnerr := fcmRegRow.Scan(&fcmRegToken)
		if scnerr != nil {
			w.WriteHeader(http.StatusAccepted)
			return
			//db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", scnerr, time.Now().In(nyLoc).Local().Format(time.DateTime)))
		}
		w.WriteHeader(http.StatusOK)
	}
	getSessionDataHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		seshToken, seshErr := r.Cookie("session_id")
		if seshErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		seshVal := strings.Split(seshToken.Value, "session_id=")[0]
		var ourSeshStruct seshStruct

		row := db.QueryRow(fmt.Sprintf("select username, pfp_name, gchat_bg_theme, gchat_order_option, cf_domain_name from tfldata.users where session_token='%s';", seshVal))
		scnerr := row.Scan(&ourSeshStruct.Username, &ourSeshStruct.Pfpname, &ourSeshStruct.BGtheme, &ourSeshStruct.GchatOrderOpt, &ourSeshStruct.CFDomain)
		if scnerr != nil {
			fmt.Println(scnerr)
		}

		data, err := json.Marshal(&ourSeshStruct)
		if err != nil {
			fmt.Println(err)
		}

		w.Write(data)
	}

	updatePfpHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/form-data")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		upload, filename, _ := r.FormFile("changepfp")

		username := r.PostFormValue("usernameinput")

		_, uperr := db.Exec(fmt.Sprintf("update tfldata.users set pfp_name='%s' where username='%s';", filename.Filename, username))
		if uperr != nil {
			fmt.Println(uperr)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s');", uperr, time.Now().In(nyLoc).Format(time.DateTime)))
			w.WriteHeader(http.StatusBadRequest)
		}
		uploadPfpToS3(awskey, awskeysecret, upload, filename.Filename, r, "changepfp")
	}
	updateChatThemeHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s');", uperr, time.Now().In(nyLoc).Format(time.DateTime)))
		}
	}
	deleteSelectedChatHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s');", delerr, time.Now().In(nyLoc).Format(time.DateTime)))
		}
	}
	updateSelectedChatHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s');", uperr, time.Now().In(nyLoc).Format(time.DateTime)))
		}
	}
	getSelectedChatHandler := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
	createIssueHandler := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
		bodyText := fmt.Sprintf("%s on %s page - %s. Orgid: %s", postData.Descdetail[1], postData.Descdetail[0], username, orgId)
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
			//out, err := coll.Aggregate(context.TODO(), bson.A{bson.D{{Key: "$match", Value: bson.D{{Key: "game", Value: "stackerz"}}}}, bson.D{{Key: "$set", Value: bson.D{{Key: "score", Value: bson.D{{Key: "$sum", Value: bson.A{"$bonus_points", "$level"}}}}}}}, bson.D{{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}}}}, bson.D{{Key: "$limit", Value: 15}}})
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
							{Key: "day", Value: bson.D{{Key: "$dayOfMonth", Value: "$createdOn"}}},
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
		inserr, _ := db.Exec(fmt.Sprintf("insert into tfldata.stack_leaderboard(\"username\", \"bonus_points\", \"level\") values('%s', %d, %d)", postData.Username, postData.BonusPoints, postData.Level))
		if inserr != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s');", inserr, time.Now().In(nyLoc).Format(time.DateTime)))
		}
		coll.InsertOne(context.TODO(), bson.M{"org_id": orgId, "game": "stackerz", "bonus_points": postData.BonusPoints, "level": postData.Level, "username": postData.Username, "createdOn": time.Now()})
	}
	getLeaderboardHandler := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
				//dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0'>" + fmt.Sprintf("%d", iter) + "</p><p class='px-2 m-0' style='text-align: center;'>" + username + " - " + score + "</p></div><br/>"
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
							{Key: "day", Value: bson.D{{Key: "$dayOfMonth", Value: "$createdOn"}}},
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
				bson.D{{Key: "$limit", Value: 15}},
				bson.D{{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}}}},
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		distinctThreadsOutput, queryErr := db.Query("select thread,threadauthor from tfldata.threads order by createdon asc;")
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
	getUsersSubscribedThreadsHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s');", inserr, time.Now().In(nyLoc).Format(time.DateTime)))
		}

	}
	validateJWTHandler := func(w http.ResponseWriter, r *http.Request) {
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	refreshTokenHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jwtCookie, cookieErr := r.Cookie("backendauth")
		if cookieErr != nil {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		validBool := validateJWTToken(jwtCookie.Value, jwtSignKey, w)
		if !validBool {
			w.Header().Set("HX-Location", "/")
			w.Header().Set("HX-Retarget", "document")
			w.Header().Set("HX-Trigger", "onUnauthorizedEvent")
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

	http.HandleFunc("/get-post-images", getPostImagesHandler)

	http.HandleFunc("/create-comment", createCommentHandler)
	http.HandleFunc("/create-event-comment", createEventCommentHandler)

	http.HandleFunc("/get-username-from-session", getSessionDataHandler)
	http.HandleFunc("/get-check-if-subscribed", getSubscribedHandler)

	http.HandleFunc("/create-event", createEventHandler)
	http.HandleFunc("/update-rsvp-for-event", updateRSVPForEventHandler)
	http.HandleFunc("/get-rsvp-data", getEventRSVPHandler)
	http.HandleFunc("/get-rsvp", getRSVPNotesHandler)

	http.HandleFunc("/group-chat-messages", getGroupChatMessagesHandler)
	http.HandleFunc("/create-a-group-chat-message", createGroupChatMessageHandler)
	http.HandleFunc("/del-thread", delThreadHandler)
	http.HandleFunc("/get-all-users-to-tag", getUsernamesToTagHandler)

	http.HandleFunc("/change-gchat-order-opt", changeGchatOrderOptHandler)

	http.HandleFunc("/create-subscription", subscriptionHandler)
	// Not currently in use
	//http.HandleFunc("/send-new-posts-push", newPostsHandlerPushNotify)

	http.HandleFunc("/update-pfp", updatePfpHandler)
	http.HandleFunc("/update-gchat-bg-theme", updateChatThemeHandler)

	http.HandleFunc("/update-selected-chat", updateSelectedChatHandler)
	http.HandleFunc("/delete-selected-chat", deleteSelectedChatHandler)

	http.HandleFunc("/create-issue", createIssueHandler)

	http.HandleFunc("/get-leaderboard", getLeaderboardHandler)
	http.HandleFunc("/update-simpleshades-score", updateSimpleShadesScoreHandler)

	http.HandleFunc("/get-stackerz-leaderboard", getStackerzLeaderboardHandler)
	http.HandleFunc("/update-stackerz-score", updateStackerzScoreHandler)

	http.HandleFunc("/get-open-threads", getOpenThreadsHandler)

	http.HandleFunc("/get-users-subscribed-threads", getUsersSubscribedThreadsHandler)
	http.HandleFunc("/change-if-notified-for-thread", changeUserSubscriptionToThreadHandler)

	http.HandleFunc("/signup", signUpHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/reset-password", getResetPasswordCodeHandler)
	http.HandleFunc("/reset-password-with-code", resetPasswordHandler)
	http.HandleFunc("/update-admin-pass", updateAdminPassHandler)

	http.HandleFunc("/jwt-validation-endpoint", validateJWTHandler)
	http.HandleFunc("/refresh-token", refreshTokenHandler)
	http.HandleFunc("/delete-jwt", deleteJWTHandler)

	//http.HandleFunc("/upload-file", h3)
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("js"))))
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	log.Fatal(http.ListenAndServe(":80", nil))
	// For production
	//log.Fatal(http.ListenAndServeTLS(":443", "../tflserver.crt", "../tflserver.key", nil))
}

/*
	func cookieExpirationCheck(w http.ResponseWriter, r *http.Request, db *sql.DB) {
		c, err := r.Cookie("session_id")

		if err != nil {
			if err == http.ErrNoCookie {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if c.Valid() != nil {
			fmt.Println("Cook is no longer valid")
		}

		var sessionUser string
		var expiryTemp time.Time
		//var ipAddr string
		row := db.QueryRow(fmt.Sprintf("select username, expiry from tfldata.sessions where session_token='%s';", c.Value))
		row.Scan(&sessionUser, &expiryTemp)

		if time.Until(expiryTemp) < (time.Minute * 5) {
			setLoginCookie(w, db, sessionUser, r)
		} else if time.Until(expiryTemp) <= (time.Minute * 1) {

			_, seshClearErr := db.Exec(fmt.Sprintf("delete from tfldata.sessions where session_token='%s';", c.Value))
			if seshClearErr != nil {
				fmt.Println(seshClearErr)
			}
			_, usersEditErr := db.Exec(fmt.Sprintf("update tfldata.users set session_token=null where session_token='%s';", c.Value))
			if usersEditErr != nil {
				fmt.Println(usersEditErr)
			}
		}

}
*/
func setLoginCookie(w http.ResponseWriter, db *sql.DB, userStr string, r *http.Request) {

	sessionToken := uuid.NewString()
	expiresAt := time.Now().Add(3600 * time.Hour)
	//fmt.Println(expiresAt.Local().Format(time.DateTime))
	//fmt.Println(userStr)
	/*_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.sessions(\"username\", \"session_token\", \"expiry\", \"ip_addr\") values('%s', '%s', '%s', '%s') on conflict(ip_addr) do update set session_token='%s', expiry='%s';", userStr, sessionToken, expiresAt.Format(time.DateTime), strings.Split(r.RemoteAddr, ":")[0], sessionToken, expiresAt.Format(time.DateTime)))
	if inserterr != nil {
		db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", inserterr))
		fmt.Println(inserterr)
	}*/
	_, updateerr := db.Exec(fmt.Sprintf("update tfldata.users set session_token='%s' where username='%s' or email='%s';", sessionToken, userStr, userStr))
	if updateerr != nil {
		db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s');", updateerr, time.Now().In(nyLoc).Format(time.DateTime)))
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
func uploadPfpToS3(k string, s string, f multipart.File, fn string, r *http.Request, formInputIdentifier string) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(k, s, "")),
	)
	//conf, err := config.NewEnvConfig(config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(k, s, "")))
	if err != nil {
		log.Fatal(err)
		os.Exit(9)
	}

	client := s3.NewFromConfig(cfg)

	defer f.Close()
	ourfile, fileHeader, errfile := r.FormFile(formInputIdentifier)

	if errfile != nil {
		log.Fatal(errfile)
	}

	fileContents := make([]byte, fileHeader.Size)

	ourfile.Read(fileContents)
	filetype := http.DetectContentType(fileContents)

	defer ourfile.Close()

	_, err4 := client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:       aws.String(s3Domain),
		Key:          aws.String("pfp/" + fn),
		Body:         f,
		ContentType:  &filetype,
		CacheControl: aws.String("max-age=86400"),
	})

	if err4 != nil {
		fmt.Println("error on upload")
		fmt.Println(err.Error())
	}

}
func uploadFileToS3(k string, s string, bucketexists bool, f multipart.File, fn string, r *http.Request) string {

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(k, s, "")),
	)

	if err != nil {
		log.Fatal(err)
		os.Exit(4)
	}

	client := s3.NewFromConfig(cfg)

	defer f.Close()
	ourfile, fileHeader, errfile := r.FormFile("file_name")

	if errfile != nil {
		//log.Fatal(errfile)
		fmt.Println(errfile)
	}

	fileContents := make([]byte, fileHeader.Size)

	ourfile.Read(fileContents)
	filetype := http.DetectContentType(fileContents)

	if strings.Contains(filetype, "image") {
		_, err4 := client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:       aws.String(s3Domain),
			Key:          aws.String("posts/images/" + fn),
			Body:         f,
			ContentType:  &filetype,
			CacheControl: aws.String("max-age=86400"),
		})

		if err4 != nil {
			fmt.Println("error on upload")
			fmt.Println(err4.Error())
		}
	} else {

		_, err4 := client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:      aws.String(s3Domain),
			Key:         aws.String("posts/videos/" + fn),
			Body:        f,
			ContentType: &filetype,
		})

		if err4 != nil {
			fmt.Println("error on upload")
			fmt.Println(err4.Error())
		}

	}
	defer ourfile.Close()
	return filetype
}

func sendNotificationToTaggedUser(w http.ResponseWriter, r *http.Request, fcmToken string, db *sql.DB, message string, app *firebase.App) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	fb_message_client, _ := app.Messaging(context.TODO())
	typePayload := make(map[string]string)
	typePayload["type"] = "groupchat"
	sentRes, sendErr := fb_message_client.Send(context.TODO(), &messaging.Message{
		Token: fcmToken,

		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: "Someone tagged you",
				Body:  message,
				Data:  typePayload,
				/*Actions: []*messaging.WebpushNotificationAction{
					{
						Action: "Open",
						Title:  "Open message",
						Icon:   "assets/android-chrome-512x512.png",
					},
				},*/
			},
		},
	})
	if sendErr != nil {
		fmt.Print(sendErr)
	}
	db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sentRes, time.Now().In(nyLoc).Format(time.DateTime)))
}

func sendNotificationToAllUsers(db *sql.DB, fbapp firebase.App, curUser string, opts notificationOpts) {

	output, outerr := db.Query(fmt.Sprintf("select username, is_subscribed from tfldata.users_to_threads where thread='%s' and username != '%s';", opts.extraPayloadVal, curUser))
	if outerr != nil {
		fmt.Println(outerr)
	}

	defer output.Close()
	fb_message_client, _ := fbapp.Messaging(context.TODO())
	typePayload := make(map[string]string)
	typePayload["type"] = opts.notificationPage
	typePayload[opts.extraPayloadKey] = opts.extraPayloadVal
	for output.Next() {
		var userToSend string
		var isSubbed bool
		output.Scan(&userToSend, &isSubbed)
		if isSubbed {
			var fcmToken string
			var sendRes string
			var sendErr error

			tokenRow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username='%s';", userToSend))
			scnerr := tokenRow.Scan(&fcmToken)

			if scnerr != nil {
				fmt.Println("ignore scan error for: " + userToSend)
			} else {

				sendRes, sendErr = fb_message_client.Send(context.TODO(), &messaging.Message{

					Token: fcmToken,
					Notification: &messaging.Notification{
						Title: opts.notificationTitle,
						Body:  opts.notificationBody,
					},
					Webpush: &messaging.WebpushConfig{
						Notification: &messaging.WebpushNotification{
							Title: opts.notificationTitle,
							Body:  opts.notificationBody,
							Data:  typePayload,
						},
					},
					Android: &messaging.AndroidConfig{
						Notification: &messaging.AndroidNotification{
							Title: opts.notificationTitle,
							Body:  opts.notificationBody,
						},
					},
				})
			}
			if sendErr != nil {
				//fmt.Print(sendErr.Error() + " for user: " + userToSend)
				if strings.Contains(sendErr.Error(), "404") {
					db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id=null where username='%s';", userToSend))
					fmt.Println("updated " + userToSend + "\\'s fcm token")
				}
			}
			db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sendRes, time.Now().In(nyLoc).Local().Format(time.DateTime)))

		}
	}

}
func generateLoginJWT(username string, w http.ResponseWriter, r *http.Request, jwtKey string) *jwt.Token {
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
func validateJWTToken(tokenStr string, tokenKey string, w http.ResponseWriter) bool {
	jwtToken, jwtValidateErr := jwt.Parse(tokenStr, func(jwtToken *jwt.Token) (interface{}, error) {

		return []byte(tokenKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if jwtValidateErr != nil {
		return false
	}
	return jwtToken.Valid
}
