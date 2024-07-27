package vars

import (
	"database/sql"
	"strings"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/api/option"
)

var Replacer *strings.Replacer

var (
	NyLoc    *time.Location
	NyLocErr error
)
var FbOpts []option.ClientOption
var (
	App    *firebase.App
	AppErr error
)
var (
	Fb_message_client *messaging.Client
	FbInitErr         error
)

var (
	Awscfg    aws.Config
	Awscfgerr error
)

var (
	Db    *sql.DB
	DbErr error
)

var S3Client *s3.Client

var Dbpass string
var Awskey string
var Awskeysecret string
var Ghissuetoken string
var Cfdistro string
var S3Domain string
var OrgId string
var MongoDBPass string
var SubLevel string
var JwtSignKey string
var Wixapikey string
var Ghusercommentkey string
