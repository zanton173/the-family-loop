package postshandler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"

	globalfunctions "tfl/functions"
	globaltypes "tfl/types"
	globalvars "tfl/vars"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

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
		output, outerr = globalvars.Db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key, createdon at time zone (select mytz from tfldata.users where username='%s') from tfldata.posts where title ilike '%s' or description ilike '%s' or author ilike '%s' order by createdon DESC limit 2;", currentUserFromSession, "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%"))
	} else if r.URL.Query().Get("limit") == "current" {
		w.Header().Set("HX-Reswap", "innerHTML")
		output, outerr = globalvars.Db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key, createdon at time zone (select mytz from tfldata.users where username='%s') from tfldata.posts where id >= %s and (title ilike '%s' or description ilike '%s' or author ilike '%s') order by createdon DESC;", currentUserFromSession, r.URL.Query().Get("page"), "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%"))
	} else {
		output, outerr = globalvars.Db.Query(fmt.Sprintf("select id, \"title\", description, author, post_files_key, createdon at time zone (select mytz from tfldata.users where username='%s') from tfldata.posts where id < %s and (title ilike '%s' or description ilike '%s' or author ilike '%s') order by createdon DESC limit 2;", currentUserFromSession, r.URL.Query().Get("page"), "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%", "%"+r.URL.Query().Get("search")+"%"))
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
				dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(109 109 109 / .34); border-radius: 20px 20px 20px 20px; box-shadow: 5px 4px 9px 3px rgb(0 0 0 / 52&percnt;);'>%s<video style='border-radius: 18px 18px; z-index: 4;' muted playsinline controls preload='auto' id='%s'><source src='https://%s/posts/videos/%s'></video><p class='createdontime' style='margin-bottom: -6%s; margin-left: 78%s;text-decoration: underline;color: #4e4c4c;'>%s</p><div class='postarrows' style='display: flex; justify-content: space-around;'><i style='padding-left: 2rem; padding-right: 2rem; padding-bottom: .4rem' onclick='nextLeftImage(`%s`)' class='bi bi-arrow-90deg-left'></i><i style='padding-left: 2rem; padding-right: 2rem; padding-bottom: .4rem' onclick='nextRightImage(`%s`)' class='bi bi-arrow-90deg-right'></i></div><div id='%d' class='card-body' style='text-align: left; padding-left: 1&percnt;;'><b>%s</b><br/><p style='margin-bottom: .2rem'>%s</p><p style='margin-bottom: .2rem' class='card-text'>%s</p><div style='display: flex; justify-content: end'>%s<button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary' style='margin-bottom: -.1rem'>Comments (%s)</button>%s</div></div></div>", editElement, postrows.Postfileskey, globalvars.Cfdistro, firstImg.filename, "%", "%", strings.Split(postrows.Createdon, "T")[0], postrows.Postfileskey, postrows.Postfileskey, postrows.Id, postrows.Author, postrows.Title, postrows.Description, reactionEmojiBeforeComment, postrows.Id, postrows.Id, commentCount, reactionBtn)
			} else if countOfImg == 1 {
				dataStr = fmt.Sprintf("<div class='card my-4' style='background-color: rgb(109 109 109 / .34); border-radius: 20px 20px 20px 20px; box-shadow: 5px 4px 9px 3px rgb(0 0 0 / 52&percnt;);'>%s<video style='border-radius: 18px 18px; z-index: 4;' muted playsinline controls preload='auto' id='%s'><source src='https://%s/posts/videos/%s'></video><p class='createdontime' style='margin-bottom: -6%s; margin-left: 78%s;text-decoration: underline;color: #4e4c4c;'>%s</p><div id='%d' class='card-body' style='text-align: left; padding-left: 1&percnt;;'><b>%s</b><br/><p style='margin-bottom: .2rem'>%s</p><p style='margin-bottom: .2rem' class='card-text'>%s</p><div style='display: flex; justify-content: end'>%s<button hx-get='/get-selected-post?post-id=%d' onclick='openPostFunction(%d)' hx-target='#modal-post-content' hx-swap='innerHTML' class='btn btn-primary' style='margin-bottom: -.1rem'>Comments (%s)</button>%s</div></div></div>", editElement, postrows.Postfileskey, globalvars.Cfdistro, firstImg.filename, "%", "%", strings.Split(postrows.Createdon, "T")[0], postrows.Id, postrows.Author, postrows.Title, postrows.Description, reactionEmojiBeforeComment, postrows.Id, postrows.Id, commentCount, reactionBtn)
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

	//var commentTmpl *template.Template

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

		dataStr = "<div class='row'><p style='display: flex; align-items: center; padding-right: 0%;' class='m-1 col-7'>" + posts.Comment + "</p><div style='align-items: center; position: relative; display: flex; padding-left: 0%; left: 1%;' class='col my-5'><b style='position: absolute; bottom: 5%'>" + posts.Author + "</b><img width='30px' class='my-1' style='margin-left: 1%; position: absolute; right: 20%; border-style: solid; border-radius: 13px / 13px; box-shadow: 3px 3px 5px; border-width: thin; top: 5%;' src='" + posts.Pfpname + "' alt='tfl pfp' /></div></div>"

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

	parseerr := r.ParseMultipartForm(10 << 20)
	if parseerr != nil {
		// handle error
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('memory error multi file upload %s');", parseerr.Error()))
	}
	// upload, filename, errfile := r.FormFile("file_name")

	//for _, fh := range r.MultipartForm.File["file_name"] {
	for i := 0; i < len(r.MultipartForm.File["file_name"]); i++ {

		fh := r.MultipartForm.File["file_name"][i]
		f, err := fh.Open()
		if err != nil {
			fmt.Println(err)
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

		f.Read(fileContents)

		filetype := http.DetectContentType(fileContents)

		f.Seek(0, 0)

		globalfunctions.UploadFileToS3(f, tmpFileName, globalvars.Db, filetype)

		_, errinsert := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.postfiles(\"file_name\", \"file_type\", \"post_files_key\") values('%s', '%s', '%s');", fh.Filename, filetype, postFilesKey))

		if errinsert != nil {
			activityStr := fmt.Sprintf("insert into postfiles table createPostHander - %s", currentUserFromSession)
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errinsert, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		}

		defer f.Close()
	}
	_, errinsert := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"author\", \"post_files_key\", \"createdon\") values(E'%s', E'%s', '%s', '%s', now());", globalvars.Replacer.Replace(r.PostFormValue("title")), globalvars.Replacer.Replace(r.PostFormValue("description")), currentUserFromSession, postFilesKey))

	if errinsert != nil {
		fmt.Println(errinsert)
		activityStr := "insert into posts table createPostHandler"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errinsert, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		w.WriteHeader(http.StatusBadRequest)
		return
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
