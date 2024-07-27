package userdatahandler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	globalfunctions "tfl/functions"
	globaltypes "tfl/types"
	globalvars "tfl/vars"
	"time"
)

func GetSessionDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)

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
			resp.Orgid = globalvars.OrgId
			resp.Username = currentUserFromSession
			bs, _ := json.Marshal(&resp)
			w.Write(bs)
			return
		}
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var ourSeshStruct globaltypes.SeshStruct

	row := globalvars.Db.QueryRow(fmt.Sprintf("select username, gchat_bg_theme, gchat_order_option, is_admin, pfp_name, fcm_registration_id, last_viewed_pchat, last_viewed_gchat from tfldata.users where username='%s';", currentUserFromSession))
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

	ourSeshStruct.CFDomain = globalvars.Cfdistro

	data, err := json.Marshal(&ourSeshStruct)
	if err != nil {
		fmt.Println(err)
	}

	w.Write(data)
}
func GetSubscribedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)

	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var fcmRegToken string
	fcmRegRow := globalvars.Db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username='%s';", currentUserFromSession))
	scnerr := fcmRegRow.Scan(&fcmRegToken)

	if scnerr != nil {
		w.WriteHeader(http.StatusAccepted)
		return
		// globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", scnerr, time.Now().In(globalvars.NyLoc).Local().Format(time.DateTime)))
	}
	w.WriteHeader(http.StatusOK)
}
func SubscriptionHandler(w http.ResponseWriter, r *http.Request) {

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
	_, inserr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id='%s' where session_token='%s';", postData.Fcmtoken, seshVal))
	if inserr != nil {
		activityStr := "attempt update fcm token where seshtoken is value subHandler"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}

}
func UpdatePfpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "multipart/form-data")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	upload, filename, _ := r.FormFile("changepfp")

	username := r.PostFormValue("usernameinput")

	fn := globalfunctions.UploadPfpToS3(upload, filename.Filename, r, "changepfp")
	_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set pfp_name='%s' where username='%s';", fn, username))
	if uperr != nil {
		activityStr := fmt.Sprintf("update table users set pfp_name failed for user %s", currentUserFromSession)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}
func UpdateChatThemeHandler(w http.ResponseWriter, r *http.Request) {
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
		Theme    string `json:"theme"`
		Username string `json:"username"`
	}
	var postData postBody
	bs, _ := io.ReadAll(r.Body)
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set gchat_bg_theme='%s' where username='%s';", postData.Theme, postData.Username))
	if uperr != nil {
		activityStr := fmt.Sprintf("updateChatTheme failed for user %s", currentUserFromSession)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", uperr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}
}
