package globaltypes

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
