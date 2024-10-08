package postshandler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	globalfunctions "tfl/functions"
	globaltypes "tfl/types"
	globalvars "tfl/vars"

	"firebase.google.com/go/messaging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

func CreateCommentHandler(w http.ResponseWriter, r *http.Request) {
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
		Comment        string   `json:"comment"`
		SelectedPostId int      `json:"selectedPostId"`
		Taggedusers    []string `json:"taggedUser"`
	}

	var postData postBody
	errmarsh := json.Unmarshal(bs, &postData)
	if errmarsh != nil {
		fmt.Println(errmarsh)
	}

	_, inserterr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.comments(\"comment\", \"post_id\", \"author\") values(E'%s', '%d', (select username from tfldata.users where username='%s'));", globalvars.Replacer.Replace(postData.Comment), postData.SelectedPostId, currentUserFromSession))
	if inserterr != nil {
		fmt.Println(inserterr)
	}
	var author string
	var pfpname string
	row := globalvars.Db.QueryRow(fmt.Sprintf("select username, pfp_name from tfldata.users where username='%s';", currentUserFromSession))
	userscnerr := row.Scan(&author, &pfpname)

	if userscnerr != nil || len(pfpname) == 0 {
		pfpname = "assets/32x32/ZCAN2301 The Family Loop Favicon_B_32 x 32.jpg"
	} else {
		pfpname = "https://" + globalvars.Cfdistro + "/pfp/" + pfpname
	}
	dataStr := "<div class='row'><p style='display: flex; align-items: center; padding-right: 0%;' class='m-1 col-7'>" + postData.Comment + "</p><div style='align-items: center; position: relative; display: flex; padding-left: 0%; left: 1%;' class='col my-5'><b style='position: absolute; bottom: 5%'>" + author + "</b><img width='30px' class='my-1' style='margin-left: 1%; position: absolute; right: 20%; border-style: solid; border-radius: 13px / 13px; box-shadow: 3px 3px 5px; border-width: thin; top: 5%;' src='" + pfpname + "' alt='tfl pfp' /></div></div>"

	commentTmpl, err := template.New("com").Parse(dataStr)
	if err != nil {
		fmt.Println(err)
	}
	commentTmpl.Execute(w, nil)
	go func() {
		var fcmToken string
		fcmrow := globalvars.Db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username = (select author from tfldata.posts where id=%d) and username != (select username from tfldata.users where username='%s') and fcm_registration_id is not null;", postData.SelectedPostId, currentUserFromSession))
		scnerr := fcmrow.Scan(&fcmToken)
		if scnerr == nil {

			//globalvars.Fb_message_client, _ := app.Messaging(context.TODO())
			typePayload := make(map[string]string)
			typePayload["type"] = "posts"
			sentRes, sendErr := globalvars.Fb_message_client.Send(context.TODO(), &messaging.Message{
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
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sentRes, time.Now().In(globalvars.NyLoc).Local().Format(time.DateTime)))
		}
		if len(postData.Taggedusers) > 0 {
			var usersPost string
			row := globalvars.Db.QueryRow(fmt.Sprintf("select author from tfldata.posts where id=%d", postData.SelectedPostId))
			row.Scan(&usersPost)

			for _, userTagged := range postData.Taggedusers {
				var fcmToken string
				fcmrow := globalvars.Db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username = '%s' and username != (select username from tfldata.users where username='%s') and fcm_registration_id is not null;", userTagged, currentUserFromSession))
				scnerr := fcmrow.Scan(&fcmToken)
				if scnerr == nil {

					//globalvars.Fb_message_client, _ := app.Messaging(context.TODO())
					typePayload := make(map[string]string)
					typePayload["type"] = "posts"
					sentRes, sendErr := globalvars.Fb_message_client.Send(context.TODO(), &messaging.Message{
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
					globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sentRes, time.Now().In(globalvars.NyLoc).Local().Format(time.DateTime)))
				}

			}
		}
	}()
}
func GetPostsHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)
	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
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
	var outerr error
	if r.URL.Query().Get("page") == "null" {
		output, outerr = globalvars.Db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key, createdon at time zone (select mytz from tfldata.users where username='%s') from tfldata.posts where (title ilike '%s' or description ilike '%s' or author ilike '%s') and available = true order by createdon DESC limit 2;", currentUserFromSession, "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%"))
	} else if r.URL.Query().Get("limit") == "current" {
		w.Header().Set("HX-Reswap", "innerHTML")
		output, outerr = globalvars.Db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key, createdon at time zone (select mytz from tfldata.users where username='%s') from tfldata.posts where id >= %s and (title ilike '%s' or description ilike '%s' or author ilike '%s') and available = true order by createdon DESC;", currentUserFromSession, r.URL.Query().Get("page"), "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%"))
	} else {
		output, outerr = globalvars.Db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key, createdon at time zone (select mytz from tfldata.users where username='%s') from tfldata.posts where id < %s and (title ilike '%s' or description ilike '%s' or author ilike '%s') and available = true order by createdon DESC limit 2;", currentUserFromSession, r.URL.Query().Get("page"), "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%"))
	}

	var dataStr string
	if outerr != nil {
		// log.Fatal(err)
		fmt.Print(outerr)
	}

	defer output.Close()
	for output.Next() {

		var postrows globaltypes.Postsrow
		var reaction string
		// if err := output.Scan(&postrows.Id, &postrows.Title, &postrows.Description, &postrows.File_name, &postrows.File_type, &postrows.Author); err != nil {
		if err := output.Scan(&postrows.Id, &postrows.Title, &postrows.Description, &postrows.Author, &postrows.Postfileskey, &postrows.Createdon); err != nil {
			activityStr := "get posts handler scanning posts query"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))

		}

		reactionRow := globalvars.Db.QueryRow(fmt.Sprintf("select reaction from tfldata.reactions where post_id=%d and author='%s';", postrows.Id, currentUserFromSession))
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
			editElement = fmt.Sprintf("<i style='position: absolute; background-color: gray; border-radius: 13px / 13px; z-index: 13' class='bi bi-trash m-1 px-1 editbtnclass' hx-post='/delete-this-post' hx-swap='none' hx-on::after-request='window.location.reload()' hx-vals=\"js:{'deletionID': %d}\" hx-params='not page, limit, token' hx-ext='json-enc' hx-confirm='Delete this post forever? This cannot be undone'></i>", postrows.Id)
		}
		comment := globalvars.Db.QueryRow(fmt.Sprintf("select count(*) from tfldata.comments where post_id='%d';", postrows.Id))
		var commentCount string
		comment.Scan(&commentCount)
		var countOfImg int
		rowCount := globalvars.Db.QueryRow(fmt.Sprintf("select count(*) from tfldata.postfiles where post_files_key='%s';", postrows.Postfileskey))
		rowCount.Scan(&countOfImg)
		var firstImg struct {
			filename string
			filetype string
		}
		firstRow := globalvars.Db.QueryRow(fmt.Sprintf("select file_name, file_type from tfldata.postfiles where post_files_key='%s' order by id asc limit 1;", postrows.Postfileskey))
		firstRow.Scan(&firstImg.filename, &firstImg.filetype)

		if strings.Contains(firstImg.filetype, "image") {

			if countOfImg > 1 {
				dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(109 109 109 / .34); border-radius: 20px 20px 20px 20px; box-shadow: 5px 4px 9px 3px rgb(0 0 0 / 52&percnt;);'>%s<img class='img-fluid' fetchpriority='high' id='%s' src='https://%s/posts/images/%s' alt='%s' style='border-radius: 18px 18px;' alt='default' /><p class='createdontime' style='margin-bottom: -6%s; margin-left: 78%s; text-decoration: underline; color: #4e4c4c;'>%s</p><div class='postarrows' style='display: flex; justify-content: space-around;'><i style='padding-left: 2rem; padding-right: 2rem; padding-bottom: .4rem' onclick='nextLeftImage(`%s`)' class='bi bi-arrow-90deg-left'></i><i style='padding-left: 2rem; padding-right: 2rem; padding-bottom: .4rem' onclick='nextRightImage(`%s`)' class='bi bi-arrow-90deg-right'></i></div><div id='%d' class='card-body' style='text-align: left; padding-left: 1&percnt;;'><b>%s</b><br/><p style='margin-bottom: .2rem'>%s</p><p style='margin-bottom: .2rem' class='card-text'>%s</p><div style='display: flex; justify-content: end'>%s<button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' class='btn btn-primary' hx-swap='innerHTML' style='margin-bottom: -.1rem'>Comments (%s)</button>%s</div></div></div>", editElement, postrows.Postfileskey, globalvars.Cfdistro, firstImg.filename, firstImg.filename, "%", "%", strings.Split(postrows.Createdon, "T")[0], postrows.Postfileskey, postrows.Postfileskey, postrows.Id, postrows.Author, postrows.Title, postrows.Description, reactionEmojiBeforeComment, postrows.Id, postrows.Id, commentCount, reactionBtn)
			} else if countOfImg == 1 {
				dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(109 109 109 / .34); border-radius: 20px 20px 20px 20px; box-shadow: 5px 4px 9px 3px rgb(0 0 0 / 52&percnt;);'>%s<img class='img-fluid' fetchpriority='high' id='%s' src='https://%s/posts/images/%s' alt='%s' style='border-radius: 18px 18px;' alt='default' /><p class='createdontime' style='margin-bottom: -6%s; margin-left: 78%s; text-decoration: underline; color: #4e4c4c;'>%s</p><div id='%d' class='card-body' style='text-align: left; padding-left: 1&percnt;;'><b>%s</b><br/><p style='margin-bottom: .2rem'>%s</p><p style='margin-bottom: .2rem' class='card-text'>%s</p><div style='display: flex; justify-content: end'>%s<button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary' style='margin-bottom: -.1rem'>Comments (%s)</button>%s</div></div></div>", editElement, postrows.Postfileskey, globalvars.Cfdistro, firstImg.filename, firstImg.filename, "%", "%", strings.Split(postrows.Createdon, "T")[0], postrows.Id, postrows.Author, postrows.Title, postrows.Description, reactionEmojiBeforeComment, postrows.Id, postrows.Id, commentCount, reactionBtn)
			}

		} else {

			if countOfImg > 1 {
				dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(109 109 109 / .34); border-radius: 20px 20px 20px 20px; box-shadow: 5px 4px 9px 3px rgb(0 0 0 / 52&percnt;);'>%s<video style='border-radius: 18px 18px; z-index: 4;' muted playsinline controls preload='metadata' id='%s'><source src='https://%s/posts/videos/%s#t=0.3'></video><p class='createdontime' style='margin-bottom: -6%s; margin-left: 78%s;text-decoration: underline;color: #4e4c4c;'>%s</p><div class='postarrows' style='display: flex; justify-content: space-around;'><i style='padding-left: 2rem; padding-right: 2rem; padding-bottom: .4rem' onclick='nextLeftImage(`%s`)' class='bi bi-arrow-90deg-left'></i><i style='padding-left: 2rem; padding-right: 2rem; padding-bottom: .4rem' onclick='nextRightImage(`%s`)' class='bi bi-arrow-90deg-right'></i></div><div id='%d' class='card-body' style='text-align: left; padding-left: 1&percnt;;'><b>%s</b><br/><p style='margin-bottom: .2rem'>%s</p><p style='margin-bottom: .2rem' class='card-text'>%s</p><div style='display: flex; justify-content: end'>%s<button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary' style='margin-bottom: -.1rem'>Comments (%s)</button>%s</div></div></div>", editElement, postrows.Postfileskey, globalvars.Cfdistro, firstImg.filename, "%", "%", strings.Split(postrows.Createdon, "T")[0], postrows.Postfileskey, postrows.Postfileskey, postrows.Id, postrows.Author, postrows.Title, postrows.Description, reactionEmojiBeforeComment, postrows.Id, postrows.Id, commentCount, reactionBtn)
			} else if countOfImg == 1 {
				dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(109 109 109 / .34); border-radius: 20px 20px 20px 20px; box-shadow: 5px 4px 9px 3px rgb(0 0 0 / 52&percnt;);'>%s<video style='border-radius: 18px 18px; z-index: 4;' muted playsinline controls preload='metadata' id='%s'><source src='https://%s/posts/videos/%s#t=0.3'></video><p class='createdontime' style='margin-bottom: -6%s; margin-left: 78%s;text-decoration: underline;color: #4e4c4c;'>%s</p><div id='%d' class='card-body' style='text-align: left; padding-left: 1&percnt;;'><b>%s</b><br/><p style='margin-bottom: .2rem'>%s</p><p style='margin-bottom: .2rem' class='card-text'>%s</p><div style='display: flex; justify-content: end'>%s<button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary' style='margin-bottom: -.1rem'>Comments (%s)</button>%s</div></div></div>", editElement, postrows.Postfileskey, globalvars.Cfdistro, firstImg.filename, "%", "%", strings.Split(postrows.Createdon, "T")[0], postrows.Id, postrows.Author, postrows.Title, postrows.Description, reactionEmojiBeforeComment, postrows.Id, postrows.Id, commentCount, reactionBtn)
			}
		}
		postTmpl, tmerr := template.New("tem").Parse(dataStr)
		if tmerr != nil {
			activityStr := "posts handler postTmpl err"
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", tmerr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		}
		postTmpl.Execute(w, nil)

	}

}
func GetSelectedPostsComments(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
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

	output, err := globalvars.Db.Query(fmt.Sprintf("select c.comment, substr(c.author, 0, 14), u.pfp_name from tfldata.comments as c join tfldata.users as u on c.author = u.username where c.post_id='%s'::integer order by c.id asc;", r.URL.Query().Get("post-id")))

	var dataStr string
	if err != nil {
		activityStr := fmt.Sprintf("getSelectedPostsCommentsHandler select query - %s", currentUserFromSession)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}

	defer output.Close()

	for output.Next() {
		var posts postComment

		if err := output.Scan(&posts.Comment, &posts.Author, &posts.Pfpname); err != nil || len(posts.Pfpname) == 0 {

			posts.Pfpname = "assets/32x32/ZCAN2301 The Family Loop Favicon_B_32 x 32.jpg"
			activityStr := fmt.Sprintf("getSelectedPostsCommentsHandler scan err - %s", currentUserFromSession)
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))

		} else {
			posts.Pfpname = "https://" + globalvars.Cfdistro + "/pfp/" + posts.Pfpname
		}

		dataStr = "<div class='row'><p style='display: flex; align-items: center; padding-right: 0%;' class='m-1 col-7'>" + posts.Comment + "</p><div style='align-items: center; position: relative; display: flex; padding-left: 0%; left: 1%;' class='col my-5'><b style='position: absolute; bottom: 5%'>" + posts.Author + "</b><img onclick='openImgBiggerView(event)' width='30px' class='my-1' style='margin-left: 1%; position: absolute; right: 20%; border-style: solid; border-radius: 13px / 13px; box-shadow: 3px 3px 5px; border-width: thin; top: 5%;' src='" + posts.Pfpname + "' alt='tfl pfp' /></div></div>"

		w.Write([]byte(dataStr))
	}

}
func GetPostImagesHandler(w http.ResponseWriter, r *http.Request) {

	var imgList []string
	rows, err := globalvars.Db.Query(fmt.Sprintf("select file_name from tfldata.postfiles where post_files_key='%s' order by id asc;", r.URL.Query().Get("id")))
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
func GetPostsReactionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	output, rowerr := globalvars.Db.Query(fmt.Sprintf("select author, reaction from tfldata.reactions where post_id='%s' and author != '%s';", r.URL.Query().Get("selectedPostId"), r.URL.Query().Get("username")))
	if rowerr != nil {
		globalvars.Db.Exec("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", rowerr, time.Now().In(globalvars.NyLoc).Local().Format(time.DateTime))
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
func GetMyLoadingPosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type loadingPostsType struct {
		Id        string `json:"id"`
		Title     string `json:"title"`
		Desc      string `json:"description"`
		CreatedOn string `json:"created"`
	}
	rows, rowerr := globalvars.Db.Query(fmt.Sprintf("select id::text,title,description,createdon at time zone (select mytz from tfldata.users where username='%s') from tfldata.posts where available = false and author = '%s';", currentUserFromSession, currentUserFromSession))
	if rowerr != nil {
		fmt.Println(rowerr)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var postDataJson []loadingPostsType
	for rows.Next() {
		var postData loadingPostsType
		var tempdescnull sql.NullString
		rows.Scan(&postData.Id, &postData.Title, &tempdescnull, &postData.CreatedOn)
		if !tempdescnull.Valid {
			postData.Desc = ""
		} else {
			postData.Desc = tempdescnull.String
		}
		postDataJson = append(postDataJson, postData)
	}
	jsonMarshed, marsherr := json.Marshal(postDataJson)
	if marsherr != nil {
		fmt.Print(marsherr)
		return
	}
	w.Write(jsonMarshed)

}
func CreatePostHandler(w http.ResponseWriter, r *http.Request) {

	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)
	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	postFilesKey := uuid.NewString()
	var postid sql.NullInt64
	insrow := globalvars.Db.QueryRow(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"author\", \"post_files_key\", \"createdon\", available) values(E'%s', E'%s', '%s', '%s', now(), false) RETURNING id;", globalvars.Replacer.Replace(r.PostFormValue("title")), globalvars.Replacer.Replace(r.PostFormValue("description")), currentUserFromSession, postFilesKey))

	insrow.Scan(&postid)
	if !postid.Valid {
		fmt.Println("post id not valid")
	}

	if insrow.Err() != nil {
		fmt.Println(insrow.Err())
		activityStr := "insert into posts table createPostHandler"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", insrow.Err(), time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	parseerr := r.ParseMultipartForm(64 << 20)
	if parseerr != nil {
		// handle error
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('memory error multi file upload %s');", parseerr.Error()))
	}
	lengthofpostsmedia := len(r.MultipartForm.File["file_name"])
	sem := make(chan struct{}, lengthofpostsmedia)

	for i := 0; i < lengthofpostsmedia; i++ {

		fh := r.MultipartForm.File["file_name"][i]

		sem <- struct{}{}

		f, err := fh.Open()
		if err != nil {

			fmt.Printf("Error opening file: %s, size: %d, error: %v\n", fh.Filename, fh.Size, err)
			activityStr := fmt.Sprintf("Open multipart file in createPostHandler - %s", currentUserFromSession)
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tmpFileName := fh.Filename

		var countIfExists int16
		countIfExistsOut := globalvars.Db.QueryRow(fmt.Sprintf("select count(*) from tfldata.postfiles where file_name = '%s';", fh.Filename))

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

		_, filereadingerr := f.Read(fileContents)
		if filereadingerr != nil {
			fmt.Print("err reading file: ")
			fmt.Println(filereadingerr)
			return
		}

		filetype := http.DetectContentType(fileContents)

		f.Seek(0, 0)
		_, errinsert := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.postfiles(\"file_name\", \"file_type\", \"post_files_key\") values(concat('postid_%s/','%s'), '%s', '%s');", strconv.Itoa(int(postid.Int64)), fh.Filename, filetype, postFilesKey))

		if errinsert != nil {
			activityStr := fmt.Sprintf("insert into postfiles table createPostHander - %s", currentUserFromSession)
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errinsert, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		}
		go func(fh *multipart.FileHeader) {

			defer func() {
				<-sem
			}()
			globalfunctions.UploadFileToS3(f, fmt.Sprintf("postid_%s/%s", strconv.Itoa(int(postid.Int64)), fh.Filename), globalvars.Db, filetype, lengthofpostsmedia, postid.Int64)

			defer f.Close()
		}(fh)
	}

	var chatMessageNotificationOpts globaltypes.NotificationOpts

	chatMessageNotificationOpts.ExtraPayloadKey = "post"
	chatMessageNotificationOpts.ExtraPayloadVal = "posts"
	chatMessageNotificationOpts.NotificationPage = "posts"

	chatMessageNotificationOpts.NotificationTitle = fmt.Sprintf("%s just made a new post!", currentUserFromSession)
	chatMessageNotificationOpts.NotificationBody = strings.ReplaceAll(r.PostFormValue("title"), "\\", "")

	go globalfunctions.SendNotificationToAllUsers(globalvars.Db, currentUserFromSession, globalvars.Fb_message_client, &chatMessageNotificationOpts)

}
func CreatePostReactionHandler(w http.ResponseWriter, r *http.Request) {
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
		Username       string `json:"username"`
		ReactionToPost string `json:"emoji"`
		Postid         int    `json:"selectedPostId"`
	}
	var postData postBody
	bs, _ := io.ReadAll(r.Body)
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s');", marsherr, time.Now().In(globalvars.NyLoc).Format(time.DateTime)))
		return
	}
	_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.reactions(\"post_id\", \"author\", \"reaction\") values(%d, '%s', '%s') on conflict(post_id,author) do update set reaction='%s';", postData.Postid, postData.Username, postData.ReactionToPost, postData.ReactionToPost))
	if inserr != nil {
		activityStr := fmt.Sprintf("insert into reactions createPostReactionHandler - %s", currentUserFromSession)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

}

func DeleteThisPostHandler(w http.ResponseWriter, r *http.Request) {
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

	output, outerr := globalvars.Db.Query(fmt.Sprintf("select pf.id,pf.file_name,pf.file_type from tfldata.posts as p join tfldata.postfiles as pf on pf.post_files_key = p.post_files_key where p.id=%d;", postData.PostID))
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
			_, err := globalvars.S3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
				Bucket: aws.String(globalvars.S3Domain),
				Key:    aws.String("posts/images/" + workData.Filename),
			})

			if err != nil {
				fmt.Println("error on image delete")
				fmt.Println(err.Error())
			}
		} else {
			_, err := globalvars.S3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
				Bucket: aws.String(globalvars.S3Domain),
				Key:    aws.String("posts/videos/" + workData.Filename),
			})

			if err != nil {
				fmt.Println("error on video delete")
				fmt.Println(err.Error())
			}
		}

		globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.postfiles where id=%d", workData.Pfilesid))
	}

	_, delerr := globalvars.Db.Exec(fmt.Sprintf("delete from tfldata.posts where id=%d", postData.PostID))
	if delerr != nil {
		fmt.Println(delerr)
	}
}
