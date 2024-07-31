package globalfunctions

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	imagego "image"
	globaltypes "tfl/types"
	globalvars "tfl/vars"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/disintegration/imaging"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/api/option"
)

/* INITIALIZE ITEMS */
func GetEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		os.Exit(1)
	}
}
func InitalizeAll() {
	GetEnv()
	globalvars.Dbpass = os.Getenv("DB_PASS")
	globalvars.Awskey = os.Getenv("AWS_ACCESS_KEY")
	globalvars.Awskeysecret = os.Getenv("AWS_ACCESS_SECRET")
	globalvars.Ghissuetoken = os.Getenv("GH_BEARER")
	globalvars.Cfdistro = os.Getenv("CF_DOMAIN")
	globalvars.S3Domain = os.Getenv("S3_BUCKET_NAME")
	globalvars.OrgId = os.Getenv("ORG_ID")
	globalvars.MongoDBPass = os.Getenv("MONGO_PASS")
	globalvars.SubLevel = os.Getenv("SUB_PACKAGE")
	globalvars.JwtSignKey = os.Getenv("JWT_SIGNING_KEY")
	globalvars.Wixapikey = os.Getenv("WIX_API_KEY")
	globalvars.Ghusercommentkey = os.Getenv("GH_USER_COMMENT_TOKEN")
	globalvars.Replacer = strings.NewReplacer("'", "\\'", "\"", "\\\"")

	globalvars.NyLoc, globalvars.NyLocErr = time.LoadLocation("America/New_York")
	globalvars.FbOpts = []option.ClientOption{option.WithCredentialsFile("the-family-loop-fb0d9-firebase-adminsdk-k6sxl-14c7d4c4f7.json")}

	globalvars.App, globalvars.AppErr = firebase.NewApp(context.TODO(), nil, globalvars.FbOpts...)

	if globalvars.AppErr != nil {
		fmt.Println("Err init firebase app")
		os.Exit(14)
	}

	globalvars.Fb_message_client, globalvars.FbInitErr = globalvars.App.Messaging(context.TODO())

	if globalvars.FbInitErr != nil {
		fmt.Println("Err init firebase messaging app")
		os.Exit(15)
	}

	globalvars.Awscfg, globalvars.Awscfgerr = config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(globalvars.Awskey, globalvars.Awskeysecret, "")),
	)

	if globalvars.Awscfgerr != nil {
		fmt.Println(globalvars.Awscfgerr)
		os.Exit(3)
	}

	globalvars.SqsClient = sqs.NewFromConfig(globalvars.Awscfg)
	globalvars.S3Client = s3.NewFromConfig(globalvars.Awscfg)
	globalvars.SesClient = ses.NewFromConfig(globalvars.Awscfg)

	connStr := fmt.Sprintf("postgresql://tfldbrole:%s@localhost/tfl?sslmode=disable", globalvars.Dbpass)
	globalvars.Db, globalvars.DbErr = sql.Open("postgres", connStr)

	if globalvars.DbErr != nil {
		log.Fatal(globalvars.DbErr)
	}

	globalvars.MongoDb, globalvars.Mongoerr = mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb+srv://tfl-user:"+globalvars.MongoDBPass+"@tfl-leaderboard.dg95d1f.mongodb.net/?retryWrites=true&w=majority"))

	if globalvars.Mongoerr != nil {
		activityStr := "mongo Initalize error"
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage, activity, createdon) values(substr('%s',0,105),substr('%s',0,105),now());", globalvars.Mongoerr.Error(), activityStr))
		return
	}

	globalvars.Leaderboardcoll = globalvars.MongoDb.Database("tfl-database").Collection("leaderboards")
}

func DbConn() *sql.DB {
	dbpass := globalvars.Dbpass

	connStr := fmt.Sprintf("postgresql://tfldbrole:%s@localhost/tfl?sslmode=disable", dbpass)
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		log.Fatal(err)
	}
	//defer db.Close()
	return db
}
func InitializeS3Client() *s3.Client {
	awskey := os.Getenv("AWS_ACCESS_KEY")
	awskeysecret := os.Getenv("AWS_ACCESS_SECRET")
	awscfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awskey, awskeysecret, "")),
	)

	if err != nil {
		log.Fatal(err)
		os.Exit(4)
	}

	return s3.NewFromConfig(awscfg)
}

