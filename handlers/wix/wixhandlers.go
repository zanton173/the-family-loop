package wixhandler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	globalfunctions "tfl/functions"
	globalvars "tfl/vars"
)

func WixWebhookChangePlanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	validBool := globalfunctions.ValidateWebhookJWTToken(globalvars.JwtSignKey, r)
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

	rewrite := bytes.ReplaceAll(envvar, []byte("SUB_PACKAGE="+globalvars.SubLevel), []byte("SUB_PACKAGE="+strings.ToLower(postData.Plan)))
	writeerr := os.WriteFile(".env", rewrite, 0644)
	if writeerr != nil {
		fmt.Println(writeerr)
		return
	}
	globalvars.SubLevel = strings.ToLower(postData.Plan)
}
func GetCurrentUserSubPlan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var wixMemberId sql.NullString
	var userEmail string
	sqlrow := globalvars.Db.QueryRow(fmt.Sprintf("select wix_member_id, email from tfldata.users where username='%s';", currentUserFromSession))
	sqlrow.Scan(&wixMemberId, &userEmail)
	if !wixMemberId.Valid {
		w.WriteHeader(http.StatusFailedDependency)
		w.Write([]byte(userEmail))
		return
	}
	type operatorDataType string
	type titleObj struct {
		Oper operatorDataType `json:"$eq"`
	}
	type orgidObj struct {
		Oper operatorDataType `json:"$eq"`
	}
	type usernameObj struct {
		Oper operatorDataType `json:"$eq"`
	}

	type filterTitleObj struct {
		Title    titleObj    `json:"title"`
		OrgId    orgidObj    `json:"orgid"`
		Username usernameObj `json:"username"`
	}

	type postReqQueryObj struct {
		Filter filterTitleObj `json:"filter"`
		Fields []string       `json:"fields"`
	}
	type postReqObj struct {
		DataCollectionId string          `json:"dataCollectionId"`
		Query            postReqQueryObj `json:"query"`
	}
	postReqBody := postReqObj{
		DataCollectionId: "regular-user-subscriptions",
		Query: postReqQueryObj{
			Filter: filterTitleObj{
				Title: titleObj{
					Oper: operatorDataType(wixMemberId.String),
				},
				OrgId: orgidObj{
					Oper: operatorDataType(globalvars.OrgId),
				},
				Username: usernameObj{
					Oper: operatorDataType(currentUserFromSession),
				},
			},
			Fields: []string{"orderid"},
		},
	}
	type collBody struct {
		Orderid string `json:"orderid,omitempty"`
	}
	type dataItems []struct {
		Id           string   `json:"id,omitempty"`
		CollBodyData collBody `json:"data"`
	}

	type clientListStruct struct {
		Dataitems dataItems `json:"dataItems"`
	}
	var respObj clientListStruct

	jsonMarshed, errMarsh := json.Marshal(&postReqBody)
	if errMarsh != nil {
		fmt.Println(errMarsh)
	}

	req, reqerr := http.NewRequest("POST", "https://www.wixapis.com/wix-data/v2/items/query", bytes.NewReader(jsonMarshed))
	if reqerr != nil {
		fmt.Println(reqerr)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", globalvars.Wixapikey)
	req.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
	req.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	bs, readerr := io.ReadAll(resp.Body)
	if readerr != nil {
		fmt.Println(readerr)
		return
	}

	marsherr := json.Unmarshal(bs, &respObj)
	if marsherr != nil {
		fmt.Println(marsherr)
		return
	}
	if len(respObj.Dataitems) > 0 {
		orderID := respObj.Dataitems[0].CollBodyData.Orderid

		orderreq, orderreqerr := http.NewRequest("GET", "https://www.wixapis.com/pricing-plans/v2/orders/"+orderID, nil)
		orderreq.Header.Set("Content-Type", "application/json")
		orderreq.Header.Set("Authorization", globalvars.Wixapikey)
		orderreq.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
		orderreq.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
		if orderreqerr != nil {
			fmt.Println(orderreqerr)
			return
		}
		orderdataResp, orderDataRespErr := client.Do(orderreq)
		if orderDataRespErr != nil {
			fmt.Println(orderDataRespErr)
			return
		}
		type innerCycleObj struct {
			Index       int    `json:"index"`
			StartedDate string `json:"startedDate"`
			EndedDate   string `json:"endedDate"`
		}

		type cycleDurObj struct {
			Count int    `json:"count"`
			Unit  string `json:"unit"`
		}
		type cancellationObj struct {
			Cause       string `json:"cause"`
			EffectiveAt string `json:"effectiveAt"`
		}
		type subscObj struct {
			CycleDuration cycleDurObj `json:"cycleDuration"`
			CycleCount    int         `json:"cycleCount"`
		}
		type pricingObj struct {
			Subscription subscObj `json:"subscription"`
		}
		type buyerObj struct {
			MemberId  string `json:"memberId"`
			ContactId string `json:"contactId"`
		}
		type orderObj struct {
			Id                   string          `json:"id"`
			PlanId               string          `json:"planId"`
			SubscriptionId       string          `json:"subscriptionId"`
			Buyer                buyerObj        `json:"buyer"`
			Status               string          `json:"status"`
			StatusNew            string          `json:"statusNew"`
			StartDate            string          `json:"startDate"`
			PlanName             string          `json:"planName"`
			PlanDesc             string          `json:"planDescription"`
			PlanPrice            string          `json:"planPrice"`
			WixPayOrderId        string          `json:"wixPayOrderId"`
			PricingData          pricingObj      `json:"pricing"`
			CurrentCycle         innerCycleObj   `json:"currentCycle"`
			Cancellation         cancellationObj `json:"cancellation"`
			AutoRenewedCancelled bool            `json:"autoRenewCanceled"`
		}

		type outerRespObj struct {
			Order orderObj `json:"order"`
		}
		var currentOrderData outerRespObj
		orderRespBody, readerr := io.ReadAll(orderdataResp.Body)
		if readerr != nil {
			fmt.Println(readerr.Error())
			return
		}

		unmarsherr := json.Unmarshal(orderRespBody, &currentOrderData)
		if unmarsherr != nil {
			fmt.Println(unmarsherr.Error())
			return
		}

		marshedObj, marsherr := json.Marshal(currentOrderData)
		if marsherr != nil {
			fmt.Println(marsherr)
			return
		}

		w.Write(marshedObj)
		defer req.Body.Close()
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}
func CancelCurrentSubRegUserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type postBody struct {
		Orderid     string `json:"orderid"`
		CancelNow   bool   `json:"radionow"`
		CancelLater bool   `json:"radiolater"`
	}
	var postData postBody
	bs, _ := io.ReadAll(r.Body)

	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)

	}
	var effectiveAt string
	if postData.CancelLater && !postData.CancelNow {
		effectiveAt = "NEXT_PAYMENT_DATE"
	} else if !postData.CancelLater && postData.CancelNow {
		effectiveAt = "IMMEDIATELY"
	}
	type sendPostBody struct {
		EffectiveAt string `json:"effectiveAt"`
	}
	var sendPostData sendPostBody
	sendPostData.EffectiveAt = effectiveAt
	jsonMarshed, marshlingerr := json.Marshal(sendPostData)
	if marshlingerr != nil {
		fmt.Println(marshlingerr)
		return
	}

	req, reqerr := http.NewRequest("POST", "https://www.wixapis.com/pricing-plans/v2/orders/"+postData.Orderid+"/cancel", bytes.NewReader(jsonMarshed))
	if reqerr != nil {
		activityStr := "error posting to wix cancel order handler"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", reqerr.Error(), activityStr))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", globalvars.Wixapikey)
	req.Header.Set("wix-account-id", "1c983d62-821d-4336-b87a-a66679cdf547")
	req.Header.Set("wix-site-id", "505f68a9-540d-40a7-abba-8ae650fa3252")
	client := &http.Client{}
	_, clientdoerr := client.Do(req)
	if clientdoerr != nil {
		activityStr := "client error for wix members reset pass only"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", clientdoerr.Error(), activityStr))
		return
	}

	defer client.CloseIdleConnections()

}
func SendResetPassOnlyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	bs, _ := io.ReadAll(r.Body)
	type formPostBody struct {
		Email string `json:"currentemail"`
	}
	var formData formPostBody
	json.Unmarshal(bs, &formData)
	type memberChildrenObj struct {
		LoginEmail string `json:"loginEmail"`
	}
	type memberObj struct {
		MemChild memberChildrenObj `json:"member"`
	}

	postReqBody := memberObj{
		MemChild: memberChildrenObj{
			LoginEmail: strings.ToLower(formData.Email),
		},
	}
	jsonMarshed, errMarsh := json.Marshal(&postReqBody)
	if errMarsh != nil {
		activityStr := "error marshing Json body for members reset pass only"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", errMarsh.Error(), activityStr))
		return
	}

	req, reqerr := http.NewRequest("POST", "https://www.wixapis.com/members/v1/members", bytes.NewReader(jsonMarshed))
	if reqerr != nil {
		activityStr := "error posting to wix members sign up / reset pass only handler"
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
		activityStr := "client error for wix members reset pass only"
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
		Email: strings.ToLower(formData.Email),
		Lang:  "en",
		RedirectObj: redirectObj{
			Url: "https://the-family-loop.com",
		},
	}
	sendjsonMarshed, senderrMarsh := json.Marshal(&postBody)
	if senderrMarsh != nil {
		activityStr := "error marshing Json body for members sign up reset pass only"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105), substr('%s',0,105), now());", senderrMarsh.Error(), activityStr))
		return
	}
	setpassreq, setpassreqerr := http.NewRequest("POST", "https://www.wixapis.com/_api/iam/recovery/v1/send-email", bytes.NewReader(sendjsonMarshed))
	if setpassreqerr != nil {
		activityStr := "error sending set pass wix members sign up reset pass only"
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
	if createresp.StatusCode == 200 {
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
		_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set wix_member_id = '%s' where username = '%s';", responseData.Memberstruct.Id, currentUserFromSession))
		if uperr != nil {
			globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,106), substr('%s',0,105), now());", uperr.Error(), "Err updating users table with wix id"))
		}
	}
	defer createresp.Body.Close()
}
func RegUserPaidForPlanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	validBool := globalfunctions.ValidateWebhookJWTToken(globalvars.JwtSignKey, r)
	if !validBool {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var setStatus = false
	var currentstatus sql.NullBool
	res := globalvars.Db.QueryRow(fmt.Sprintf("select is_paying_subscriber from tfldata.users where username='%s';", r.URL.Query().Get("username")))
	res.Scan(&currentstatus)
	if !currentstatus.Valid {
		currentstatus.Bool = false
	}
	if currentstatus.Bool {
		setStatus = false
	} else {
		setStatus = true
	}
	_, uperr := globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set is_paying_subscriber = %t where username = '%s';", setStatus, r.URL.Query().Get("username")))
	if uperr != nil {
		activityStr := "updating user is now paying wix webhook"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values (substr('%s',0,105), substr('%s',0,105), now());", uperr.Error(), activityStr))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}
