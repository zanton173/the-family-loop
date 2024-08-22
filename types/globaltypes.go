package globaltypes

import (
	"database/sql"
)

type NotificationOpts struct {
	NotificationPage  string
	ExtraPayloadKey   string
	ExtraPayloadVal   string
	NotificationTitle string
	NotificationBody  string
	IsTagged          bool
}
type Postsrow struct {
	Id           int
	Title        string
	Description  string
	Author       string
	Postfileskey string
	Createdon    string
}

type Postjoin struct {
	Filename     string
	Filetype     string
	Postfileskey string
}
type SeshStruct struct {
	Username         string
	Pfpname          sql.NullString
	BGtheme          string
	GchatOrderOpt    bool
	CFDomain         string
	Isadmin          bool
	Fcmkey           sql.NullString
	LastViewedPChat  sql.NullString
	LastViewedThread sql.NullString
	IsLoopOwner      bool
}
type WebSocketPongMessage struct {
	Data   string `json:"data"`
	Player string `json:"username"`
	Type   string `json:"type"`
}