/* SESSION ITEMS */
func ValidateCurrentSessionId(db *sql.DB, r *http.Request) (bool, string, string) {
	var handlerForLogin string
	session_token, seshErr := r.Cookie("session_id")
	if seshErr != nil {
		handlerForLogin = "onUnauthorizedEvent"
		return false, "Please login", handlerForLogin
	}

	var username sql.NullString
	var currentlypaying sql.NullBool
	var currentSessionToken sql.NullString
	row := db.QueryRow(fmt.Sprintf("select username, is_paying_subscriber, session_token from tfldata.users where session_token='%s';", strings.Split(session_token.Value, "session_id=")[0]))
	scnerr := row.Scan(&username, &currentlypaying, &currentSessionToken)
	if scnerr != nil {
		handlerForLogin = "onUnauthorizedEvent"
		return false, "Please login", handlerForLogin
	}

	if currentSessionToken.Valid && currentSessionToken.String != strings.Split(session_token.Value, "session_id=")[0] {
		handlerForLogin = "onUnauthorizedEvent"
		return false, "Please login", handlerForLogin
	}

	if !currentlypaying.Valid {
		currentlypaying.Bool = false
		handlerForLogin = "onRevealedYouHaveNotPurchasedRegularUserSubscriptionPlan"
	}
	if !currentlypaying.Bool {
		handlerForLogin = "onRevealedYouHaveNotPurchasedRegularUserSubscriptionPlan"
		return false, username.String, handlerForLogin
	}
	return scnerr == nil, username.String, handlerForLogin

}
func ValidateJWTToken(tokenKey string, r *http.Request) bool {
	jwtCookie, cookieErr := r.Cookie("backendauth")
	if cookieErr != nil {
		return false
	}

	jwtToken, jwtValidateErr := jwt.Parse(jwtCookie.Value, func(jwtToken *jwt.Token) (interface{}, error) {
		return []byte(tokenKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if jwtValidateErr != nil {
		return false
	}
	return jwtToken.Valid
}
func GenerateLoginJWT(username string, w http.ResponseWriter, jwtKey string) *jwt.Token {
	daysToExp := int64(7)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":  "backend-auth",
		"user": username,
		"exp":  time.Now().Unix() + (24 * 60 * 60 * daysToExp),
	})
	expiresAt := time.Now().Add(24 * time.Duration(daysToExp) * time.Hour)

	signKey, _ := token.SignedString([]byte(jwtKey))
	http.SetCookie(w, &http.Cookie{
		Name:     "backendauth",
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		Value:    signKey,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})
	return token
}
func ValidateWebhookJWTToken(tokenKey string, r *http.Request) bool {
	jwtHeaderVal := r.Header.Get("Authorization")
	jwtToken, jwtValidateErr := jwt.Parse(jwtHeaderVal, func(jwtToken *jwt.Token) (interface{}, error) {
		return []byte(tokenKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if jwtValidateErr != nil {
		return false
	}
	return jwtToken.Valid
}
func SetLoginCookie(w http.ResponseWriter, db *sql.DB, userStr string, acceptedTz string) {

	sessionToken := uuid.NewString()
	expiresAt := time.Now().Add(3600 * time.Hour)
	//fmt.Println(expiresAt.Local().Format(time.DateTime))
	//fmt.Println(userStr)
	/*_, inserterr := db.Exec(fmt.Sprintf("insert into tfldata.sessions(\"username\", \"session_token\", \"expiry\", \"ip_addr\") values('%s', '%s', '%s', '%s') on conflict(ip_addr) do update set session_token='%s', expiry='%s';", userStr, sessionToken, expiresAt.Format(time.DateTime), strings.Split(r.RemoteAddr, ":")[0], sessionToken, expiresAt.Format(time.DateTime)))
	  if inserterr != nil {
	  db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserterr))
	  fmt.Println(inserterr)
	  }*/
	_, updateerr := db.Exec(fmt.Sprintf("update tfldata.users set session_token='%s', mytz='%s' where username='%s' or email='%s';", sessionToken, acceptedTz, userStr, userStr))
	if updateerr != nil {
		db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s');", updateerr, time.Now().In(globalvars.NyLoc).Format(time.DateTime)))
		fmt.Printf("err: '%s'", updateerr)
	}
	maxAge := time.Until(expiresAt)

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionToken,
		MaxAge:   int(maxAge.Seconds()),
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

}

/* FUNCTION ITEMS */
func UploadFileToS3(f multipart.File, fn string, db *sql.DB, filetype string) {

	if strings.Contains(filetype, "image") {

		f.Seek(0, 0)
		var gettagerr error
		var tag *tiff.Tag
		x, exiferr := exif.Decode(f)
		if exiferr != nil {
			fmt.Println("Err decoding exif format")
			gettagerr = exiferr
		} else {
			tag, gettagerr = x.Get(exif.Orientation)
		}
		if gettagerr != nil {
			f.Seek(0, 0)
			buf := bytes.NewBuffer(nil)
			_, err := io.Copy(buf, f)
			if err != nil {
				fmt.Println("Err copying file to buffer")
			}

			newimg, _, decerr := imagego.Decode(buf)
			if decerr != nil {
				fmt.Println("dec err: " + decerr.Error())
				// we can actually exit program here
			}
			var compfile bytes.Buffer
			encerr := jpeg.Encode(&compfile, newimg, &jpeg.Options{
				Quality: 18,
			})
			if encerr != nil {
				fmt.Println(encerr)
			}
			_, err4 := globalvars.S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket:       aws.String(globalvars.S3Domain),
				Key:          aws.String("posts/images/" + fn),
				Body:         &compfile,
				ContentType:  &filetype,
				CacheControl: aws.String("max-age=31536000"),
			})

			if err4 != nil {
				fmt.Println("error on upload")
				fmt.Println(err4.Error())
			}

		} else {
			f.Seek(0, 0)
			imgtrn, _, err := imagego.Decode(f)
			if err != nil {
				fmt.Println(err)
				fmt.Println("err on imgtrn decoding")
			}
			if tag.Count == 1 && tag.Format() == tiff.IntVal {
				orientation, err := tag.Int(0)
				if err != nil {
					fmt.Println(err)
					fmt.Println("orientation err")
				}
				switch orientation {
				case 3: // rotate 180
					imgtrn = imaging.Rotate180(imgtrn)
				case 6: // rotate 270
					imgtrn = imaging.Rotate270(imgtrn)
				case 8: //rotate 90
					imgtrn = imaging.Rotate90(imgtrn)
				}
			}

			newbuf := bytes.NewBuffer(nil)
			trnencerr := jpeg.Encode(newbuf, imgtrn, nil)

			if trnencerr != nil {
				fmt.Println(trnencerr)
				fmt.Println("error encoding turned image")

			}

			newimg, _, decerr := imagego.Decode(newbuf)
			if decerr != nil {

				activityStr := "error on image decoding for uploadfiletos3"
				db.Exec(fmt.Sprintf("insert into tfldata.errlog(errmessage,activity,createdon) values (substr('%s',0,420), substr('%s',0,106), now());", decerr.Error(), activityStr))
				return
			}
			var compfile bytes.Buffer
			encerr := jpeg.Encode(&compfile, newimg, &jpeg.Options{
				Quality: 18,
			})

			if encerr != nil {
				fmt.Println(encerr)
			}
			_, err4 := globalvars.S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket:       aws.String(globalvars.S3Domain),
				Key:          aws.String("posts/images/" + fn),
				Body:         &compfile,
				ContentType:  &filetype,
				CacheControl: aws.String("max-age=31536000"),
			})

			if err4 != nil {
				fmt.Println("error on upload")
				fmt.Println(err4.Error())
			}

		}
	} else {

		_, err4 := globalvars.S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:       aws.String(globalvars.S3Domain),
			Key:          aws.String("posts/videos/" + fn),
			Body:         f,
			ContentType:  &filetype,
			CacheControl: aws.String("max-age=31536000"),
		})

		if err4 != nil {
			fmt.Println("error on upload")
			fmt.Println(err4.Error())
		}

	}
}
func SendNotificationToAllUsers(db *sql.DB, curUser string, fb_message_client *messaging.Client, opts *globaltypes.NotificationOpts) {

	usersNotInUtT, outperr := db.Query(fmt.Sprintf("select username from tfldata.users where username not in (select username from tfldata.users_to_threads where thread='%s');", opts.ExtraPayloadVal))
	if outperr != nil {
		activityStr := "Non issue logging on sendnotificationtoallusers first db output"
		db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"activity\", \"createdon\") values ('%s', '%s', now());", outperr, activityStr))
	}

	defer usersNotInUtT.Close()

	for usersNotInUtT.Next() {
		var user string
		usersNotInUtT.Scan(&user)
		db.Exec(fmt.Sprintf("insert into tfldata.users_to_threads(\"username\",\"thread\",\"is_subscribed\") values('%s', '%s', true) on conflict(username,thread) do nothing;", user, opts.ExtraPayloadVal))
	}

	var output *sql.Rows
	var outerr error
	if opts.IsTagged {
		output, outerr = db.Query(fmt.Sprintf("select username from tfldata.users_to_threads where thread='%s' and username != '%s';", opts.ExtraPayloadVal, curUser))
	} else {
		output, outerr = db.Query(fmt.Sprintf("select username from tfldata.users_to_threads where thread='%s' and username != '%s' and is_subscribed=true;", opts.ExtraPayloadVal, curUser))
	}
	if outerr != nil {
		activityStr := "Panic on sendnotificationtoallusers second db output"
		db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"activity\", \"createdon\") values ('%s', '%s', now());", outerr, activityStr))
	}

	defer output.Close()

	typePayload := make(map[string]string)
	typePayload["type"] = opts.NotificationPage
	typePayload[opts.ExtraPayloadKey] = opts.ExtraPayloadVal
	for output.Next() {
		var userToSend string

		usrToSendScnErr := output.Scan(&userToSend)

		if usrToSendScnErr == nil {
			var fcmToken sql.NullString
			var sendErr error
			tokenRow := db.QueryRow(fmt.Sprintf("select fcm_registration_id from tfldata.users where username='%s';", userToSend))
			scnerr := tokenRow.Scan(&fcmToken)
			if scnerr == nil {
				if fcmToken.Valid {

					_, sendErr = fb_message_client.Send(context.TODO(), &messaging.Message{

						Token: fcmToken.String,
						Data:  typePayload,
						Notification: &messaging.Notification{
							Title:    opts.NotificationTitle,
							Body:     opts.NotificationBody,
							ImageURL: "/assets/icon-512x512.png",
						},
						Webpush: &messaging.WebpushConfig{
							Notification: &messaging.WebpushNotification{
								Title: opts.NotificationTitle,
								Body:  opts.NotificationBody,
								Data:  typePayload,
								Image: "/assets/icon-512x512.png",
								Icon:  "/assets/icon-512x512.png",
								Tag:   typePayload["type"],
								Actions: []*messaging.WebpushNotificationAction{
									{
										Action: typePayload["type"],
										Title:  opts.NotificationTitle,
										Icon:   "/assets/icon-512x512.png",
									},
									{
										Action: typePayload[opts.ExtraPayloadKey],
										Title:  "NA",
										Icon:   "/assets/icon-512x512.png",
									},
								},
							},
						},
						Android: &messaging.AndroidConfig{
							Notification: &messaging.AndroidNotification{
								Title:       opts.NotificationTitle,
								Body:        opts.NotificationBody,
								ClickAction: typePayload["type"],
								ImageURL:    "/assets/icon-512x512.png",
								Icon:        "/assets/icon-512x512.png",
							},
						},
					})

					if sendErr != nil {
						activityStr := "Error sending notificationtoallusers"
						db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", sendErr.Error(), time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
						// fmt.Print(sendErr.Error() + " for user: " + userToSend)
						if strings.Contains(sendErr.Error(), "404") {
							db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id=null where username='%s';", userToSend))
							fmt.Println("updated " + userToSend + "'s fcm token")
						}
					}
				}
			}
		}
		//db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sendRes, time.Now().In(globalvars.NyLoc).Local().Format(time.DateTime)))

	}
	//db.Close()
}
func UploadTimeCapsuleToS3(f *os.File, fn string, yearsToStore string) {
	f.Seek(0, 0)

	defer f.Close()

	_, s3err := globalvars.S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(globalvars.S3Domain),
		Key:         aws.String("timecapsules/" + fn),
		ContentType: aws.String("application/octet-stream"),
		Body:        f,
		//StorageClass: types.StorageClassGlacier,
		Tagging: aws.String("YearsToStore=" + yearsToStore),
	})

	if s3err != nil {
		fmt.Println("error on upload")
		fmt.Println(s3err.Error())
	}
	/*
	   var yearstostore string
	   		row := db.QueryRow(fmt.Sprintf("select yearstostore from tfldata.timecapsule where tcfilename='%s';", postData.Capsulename))
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
	   		})
	*/
	defer os.Remove(fn)
}
func DeleteFileFromS3(delname string, delPath string) {

	_, err := globalvars.S3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(globalvars.S3Domain),
		Key:    aws.String(delPath + delname),
	})

	if err != nil {
		fmt.Println("error on file delete")
		fmt.Println(err.Error())
	}
}

