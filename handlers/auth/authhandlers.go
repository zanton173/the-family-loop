package authhandler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	globalfunctions "tfl/functions"
	globalvars "tfl/vars"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"golang.org/x/crypto/bcrypt"
)

func UpdateFCMTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id = null where username = '%s';", r.URL.Query().Get("username")))
}

func SignUpHandler(w http.ResponseWriter, r *http.Request) {
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

func LoginHandler(w http.ResponseWriter, r *http.Request) {

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
func UpdateAdminPassHandler(w http.ResponseWriter, r *http.Request) {
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
func GetResetPasswordCodeHandler(w http.ResponseWriter, r *http.Request) {
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

	_, senderr := globalvars.SqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
		QueueUrl:    aws.String("https://sqs.us-east-1.amazonaws.com/529465713677/sendresetcode"),
		MessageBody: aws.String(fmt.Sprintf("{\"user\": \"%s\", \"resetcode\": \"%s\", \"email\": \"%s\", \"username\": \"%s\"}", emailInput, string(b), userEmail, userName)),
	})
	if senderr != nil {
		fmt.Println(senderr)
	}

	w.Write([]byte(fmt.Sprintf("{\"user\":\"%s\", \"code\": \"%s\", \"email\": \"%s\"}", userName, string(b), userEmail)))
}
func ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {

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

	out, geterr := globalvars.SqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
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

		out, _ := globalvars.SqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
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

		_, delErr := globalvars.SqsClient.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
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
