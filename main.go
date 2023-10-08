package main

import (
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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type Postsrow struct {
	Id          int64
	Title       string
	Description string
	File_name   string
	File_type   string
	Author      string
}
type seshStruct struct {
	Username string
	Pfpname  string
}

var awskey string
var awskeysecret string

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		os.Exit(1)
	}
	dbpass := os.Getenv("DB_PASS")
	awskey = os.Getenv("AWS_ACCESS_KEY")
	awskeysecret = os.Getenv("AWS_ACCESS_SECRET")

	// Connect to database
	connStr := fmt.Sprintf("postgresql://tfldbrole:%s@localhost/tfl?sslmode=disable", dbpass)
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var postTmpl *template.Template
	var tmerr error
	signUpHandler := func(w http.ResponseWriter, r *http.Request) {

		if r.PostFormValue("passwordsignup") != r.PostFormValue("confirmpasswordsignup") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		upload, filename, errfile := r.FormFile("pfpformfile")
		if errfile != nil {
			fmt.Println(errfile)
			w.WriteHeader(http.StatusBadRequest)
		}
		uploadPfpToS3(awskey, awskeysecret, false, upload, filename.Filename, r)
		bs := []byte(r.PostFormValue("passwordsignup"))

		bytesOfPass, err := bcrypt.GenerateFromPassword(bs, len(bs))

		if err != nil {
			fmt.Println(err)
		}
		// TODO: Add pfp insert
		_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.users(\"username\", \"password\", \"pfp_name\") values('%s', '%s', '%s');", r.PostFormValue("usernamesignup"), bytesOfPass, filename.Filename))

		if errinsert != nil {
			fmt.Println(errinsert)
			w.WriteHeader(http.StatusBadRequest)
		}

	}

	loginHandler := func(w http.ResponseWriter, r *http.Request) {
		userStr := r.PostFormValue("usernamelogin")
		var password string
		db.QueryRow(fmt.Sprintf("select password from tfldata.users where username='%s';", userStr)).Scan(&password)

		err := bcrypt.CompareHashAndPassword([]byte(password), []byte(r.PostFormValue("passwordlogin")))

		if err != nil {
			w.Header().Set("HX-Trigger", "loginevent")
		} else if err == nil {
			setLoginCookie(w, db, userStr)
			w.Header().Set("HX-Refresh", "true")
		}

	}

	pagesHandler := func(w http.ResponseWriter, r *http.Request) {

		//tmpl := template.Must(template.ParseFiles("index.html"))

		//tmpl.Execute(w, nil)

		bs, _ := os.ReadFile("navigation.html")
		navtmple := template.New("Navt")
		tm, _ := navtmple.Parse(string(bs))

		if err != nil {
			fmt.Println(err)
		}

		switch r.URL.Path {
		case "/home":

			go cookieExpirationCheck(w, r, db)
			tmpl := template.Must(template.ParseFiles("index.html"))
			tmpl.Execute(w, nil)
			tm.ExecuteTemplate(w, "Navt", nil)
		case "/calendar":
			tmpl := template.Must(template.ParseFiles("calendar.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)

		case "/":
			http.Redirect(w, r, "/home", http.StatusMovedPermanently)
		default:
			tmpl := template.Must(template.ParseFiles("404.html"))
			tmpl.Execute(w, nil)
			tm.Execute(w, nil)
		}
	}

	getPostsHandler := func(w http.ResponseWriter, r *http.Request) {

		output, err := db.Query("select id, title, description, file_name, file_type, author from tfldata.posts order by id DESC;")
		var count string
		db.QueryRow("select count(*) from tfldata.posts;").Scan(&count)

		var dataStr string
		if err != nil {
			log.Fatal(err)
		}

		defer output.Close()

		for output.Next() {
			var postrows Postsrow

			if err := output.Scan(&postrows.Id, &postrows.Title, &postrows.Description, &postrows.File_name, &postrows.File_type, &postrows.Author); err != nil {
				log.Fatal(err)

			}

			// TODO cache images
			if strings.Contains(postrows.File_type, "image") {
				imgclient := http.Client{}

				imgreq, _ := http.NewRequest("GET", fmt.Sprintf("https://the-family-loop-customer-hash.s3.amazonaws.com/posts/images/%s", postrows.File_name), nil)

				imgreq.Header.Set("Cache-Control", "max-age=86400")
				resp, _ := imgclient.Do(imgreq)

				dataStr = fmt.Sprintf("<div class='card my-4' style='border-radius: 14px;'><img src='%s' style='border-radius: 14px;' alt='%s' /><div class='card-body'><h5 class='card-title'>%s</h5><p class='card-text'>%s</p><button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' class='btn btn-primary'>Open a post</button></div></div>", resp.Request.URL, postrows.File_name, postrows.Title, postrows.Description, postrows.Id, postrows.Id)
				imgclient.CloseIdleConnections()
				defer resp.Body.Close()
			} else if strings.Contains(postrows.File_type, "video") {
				dataStr = fmt.Sprintf("<div class='card my-4' style='border-radius: 14px;'><video controls id='video'><source src='https://the-family-loop-customer-hash.s3.amazonaws.com/posts/videos/%s' type='%s'></video><div class='card-body'><h5 class='card-title'>%s</h5><p class='card-text'>%s</p><button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' class='btn btn-primary'>Open a post</button></div></div>", postrows.File_name, postrows.File_type, postrows.Title, postrows.Description, postrows.Id, postrows.Id)
			}

			postTmpl, tmerr = template.New("tem").Parse(dataStr)
			if tmerr != nil {
				fmt.Print(tmerr)
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
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var username string
		row := db.QueryRow(fmt.Sprintf("select username from users where session_token='%s'", c))
		row.Scan(&username)
		upload, filename, errfile := r.FormFile("file_name")

		if errfile != nil {
			log.Fatal(errfile)
		}

		// Returning a filetype from the createandupload function
		// somehow gets the right filetype
		filetype := createTFLBucketAndUpload(awskey, awskeysecret, false, upload, filename.Filename, r)

		_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"file_name\", \"file_type\", \"author\") values('%s', '%s', '%s', '%s', '%s');", r.PostFormValue("title"), r.PostFormValue("description"), filename.Filename, filetype, username))

		if errinsert != nil {
			log.Fatal(err)
		}
		defer upload.Close()

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
			log.Fatal(err)
		}

		defer output.Close()

		for output.Next() {
			var posts postComment

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
		}

		_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.comments(\"comment\", \"event_id\", \"author\") values('%s', '%d', (select username from tfldata.users where session_token='%s'));", postData.Eventcomment, postData.CommentSelectedEventId, c.Value))
		if inserterr != nil {
			fmt.Println(inserterr)
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
		w.WriteHeader(http.StatusOK)
	}
	getSelectedEventsComments := func(w http.ResponseWriter, r *http.Request) {
		type getBody struct {
			CommentSelectedEventId int `json:"commentSelectedEventID"`
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
		w.WriteHeader(http.StatusOK)
	}
	getEventsHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		type EventData struct {
			Eventid      int
			Startdate    string
			Enddate      string
			Eventowner   string
			Eventdetails string
			Eventtitle   string
		}

		ourEvents := []EventData{}
		output, err := db.Query("select start_date, end_date, event_owner, event_details, event_title, id from tfldata.calendar;")
		if err != nil {
			fmt.Println(err)
		}
		defer output.Close()
		for output.Next() {
			var tempData EventData
			scnerr := output.Scan(&tempData.Startdate, &tempData.Enddate, &tempData.Eventowner, &tempData.Eventdetails, &tempData.Eventtitle, &tempData.Eventid)
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
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if c.Valid() != nil {
			fmt.Println("Cook is no longer valid")
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		bs, _ := io.ReadAll(r.Body)
		type PostBody struct {
			Startdate    string `json:"start_date"`
			Enddate      string `json:"end_date"`
			Eventdetails string `json:"event_details"`
			Eventtitle   string `json:"event_title"`
		}

		var postData PostBody

		errmarsh := json.Unmarshal(bs, &postData)
		if errmarsh != nil {
			fmt.Println(errmarsh)
		}

		_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.calendar(\"start_date\", \"end_date\", \"event_owner\", \"event_details\", \"event_title\") values('%s', '%s', (select username from tfldata.users where session_token='%s'), '%s', '%s');", postData.Startdate, postData.Enddate, c.Value, postData.Eventdetails, postData.Eventtitle))
		if inserterr != nil {
			fmt.Println(inserterr)
			w.WriteHeader(http.StatusBadRequest)
		}

		var author string
		row := db.QueryRow(fmt.Sprintf("select username from tfldata.users where session_token='%s';", c.Value))
		row.Scan(&author)

		w.WriteHeader(http.StatusOK)
	}
	getSessionDataHandler := func(w http.ResponseWriter, r *http.Request) {

		var ourSeshStruct seshStruct
		row := db.QueryRow(fmt.Sprintf("select username, pfp_name from tfldata.users where session_token='%s';", r.URL.Query().Get("id")))
		row.Scan(&ourSeshStruct.Username, &ourSeshStruct.Pfpname)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		data, err := json.Marshal(&ourSeshStruct)
		if err != nil {
			fmt.Println(err)
		}

		w.Write(data)
	}
	clearCookieHandler := func(w http.ResponseWriter, r *http.Request) {

		c, _ := r.Cookie("session_id")

		_, seshClearErr := db.Exec(fmt.Sprintf("delete from tfldata.sessions where session_token='%s';", c.Value))
		if seshClearErr != nil {
			fmt.Println(seshClearErr)
		}
		_, usersEditErr := db.Exec(fmt.Sprintf("update tfldata.users set session_token=null where session_token='%s';", c.Value))
		if usersEditErr != nil {
			fmt.Println(usersEditErr)
		}

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

	http.HandleFunc("/create-comment", createCommentHandler)
	http.HandleFunc("/create-event-comment", createEventCommentHandler)

	http.HandleFunc("/get-username-from-session", getSessionDataHandler)

	http.HandleFunc("/clear-cookie", clearCookieHandler)

	http.HandleFunc("/create-event", createEventHandler)

	http.HandleFunc("/signup", signUpHandler)
	http.HandleFunc("/login", loginHandler)

	//http.HandleFunc("/upload-file", h3)
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
	log.Fatal(http.ListenAndServe(":80", nil))
	// For production
	//log.Fatal(http.ListenAndServeTLS(":443", "./cert.key", "./cert.pem", nil))
}

func cookieExpirationCheck(w http.ResponseWriter, rCookie *http.Request, db *sql.DB) {
	c, err := rCookie.Cookie("session_id")

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
	row := db.QueryRow(fmt.Sprintf("select username, expiry from tfldata.sessions where session_token='%s'", c.Value))
	row.Scan(&sessionUser, &expiryTemp)

	if time.Until(expiryTemp) < (time.Minute * 5) {
		setLoginCookie(w, db, sessionUser)
	} else if time.Until(expiryTemp) == (time.Minute * 1) {
		// TODO update DB records
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
func setLoginCookie(w http.ResponseWriter, db *sql.DB, userStr string) {
	sessionToken := uuid.NewString()
	expiresAt := time.Now().Add(840 * time.Minute)
	//fmt.Println(expiresAt.Local().Format(time.DateTime))
	_, updateerr := db.Exec(fmt.Sprintf("update tfldata.users set session_token='%s' where username='%s';", sessionToken, userStr))
	if updateerr != nil {
		fmt.Println(updateerr)
	}
	_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.sessions(\"username\", \"session_token\", \"expiry\") values('%s', '%s', '%s') on conflict(username) do update set session_token='%s', expiry='%s';", userStr, sessionToken, expiresAt.Format(time.DateTime), sessionToken, expiresAt.Format(time.DateTime)))
	if inserterr != nil {
		fmt.Println(inserterr)
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
func uploadPfpToS3(k string, s string, bucketexists bool, f multipart.File, fn string, r *http.Request) {
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
	ourfile, fileHeader, errfile := r.FormFile("pfpformfile")

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
			Key:          aws.String("pfp/" + fn),
			Body:         f,
			ContentType:  &filetype,
			CacheControl: aws.String("max-age=86400"),
		})

		if err4 != nil {
			fmt.Println("error on upload")
			fmt.Println(err)
		}

	} else {
		fmt.Println("Unknown file type. How did this get here?")
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
			Policy: aws.String(`{"Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Statement",
            "Effect": "Allow",
            "Principal": "*",
            "Action": [
                "s3:GetObject*",
                "s3:PutObject*"
            ],
            "Resource": [
				"arn:aws:s3:::the-family-loop` + `-customer-hash/posts/*",
				"arn:aws:s3:::the-family-loop` + `-customer-hash/pfp/*"
        ]
			}
    ]}`),
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
	//fmt.Println(filetype)
	defer ourfile.Close()

	//fileChan := make(chan s3.GetObjectAttributesOutput, 1)

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
		/*for range fileChan {
			go func() {
				nameOut, errname := client.GetObjectAttributes(context.TODO(), &s3.GetObjectAttributesInput{
					Bucket: aws.String("the-family-loop" + "-customer-hash"),
					Key:    aws.String("posts/images/" + fn),
					ObjectAttributes: []types.ObjectAttributes{
						"ObjectSize",
					},
				})
				if errname != nil {
					fmt.Println("Some sort of error :(")
					log.Fatal(errname)
				}
				fileChan <- *nameOut
			}()
		}*/
	} else if strings.Contains(filetype, "video") {

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

		/* go func() {
			nameOut, errname := client.GetObjectAttributes(context.TODO(), &s3.GetObjectAttributesInput{
				Bucket: aws.String("the-family-loop" + "-customer-hash"),
				Key:    aws.String("posts/videos/" + fn),
				ObjectAttributes: []types.ObjectAttributes{
					"ObjectSize",
				},
			})
			if errname != nil {
				fmt.Println("Some sort of error :(")
				log.Fatal(errname)
			}
			fileChan <- *nameOut

		}()
		for val := range fileChan {
			fmt.Println(val.ObjectSize)

		}*/

	} else {
		fmt.Println("Unknown file type. How did this get here?")
	}
	return filetype
}

/*func uploadPostPhotoTos3(f multipart.File, fn string, client *s3.Client) {

}*/
