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

	"firebase.google.com/go/messaging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/disintegration/imaging"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
)

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
	defer db.Close()
	return scnerr == nil, username.String, handlerForLogin

}
func GetEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		os.Exit(1)
	}
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
func UploadFileToS3(f multipart.File, fn string, db *sql.DB, filetype string, s3Client *s3.Client) {
	s3Domain := os.Getenv("S3_BUCKET_NAME")
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
			_, err4 := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket:       aws.String(s3Domain),
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
			_, err4 := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket:       aws.String(s3Domain),
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

		_, err4 := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:       aws.String(s3Domain),
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
							ImageURL: "/assets/icon-180x180.jpg",
						},
						Webpush: &messaging.WebpushConfig{
							Notification: &messaging.WebpushNotification{
								Title: opts.NotificationTitle,
								Body:  opts.NotificationBody,
								Data:  typePayload,
								Image: "/assets/icon-180x180.jpg",
								Icon:  "/assets/icon-96x96.png",
								Actions: []*messaging.WebpushNotificationAction{
									{
										Action: typePayload["type"],
										Title:  opts.NotificationTitle,
										Icon:   "/assets/icon-96x96.png",
									},
									{
										Action: typePayload[opts.ExtraPayloadKey],
										Title:  "NA",
										Icon:   "/assets/icon-96x96.png",
									},
								},
							},
						},
						Android: &messaging.AndroidConfig{
							Notification: &messaging.AndroidNotification{
								Title:    opts.NotificationTitle,
								Body:     opts.NotificationBody,
								ImageURL: "/assets/icon-180x180.jpg",
								Icon:     "/assets/icon-96x96.png",
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
		//db.Exec(fmt.Sprintf("insert into tfldata.sent_notification_log(\"notification_result\", \"createdon\") values('%s', '%s');", sendRes, time.Now().In(nyLoc).Local().Format(time.DateTime)))

	}
}