func SendNotificationToSingleUser(db *sql.DB, fb_message_client *messaging.Client, fcmToken string, opts globaltypes.NotificationOpts) {
	typePayload := make(map[string]string)
	typePayload["type"] = opts.ExtraPayloadVal
	sentRes, sendErr := fb_message_client.Send(context.TODO(), &messaging.Message{
		Token: fcmToken,
		Notification: &messaging.Notification{
			Title:    opts.NotificationTitle,
			Body:     strings.ReplaceAll(opts.NotificationBody, "\\", ""),
			ImageURL: "/assets/icon-180x180.jpg",
		},

		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: opts.NotificationTitle,
				Body:  strings.ReplaceAll(opts.NotificationBody, "\\", ""),
				Data:  typePayload,
				Image: "/assets/icon-180x180.jpg",
				Icon:  "/assets/icon-96x96.jpg",
				Actions: []*messaging.WebpushNotificationAction{
					{
						Action: typePayload["type"],
						Title:  opts.NotificationTitle,
						Icon:   "/assets/icon-96x96.png",
					},
					{
						Action: typePayload["type"],
						Title:  "NA",
						Icon:   "/assets/icon-96x96.png",
					},
				},
			},
		},
	})
	if sendErr != nil {
		fmt.Print(sendErr)
	}
	db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sentRes, time.Now().In(globalvars.NyLoc).Local().Format(time.DateTime)))
}

