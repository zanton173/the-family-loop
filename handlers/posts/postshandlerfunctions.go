package postshandler

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	globalfunctions "tfl/functions"
	globaltypes "tfl/types"
	globalvars "tfl/vars"

	"github.com/google/uuid"
)

func CreatePostHandler(w http.ResponseWriter, r *http.Request) {
	jwtSignKey := os.Getenv("JWT_SIGNING_KEY")
	dbvalidate := globalfunctions.DbConn()
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(dbvalidate, r)
	validBool := globalfunctions.ValidateJWTToken(jwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	db := globalfunctions.DbConn()

	postFilesKey := uuid.NewString()

	parseerr := r.ParseMultipartForm(10 << 20)
	if parseerr != nil {
		// handle error
		db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\") values('memory error multi file upload %s');", parseerr.Error()))
	}
	// upload, filename, errfile := r.FormFile("file_name")

	//for _, fh := range r.MultipartForm.File["file_name"] {
	for i := 0; i < len(r.MultipartForm.File["file_name"]); i++ {

		fh := r.MultipartForm.File["file_name"][i]
		f, err := fh.Open()
		if err != nil {
			fmt.Println(err)
			activityStr := fmt.Sprintf("Open multipart file in createPostHandler - %s", currentUserFromSession)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", err, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
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

		globalfunctions.UploadFileToS3(f, tmpFileName, db, filetype)

		_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.postfiles(\"file_name\", \"file_type\", \"post_files_key\") values('%s', '%s', '%s');", fh.Filename, filetype, postFilesKey))

		if errinsert != nil {
			activityStr := fmt.Sprintf("insert into postfiles table createPostHander - %s", currentUserFromSession)
			db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errinsert, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		}

		defer f.Close()
	}
	_, errinsert := db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"author\", \"post_files_key\", \"createdon\") values(E'%s', E'%s', '%s', '%s', now());", globalvars.Replacer.Replace(r.PostFormValue("title")), globalvars.Replacer.Replace(r.PostFormValue("description")), currentUserFromSession, postFilesKey))

	if errinsert != nil {
		fmt.Println(errinsert)
		activityStr := "insert into posts table createPostHandler"
		db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", errinsert, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var chatMessageNotificationOpts globaltypes.NotificationOpts

	chatMessageNotificationOpts.ExtraPayloadKey = "post"
	chatMessageNotificationOpts.ExtraPayloadVal = "posts"
	chatMessageNotificationOpts.NotificationPage = "posts"

	chatMessageNotificationOpts.NotificationTitle = fmt.Sprintf("%s just made a new post!", currentUserFromSession)
	chatMessageNotificationOpts.NotificationBody = strings.ReplaceAll(r.PostFormValue("title"), "\\", "")

	go globalfunctions.SendNotificationToAllUsers(db, currentUserFromSession, globalvars.Fb_message_client, &chatMessageNotificationOpts)

}
