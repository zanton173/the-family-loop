package cshandler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	globalfunctions "tfl/functions"
	globalvars "tfl/vars"

	"github.com/google/go-github/github"
)

func CreateIssueHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

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
		issueLabel = []string{"enhancement", currentUserFromSession + "_" + strings.Split(globalvars.OrgId, "_")[0] + strings.Split(globalvars.OrgId, "_")[1][:3]}
	} else if postData.Label == "bug" {
		issueLabel = []string{"bug", currentUserFromSession + "_" + strings.Split(globalvars.OrgId, "_")[0] + strings.Split(globalvars.OrgId, "_")[1][:3]}
	}

	bodyText := fmt.Sprintf("%s on %s page - %s. Orgid: %s", strings.ReplaceAll(postData.Descdetail[1], "-", " "), postData.Descdetail[0], currentUserFromSession, strings.Split(globalvars.OrgId, "_")[0]+strings.Split(globalvars.OrgId, "_")[1][:3])
	issueJson := github.IssueRequest{
		Title: &postData.Issuetitle,
		Body:  &bodyText,
	}

	jsonMarshed, errMarsh := json.Marshal(issueJson)
	if errMarsh != nil {
		fmt.Println(errMarsh)
	}

	req, reqerr := http.NewRequest("POST", "https://api.github.com/repos/zanton173/the-family-loop/issues", bytes.NewReader(jsonMarshed))
	if reqerr != nil {
		fmt.Println(reqerr)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+globalvars.Ghusercommentkey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	type respBody struct {
		Number int `json:"number"`
	}

	var respData respBody
	bsread, _ := io.ReadAll(resp.Body)

	marshtoreaderr := json.Unmarshal(bsread, &respData)
	if marshtoreaderr != nil {
		fmt.Println(marshtoreaderr)
		return
	}

	issueWithLabelJson := github.IssueRequest{
		Labels: &issueLabel,
	}

	jsonMarshedWithLabel, errLabelMarsh := json.Marshal(issueWithLabelJson)
	if errLabelMarsh != nil {
		fmt.Println("Marshing here")
		fmt.Println(errLabelMarsh)
		return
	}

	updatereq, updatereqerr := http.NewRequest("PATCH", "https://api.github.com/repos/zanton173/the-family-loop/issues/"+fmt.Sprint(respData.Number), bytes.NewReader(jsonMarshedWithLabel))
	if updatereqerr != nil {
		fmt.Println(updatereqerr)
		return
	}
	updatereq.Header.Set("Accept", "application/vnd.github+json")
	updatereq.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	updatereq.Header.Set("Authorization", "token "+globalvars.Ghissuetoken)

	_, uperr := client.Do(updatereq)
	if uperr != nil {
		fmt.Println(uperr)
		return
	}
}
func CreateGHIssueCommentHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type formValues struct {
		Comment string `json:"bugissuecomment"`
		ComURL  string `json:"comurl"`
	}
	type dataBody struct {
		Body string `json:"body"`
	}
	bs, readerr := io.ReadAll(r.Body)
	if readerr != nil {
		fmt.Println(readerr)
	}
	var formValuesData formValues
	marsherr := json.Unmarshal(bs, &formValuesData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	var sendBody dataBody
	sendBody.Body = formValuesData.Comment
	sendToPost, sendToPostErr := json.Marshal(sendBody)
	if sendToPostErr != nil {
		fmt.Println(sendToPostErr)
		return
	}

	req, reqerr := http.NewRequest("POST", formValuesData.ComURL+"?per_page=100", bytes.NewReader(sendToPost))
	if reqerr != nil {
		fmt.Println(reqerr)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+globalvars.Ghusercommentkey)

	client := &http.Client{}
	_, err := client.Do(req)
	if err != nil {

		fmt.Println(err)
		return
	}

	defer client.CloseIdleConnections()
}
func GetCustomerSupportIssuesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	type issueResp struct {
		CommentsLink string `json:"comments_url"`
		Issuetitle   string `json:"title"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
		ClosedAt     string `json:"closed_at,omitempty"`
		Body         string `json:"body"`
	}
	type issueList []issueResp

	req, reqerr := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/zanton173/the-family-loop/issues?labels=%s&per_page=100&sort=created&direction=desc&state=%s", currentUserFromSession+"_"+strings.Split(globalvars.OrgId, "_")[0]+strings.Split(globalvars.OrgId, "_")[1][:3], r.URL.Query().Get("state")), nil)
	if reqerr != nil {
		fmt.Println(reqerr)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+globalvars.Ghusercommentkey)

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

	var listOfResp issueList
	var listOfAll issueList

	marshbodyerr := json.Unmarshal(bs, &listOfResp)

	if marshbodyerr != nil {
		fmt.Println("marsh at issue list:" + marshbodyerr.Error())
		return
	}

	listOfAll = append(listOfAll, listOfResp...)

	marsheddata, marsheddataerr := json.Marshal(listOfAll)
	if marsheddataerr != nil {
		fmt.Println("final marsh err")
		return
	}

	w.Write(marsheddata)

}
func GetGHIssuesComments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	type commentResp struct {
		Created string `json:"created_at"`
		Body    string `json:"body"`
		Author  struct {
			Username string `json:"login"`
		} `json:"user"`
	}
	type commentList []commentResp
	req, reqerr := http.NewRequest("GET", r.URL.Query().Get("comurl")+"?per_page=100", nil)
	if reqerr != nil {
		fmt.Println(reqerr)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+globalvars.Ghusercommentkey)

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
	var listofcomments commentList
	var commentListFull commentList
	marshbodyerr := json.Unmarshal(bs, &listofcomments)

	if marshbodyerr != nil {
		fmt.Println(marshbodyerr)
		return
	}

	commentListFull = append(listofcomments, commentListFull...)
	marsheddata, marsheddataerr := json.Marshal(commentListFull)
	if marsheddataerr != nil {
		fmt.Println("final marsh err")
		return
	}
	w.Write(marsheddata)

}
