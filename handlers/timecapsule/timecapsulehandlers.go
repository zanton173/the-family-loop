package tchandler

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	globalfunctions "tfl/functions"
	"time"

	globalvars "tfl/vars"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func CreateNewTimeCapsuleHandler(w http.ResponseWriter, r *http.Request) {
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
func GetMyNotYetPurchasedTimeCapsulesHandler(w http.ResponseWriter, r *http.Request) {
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
func GetMyPurchasedTimeCapsulesHandler(w http.ResponseWriter, r *http.Request) {
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
func GetMyAvailableTimeCapsulesHandler(w http.ResponseWriter, r *http.Request) {
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
func AvailableTcWasDownloaded(w http.ResponseWriter, r *http.Request) {
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
func GetMyTcRequestStatusHandler(w http.ResponseWriter, r *http.Request) {
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
func InitiateMyTCRestoreHandler(w http.ResponseWriter, r *http.Request) {
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
func WixWebhookEarlyAccessPaymentCompleteHandler(w http.ResponseWriter, r *http.Request) {
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
func WixWebhookTCInitialPurchaseHandler(w http.ResponseWriter, r *http.Request) {
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
func DeleteMyTChandler(w http.ResponseWriter, r *http.Request) {
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