func UploadPfpToS3(f multipart.File, fn string, r *http.Request, formInputIdentifier string) string {

	defer f.Close()
	ourfile, fileHeader, errfile := r.FormFile(formInputIdentifier)

	if errfile != nil {
		log.Fatal(errfile)
	}

	fileContents := make([]byte, fileHeader.Size)

	ourfile.Read(fileContents)
	filetype := http.DetectContentType(fileContents)
	f.Seek(0, 0)
	buf := bytes.NewBuffer(nil)
	_, err := io.Copy(buf, f)
	if err != nil {
		os.Exit(2)
	}

	f.Seek(0, 0)

	newimg, _, decerr := imagego.Decode(buf)
	if decerr != nil {
		log.Fatal("dec err: " + decerr.Error())
	}
	var compfile bytes.Buffer
	encerr := jpeg.Encode(&compfile, newimg, &jpeg.Options{
		Quality: 18,
	})
	if encerr != nil {
		fmt.Println(encerr)
	}
	tmpFileName := fn

	getout, geterr := globalvars.S3Client.GetObjectAttributes(context.TODO(), &s3.GetObjectAttributesInput{
		Bucket: aws.String(globalvars.S3Domain),
		Key:    aws.String("pfp/" + tmpFileName),
		ObjectAttributes: []types.ObjectAttributes{
			"ObjectSize",
		},
	})

	if geterr != nil {
		fmt.Println("We can ignore this image: " + geterr.Error())

	} else {

		if *getout.ObjectSize > 1 {
			tmpFileName = strings.ReplaceAll(strings.ReplaceAll(time.Now().Format(time.DateTime), " ", "_"), ":", "") + "_" + tmpFileName
			fn = tmpFileName
		}
	}

	if len(tmpFileName) > 55 {
		fn = tmpFileName[len(tmpFileName)-35:]
	}
	defer ourfile.Close()

	_, err4 := globalvars.S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:       aws.String(globalvars.S3Domain),
		Key:          aws.String("pfp/" + fn),
		Body:         &compfile,
		ContentType:  &filetype,
		CacheControl: aws.String("max-age=31536000"),
	})

	if err4 != nil {
		fmt.Println("error on upload")
		fmt.Println(err4.Error())
	}
	return fn
}
