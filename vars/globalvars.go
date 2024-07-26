package vars

import (
	"context"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/api/option"
)

var Replacer *strings.Replacer = strings.NewReplacer("'", "\\'", "\"", "\\\"")

var NyLoc, NyLocErr = time.LoadLocation("America/New_York")
var FbOpts = []option.ClientOption{option.WithCredentialsFile("the-family-loop-fb0d9-firebase-adminsdk-k6sxl-14c7d4c4f7.json")}
var App, AppErr = firebase.NewApp(context.TODO(), nil, FbOpts...)
var Fb_message_client, FbInitErr = App.Messaging(context.TODO())

var awscfg, err = config.LoadDefaultConfig(context.TODO(),
	config.WithDefaultRegion("us-east-1"),
	config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(Awskey, Awskeysecret, "")),
)
var S3Client = s3.NewFromConfig(awscfg)

var Dbpass = os.Getenv("DB_PASS")
var Awskey = os.Getenv("AWS_ACCESS_KEY")
var Awskeysecret = os.Getenv("AWS_ACCESS_SECRET")
var Ghissuetoken = os.Getenv("GH_BEARER")
var Cfdistro = os.Getenv("CF_DOMAIN")
var S3Domain = os.Getenv("S3_BUCKET_NAME")
var OrgId = os.Getenv("ORG_ID")
var MongoDBPass = os.Getenv("MONGO_PASS")
var SubLevel = os.Getenv("SUB_PACKAGE")
var JwtSignKey = os.Getenv("JWT_SIGNING_KEY")
var Wixapikey = os.Getenv("WIX_API_KEY")
var Ghusercommentkey = os.Getenv("GH_USER_COMMENT_TOKEN")
