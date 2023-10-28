package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"firebase.google.com/go/messaging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/go-github/github"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/option"
)

type Postsrow struct {
	Id           int64
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
	Username string
	Pfpname  string
}

var awskey string
var awskeysecret string
var ghissuetoken string
var nyLoc *time.Location

func main() {
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

	//fbapikey := os.Getenv("FIREBASE_API_KEY")

	/*fbauthdomain := os.Getenv("FIREBASE_AUTH_DOMAIN")
	fbprojectid := os.Getenv("FIREBASE_PROJECT_ID")
	fbstoragebucket := os.Getenv("FIREBASE_STORAGE_BUCKET")
	fbmessagesenderid := os.Getenv("FIREBASE_MESSAGING_SENDER_ID")
	fbappid := os.Getenv("FIREBASE_APP_ID")
	fbconfig := os.Getenv("FIREBASE_CONFIG")*/

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

	var postTmpl *template.Template
	var tmerr error

	subscriptionHandler := func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		bs, _ := io.ReadAll(r.Body)

		type postBody struct {
			Fcmtoken string `json:"fcm_token"`
			Username string `json:"username"`
		}
		var postData postBody
		psdmae := json.Unmarshal(bs, &postData)
		if psdmae != nil {
			fmt.Print(psdmae)
		}

		_, inserr := db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id='%s' where session_token='%s';", postData.Fcmtoken, postData.Username))
		if inserr != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", inserr, time.Now().In(nyLoc).Format(time.DateTime)))
		}

	}

	newPostsHandlerPushNotify := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		bs, _ := io.ReadAll(r.Body)

		type postBody struct {
			Id string `json:"id"`
		}
		var postData postBody
		marshErr := json.Unmarshal(bs, &postData)
		if marshErr != nil {
			fmt.Print(marshErr)
		}

		var fcmToken string
		tokenRow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where session_token='%s';", postData.Id))
		scnerr := tokenRow.Scan(&fcmToken)

		if scnerr != nil {
			fmt.Println(scnerr)
		}

		fb_message_client, _ := app.Messaging(context.TODO())

		sentRes, sendErr := fb_message_client.Send(context.TODO(), &messaging.Message{
			Token: fcmToken,
			Notification: &messaging.Notification{
				Title: "There's a new post!",
				Body:  "Somebody just made a new post!",
			},

			Webpush: &messaging.WebpushConfig{
				Notification: &messaging.WebpushNotification{
					Title: "There's a new post!",
					Body:  "Somebody just made a new post!",
				},
			},
			Android: &messaging.AndroidConfig{
				Notification: &messaging.AndroidNotification{
					Title: "There's a new post!",
					Body:  "Somebody just made a new post!",
				},
			},
		})
		if sendErr != nil {
			fmt.Print(sendErr)
		}
		db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\") values('%s');", sentRes))
	}

	signUpHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/form-data")
		fb_auth_client, clienterr := app.Auth(context.TODO())
		if clienterr != nil {
			fmt.Println(clienterr)
		}

		if r.PostFormValue("passwordsignup") != r.PostFormValue("confirmpasswordsignup") {
			w.WriteHeader(http.StatusUnauthorized)
			return
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
		record, usererr := fb_auth_client.CreateUser(context.TODO(), (&auth.UserToCreate{}).DisplayName(strings.ToLower(r.PostFormValue("usernamesignup"))).Email(strings.ToLower(r.PostFormValue("emailsignup"))).Password(r.PostFormValue("passwordsignup")).PhotoURL(fmt.Sprintf("https://d33gjmrumfzeah.cloudfront.net/pfp/%s", filename.Filename)))
		if usererr != nil {
			fmt.Println(usererr)
			w.WriteHeader(http.StatusConflict)
			return
		}

		// TODO: Add pfp insert
		_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.users(\"username\", \"password\", \"pfp_name\", \"email\", \"firebase_user_uid\") values('%s', '%s', '%s', '%s', '%s');", strings.ToLower(r.PostFormValue("usernamesignup")), bytesOfPass, filename.Filename, strings.ToLower(r.PostFormValue("emailsignup")), record.UID))

		if errinsert != nil {
			fmt.Println(errinsert)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", errinsert, time.Now().In(nyLoc).Format(time.DateTime)))
			w.WriteHeader(http.StatusBadRequest)
		}

	}

	loginHandler := func(w http.ResponseWriter, r *http.Request) {
		/*var userUid string
		fb_auth_client, clienterr := app.Auth(context.TODO())
		if clienterr != nil {
			fmt.Println(clienterr)
		}*/

		userStr := strings.ToLower(r.PostFormValue("usernamelogin"))
		/*userIdRow := db.QueryRow(fmt.Sprintf("select firebase_user_uid from tfldata.users where username='%s';", userStr))
		userScnErr := userIdRow.Scan(&userUid)
		if userScnErr != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", userScnErr))
		}
		_, loginerr := fb_auth_client.GetUser(context.Background(), userUid)

		if loginerr != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", loginerr))

		}*/

		var password string
		passScan := db.QueryRow(fmt.Sprintf("select password from tfldata.users where username='%s' or email='%s';", userStr, userStr))
		scnerr := passScan.Scan(&password)
		if scnerr != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('this was the scan error %s with dbpassword %s and form user is %s');", scnerr, password, userStr))
			fmt.Print(scnerr)
		}
		err := bcrypt.CompareHashAndPassword([]byte(password), []byte(r.PostFormValue("passwordlogin")))

		if err != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))
			w.Header().Set("HX-Trigger", "loginevent")
		} else if err == nil {
			setLoginCookie(w, db, userStr, r)

			w.Header().Set("HX-Refresh", "true")
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
		default:
			tmpl := template.Must(template.ParseFiles("index.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		}

	}

	getPostsHandler := func(w http.ResponseWriter, r *http.Request) {

		output, err := db.Query("select id, title, description, author, post_files_key from tfldata.posts order by id DESC;")
		var count string
		db.QueryRow("select count(*) from tfldata.posts;").Scan(&count)

		var dataStr string
		if err != nil {
			log.Fatal(err)
		}

		defer output.Close()

		for output.Next() {
			var postrows Postsrow

			//if err := output.Scan(&postrows.Id, &postrows.Title, &postrows.Description, &postrows.File_name, &postrows.File_type, &postrows.Author); err != nil {
			if err := output.Scan(&postrows.Id, &postrows.Title, &postrows.Description, &postrows.Author, &postrows.Postfileskey); err != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))

			}

			comment := db.QueryRow(fmt.Sprintf("select count(*) from tfldata.comments where post_id='%d';", postrows.Id))
			var commentCount string
			comment.Scan(&commentCount)
			var countOfImg int32
			rowCount := db.QueryRow(fmt.Sprintf("select count(*) from tfldata.postfiles where post_files_key='%s';", postrows.Postfileskey))
			rowCount.Scan(&countOfImg)
			var firstImg struct {
				filename string
				filetype string
			}
			firstRow := db.QueryRow(fmt.Sprintf("select file_name, file_type from tfldata.postfiles where post_files_key='%s' order by id desc limit 1;", postrows.Postfileskey))
			firstRow.Scan(&firstImg.filename, &firstImg.filetype)

			// TODO cache images
			if strings.Contains(firstImg.filetype, "image") {
				/*imgclient := http.Client{}

				imgreq, _ := http.NewRequest("GET", fmt.Sprintf("https://d33gjmrumfzeah.cloudfront.net/posts/images/%s", postrows.File_name), nil)

				imgreq.Header.Set("Cache-Control", "max-age=86400")
				resp, _ := imgclient.Do(imgreq)*/
				if countOfImg > 1 {
					dataStr = fmt.Sprintf("<div class='card my-4' style='border-radius: 106px 106px / 91px; box-shadow: 12px 12px 2px 1px rgb(41 88 93 / 20&percnt;);'><img class='img-fluid' id='%s' src='https://d33gjmrumfzeah.cloudfront.net/posts/images/%s' alt='%s' style='border-radius: 65px 65px / 50px;' alt='default' /><div class='p-2' style='display: flex; justify-content: space-around;'><i onclick='nextLeftImage(`%s`)' class='bi bi-arrow-90deg-left'></i><i onclick='nextRightImage(`%s`)' class='bi bi-arrow-90deg-right'></i></div><div class='card-body'><h5 class='card-title'>%s - %s</h5><p class='card-text'>%s</p><button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' class='btn btn-primary'>Comments (%s)</button></div></div>", postrows.Postfileskey, firstImg.filename, firstImg.filename, postrows.Postfileskey, postrows.Postfileskey, postrows.Title, postrows.Author, postrows.Description, postrows.Id, postrows.Id, commentCount)
				} else if countOfImg == 1 {
					dataStr = fmt.Sprintf("<div class='card my-4' style='border-radius: 106px 106px / 91px; box-shadow: 12px 12px 2px 1px rgb(41 88 93 / 20&percnt;);'><img class='img-fluid' id='%s' src='https://d33gjmrumfzeah.cloudfront.net/posts/images/%s' alt='%s' style='border-radius: 65px 65px / 50px;' alt='default' /><div class='p-2' style='display: flex; justify-content: space-around;'></div><div class='card-body'><h5 class='card-title'>%s - %s</h5><p class='card-text'>%s</p><button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' class='btn btn-primary'>Comments (%s)</button></div></div>", postrows.Postfileskey, firstImg.filename, firstImg.filename, postrows.Title, postrows.Author, postrows.Description, postrows.Id, postrows.Id, commentCount)
				}
				//imgclient.CloseIdleConnections()
				//defer resp.Body.Close()
			} else if strings.Contains(firstImg.filetype, "video") || strings.Contains(firstImg.filetype, "octet-stream") {
				dataStr = fmt.Sprintf("<div class='card my-4' style='border-radius: 106px 106px / 91px; box-shadow: 12px 12px 2px 1px rgb(41 88 93 / 20&percnt;);'><video style='border-radius: 65px 65px / 91px;' controls id='video'><source src='https://d33gjmrumfzeah.cloudfront.net/posts/videos/%s'></video><div class='p-2' style='display: flex; justify-content: space-around;'></div><div class='card-body'><h5 class='card-title'>%s - %s</h5><p class='card-text'>%s</p><button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' class='btn btn-primary'>Comments (%s)</button></div></div>", firstImg.filename, postrows.Title, postrows.Author, postrows.Description, postrows.Id, postrows.Id, commentCount)
			}

			postTmpl, tmerr = template.New("tem").Parse(dataStr)
			if tmerr != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", tmerr, time.Now().In(nyLoc).Format(time.DateTime)))
			}
			postTmpl.Execute(w, nil)
		}

	}

	getPostCountHandler := func(w http.ResponseWriter, r *http.Request) {

		var count string
		db.QueryRow("select count(*) from tfldata.posts;").Scan(&count)

		dataStr := "<script>dbCount = " + count + "</script>"
		tmp, err := template.New("but").Parse(dataStr)
		if err != nil {
			fmt.Println(err)
		}
		tmp.Execute(w, nil)

	}

	createPostHandler := func(w http.ResponseWriter, r *http.Request) {
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
		var username string
		row := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", c.Value))
		row.Scan(&username)

		postFilesKey := uuid.NewString()

		_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"author\", \"post_files_key\") values('%s', '%s', '%s', '%s');", r.PostFormValue("title"), r.PostFormValue("description"), username, postFilesKey))

		if errinsert != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", errinsert, time.Now().In(nyLoc).Format(time.DateTime)))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

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
			filetype := createTFLBucketAndUpload(awskey, awskeysecret, false, f, fh.Filename, r)

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
			filetype := createTFLBucketAndUpload(awskey, awskeysecret, false, upload, filename.Filename, r)

			_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"file_name\", \"file_type\", \"author\") values('%s', '%s', '%s', '%s', '%s');", r.PostFormValue("title"), r.PostFormValue("description"), filename.Filename, filetype, username))

			if errinsert != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", errinsert))
			}*/
		//defer upload.Close()

	}
	getSelectedPostsComments := func(w http.ResponseWriter, r *http.Request) {
		type postComment struct {
			Comment string
			Author  string
		}

		var commentTmpl *template.Template

		output, err := db.Query(fmt.Sprintf("select comment, author from tfldata.comments where post_id='%s'::integer order by post_id desc;", r.URL.Query().Get("post-id")))

		var dataStr string
		if err != nil {
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))
		}

		defer output.Close()

		for output.Next() {
			var posts postComment

			if err := output.Scan(&posts.Comment, &posts.Author); err != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))

			}
			dataStr = "<p class='p-2'>" + posts.Comment + " - " + posts.Author + "</p>"

			commentTmpl, err = template.New("com").Parse(dataStr)
			if err != nil {
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", err, time.Now().In(nyLoc).Format(time.DateTime)))
			}
			commentTmpl.Execute(w, nil)
		}

	}
	createEventCommentHandler := func(w http.ResponseWriter, r *http.Request) {
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

		_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.comments(\"comment\", \"event_id\", \"author\") values('%s', '%d', (select username from tfldata.users where session_token='%s'));", postData.Eventcomment, postData.CommentSelectedEventId, c.Value))
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

		_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.comments(\"comment\", \"post_id\", \"author\") values('%s', '%d', (select username from tfldata.users where session_token='%s'));", postData.Comment, postData.SelectedPostId, c.Value))
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

	}
	getEventsHandler := func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/json; charset=utf-8")

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

	createEventHandler := func(w http.ResponseWriter, r *http.Request) {
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
		_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.calendar(\"start_date\", \"event_owner\", \"event_details\", \"event_title\") values('%s', (select username from tfldata.users where session_token='%s'), '%s', '%s');", postData.Startdate, c.Value, postData.Eventdetails, postData.Eventtitle))
		if inserterr != nil {
			fmt.Println(inserterr)
			w.WriteHeader(http.StatusBadRequest)
		}

	}
	createGroupChatMessageHandler := func(w http.ResponseWriter, r *http.Request) {
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
		chatMessage := r.PostFormValue("gchatmessage")
		taggedUser := r.PostFormValue("taggedUser")
		var userName string
		var fcmRegToken string
		userNameRow := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", c.Value))
		userNameRow.Scan(&userName)
		if taggedUser > "" {
			fcmRegRow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username='%s';", taggedUser))
			scnerr := fcmRegRow.Scan(&fcmRegToken)
			if scnerr != nil {
				fmt.Println(scnerr)
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('%s', '%s')", scnerr, time.Now().Local().Format(time.DateTime)))
			}
			sendNotificationToTaggedUser(w, r, fcmRegToken, db, chatMessage, app)
		}

		_, inserr := db.Exec(fmt.Sprintf("insert into tfldata.gchat(\"chat\", \"author\", \"createdon\") values('%s', '%s', '%s');", chatMessage, userName, time.Now().In(nyLoc).Format(time.DateTime)))
		if inserr != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Header().Set("HX-Trigger", "success-send")

	}
	getGroupChatMessagesHandler := func(w http.ResponseWriter, r *http.Request) {
		output, err := db.Query("select chat, author, createdon from (select * from tfldata.gchat order by id DESC limit 20) as tmp order by createdon asc;")
		if err != nil {
			fmt.Println(err)
		}
		defer output.Close()
		for output.Next() {

			var message string
			var author string
			var createdat time.Time
			var formatCreatedatTime string

			output.Scan(&message, &author, &createdat)
			if time.Now().UTC().Sub(createdat) > (24 * time.Hour) {
				formatCreatedatTime = time.ANSIC
				formatCreatedatTime = strings.Split(formatCreatedatTime, " ")[0]
			} else if time.Now().UTC().Sub(createdat) > (72 * time.Hour) {
				formatCreatedatTime = time.DateOnly
			} else {
				formatCreatedatTime = time.Kitchen
			}

			dataStr := "<div style='max-width: 100%; background-color: rgb(107 166 254 / .3);' class='container my-1'><div class='row'><b class='col-2 px-1'>" + author + "</b><p class='col-10 my-0' style='padding-top: 1rem!important'>" + message + "</p></div><div class='row'><p class='col' style='margin-left: 75%; font-size: small;'>&nbsp;&nbsp" + createdat.Format(formatCreatedatTime) + "</p></div></div>"
			chattmp, tmperr := template.New("gchat").Parse(dataStr)
			if tmperr != nil {
				fmt.Println(tmperr)
			}
			chattmp.Execute(w, nil)

		}
	}
	getUsernamesToTagHandler := func(w http.ResponseWriter, r *http.Request) {

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
		var imgList []string
		rows, err := db.Query(fmt.Sprintf("select file_name from tfldata.postfiles where post_files_key='%s';", r.URL.Query().Get("id")))
		if err != nil {
			fmt.Println(err)
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
	getSessionDataHandler := func(w http.ResponseWriter, r *http.Request) {

		var ourSeshStruct seshStruct

		row := db.QueryRow(fmt.Sprintf("select username, pfp_name from tfldata.users where session_token='%s';", r.URL.Query().Get("id")))
		scnerr := row.Scan(&ourSeshStruct.Username, &ourSeshStruct.Pfpname)
		if scnerr != nil {
			fmt.Println(scnerr)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		data, err := json.Marshal(&ourSeshStruct)
		if err != nil {
			fmt.Println(err)
		}

		w.Write(data)
	}
	clearCookieHandler := func(w http.ResponseWriter, r *http.Request) {

		c, _ := r.Cookie("session_id")

		/*_, seshClearErr := db.Exec(fmt.Sprintf("delete from tfldata.sessions where session_token='%s';", c.Value))
		if seshClearErr != nil {
			fmt.Println(seshClearErr)
		}*/
		_, usersEditErr := db.Exec(fmt.Sprintf("update tfldata.users set session_token=null where session_token='%s';", c.Value))
		if usersEditErr != nil {
			fmt.Println(usersEditErr)
		}

	}
	updatePfpHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/form-data")
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
	createIssueHandler := func(w http.ResponseWriter, r *http.Request) {
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
		bodyText := fmt.Sprintf("%s on %s page - %s", postData.Descdetail[1], postData.Descdetail[0], username)
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
	/*h3 := func(w http.ResponseWriter, r *http.Request) {
		upload, filename, err := r.FormFile("file_name")
		if err != nil {
			log.Fatal(err)
		}

		//uploadPostPhotoTos3(upload, filename.Filename, s3_client)

	}*/
	http.HandleFunc("/", pagesHandler)
	http.HandleFunc("/create-post", createPostHandler)

	http.HandleFunc("/get-posts", getPostsHandler)
	http.HandleFunc("/new-posts", getPostCountHandler)

	http.HandleFunc("/get-selected-post", getSelectedPostsComments)
	http.HandleFunc("/get-events", getEventsHandler)
	http.HandleFunc("/get-event-comments", getSelectedEventsComments)

	http.HandleFunc("/get-post-images", getPostImagesHandler)

	http.HandleFunc("/create-comment", createCommentHandler)
	http.HandleFunc("/create-event-comment", createEventCommentHandler)

	http.HandleFunc("/get-username-from-session", getSessionDataHandler)

	http.HandleFunc("/clear-cookie", clearCookieHandler)

	http.HandleFunc("/create-event", createEventHandler)

	http.HandleFunc("/group-chat-messages", getGroupChatMessagesHandler)
	http.HandleFunc("/create-a-group-chat-message", createGroupChatMessageHandler)
	http.HandleFunc("/get-all-users-to-tag", getUsernamesToTagHandler)

	http.HandleFunc("/create-subscription", subscriptionHandler)
	http.HandleFunc("/send-new-posts-push", newPostsHandlerPushNotify)

	http.HandleFunc("/update-pfp", updatePfpHandler)

	http.HandleFunc("/create-issue", createIssueHandler)

	http.HandleFunc("/signup", signUpHandler)
	http.HandleFunc("/login", loginHandler)

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
	expiresAt := time.Now().Add(4320 * time.Minute)
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
		Bucket:       aws.String("the-family-loop" + "-customer-hash"),
		Key:          aws.String("pfp/" + fn),
		Body:         f,
		ContentType:  &filetype,
		CacheControl: aws.String("max-age=86400"),
	})

	if err4 != nil {
		fmt.Println("error on upload")
		fmt.Println(err)
	}

}
func createTFLBucketAndUpload(k string, s string, bucketexists bool, f multipart.File, fn string, r *http.Request) string {

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(k, s, "")),
	)

	if err != nil {
		log.Fatal(err)
		os.Exit(4)
	}

	client := s3.NewFromConfig(cfg)

	listbuck, err := client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})

	if err != nil {
		log.Fatal(err)
	}
	for _, val := range listbuck.Buckets {
		if strings.EqualFold(*val.Name, *aws.String("the-family-loop" + "-customer-hash")) {
			//fmt.Println("Bucket exists!")
			bucketexists = true
		} else {
			//fmt.Println("lets create the bucket")
			bucketexists = false
		}
	}
	if !bucketexists {
		_, err := client.CreateBucket(context.TODO(),
			&s3.CreateBucketInput{
				Bucket: aws.String("the-family-loop" + "-customer-hash"),
			},
		)
		if err != nil {
			log.Fatal(err)
		}
	}
	_, err2 := client.PutPublicAccessBlock(context.TODO(),
		&s3.PutPublicAccessBlockInput{
			Bucket: aws.String("the-family-loop" + "-customer-hash"),
			PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
				BlockPublicPolicy:     false,
				BlockPublicAcls:       false,
				RestrictPublicBuckets: false,
				IgnorePublicAcls:      true,
			},
		})
	if err2 != nil {
		log.Fatal(err2)

	}
	_, err3 := client.PutBucketPolicy(context.TODO(),
		&s3.PutBucketPolicyInput{
			Bucket: aws.String("the-family-loop" + "-customer-hash"),
			Policy: aws.String(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Sid": "Statement",
			"Effect": "Allow",
			"Principal": {
			    "Service": "cloudfront.amazonaws.com"
			},
			"Action": [
				"s3:GetObject*",
				"s3:PutObject*"
			],
			"Resource": [
				"arn:aws:s3:::the-family-loop-customer-hash/posts/*",
				"arn:aws:s3:::the-family-loop-customer-hash/pfp/*"
			],
			"Condition": {
                    "StringEquals": {
                      "AWS:SourceArn": "arn:aws:cloudfront::529465713677:distribution/EYETELDNATROU"
                    }
                }
		}
	]
}`),
		})
	if err3 != nil {
		fmt.Println(err3)
	}

	defer f.Close()
	ourfile, fileHeader, errfile := r.FormFile("file_name")

	if errfile != nil {
		log.Fatal(errfile)
	}

	fileContents := make([]byte, fileHeader.Size)

	ourfile.Read(fileContents)
	filetype := http.DetectContentType(fileContents)

	defer ourfile.Close()

	if strings.Contains(filetype, "image") {

		_, err4 := client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:       aws.String("the-family-loop" + "-customer-hash"),
			Key:          aws.String("posts/images/" + fn),
			Body:         f,
			ContentType:  &filetype,
			CacheControl: aws.String("max-age=86400"),
		})

		if err4 != nil {
			fmt.Println("error on upload")
			fmt.Println(err)
		}
	} else {

		_, err4 := client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:      aws.String("the-family-loop" + "-customer-hash"),
			Key:         aws.String("posts/videos/" + fn),
			Body:        f,
			ContentType: &filetype,
		})

		if err4 != nil {
			fmt.Println("error on upload")
			fmt.Println(err)
		}

	}
	return filetype
}

func sendNotificationToTaggedUser(w http.ResponseWriter, r *http.Request, fcmToken string, db *sql.DB, message string, app *firebase.App) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	fb_message_client, _ := app.Messaging(context.TODO())
	sentRes, sendErr := fb_message_client.Send(context.TODO(), &messaging.Message{
		Token: fcmToken,

		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: "Someone tagged you",
				Body:  message,
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

/*func uploadPostPhotoTos3(f multipart.File, fn string, client *s3.Client) {

}*/
