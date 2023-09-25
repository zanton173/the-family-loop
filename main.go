package main

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Postsrow struct {
	Id          int64
	Title       string
	Description string
	File_name   string
	File_type   string
}

var awskey string
var awskeysecret string

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		os.Exit(1)
	}
	dbpass := os.Getenv("DB_PASS")
	awskey = os.Getenv("AWS_ACCESS_KEY")
	awskeysecret = os.Getenv("AWS_ACCESS_SECRET")
	var storedCount string

	// Connect to database
	connStr := fmt.Sprintf("postgresql://tfldbrole:%s@localhost/tfl?sslmode=disable", dbpass)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.QueryRow("select count(*) from tfldata.posts;").Scan(&storedCount)

	var postTmpl *template.Template
	var tmerr error
	pagesHandler := func(w http.ResponseWriter, r *http.Request) {
		//tmpl := template.Must(template.ParseFiles("index.html"))
		//tmpl.Execute(w, nil)

		switch r.URL.Path {
		case "/home":
			tmpl := template.Must(template.ParseFiles("index.html"))

			tmpl.Execute(w, nil)
		default:
			http.Redirect(w, r, "/home", http.StatusPermanentRedirect)
		}

	}

	getPostsHandler := func(w http.ResponseWriter, r *http.Request) {
		output, err := db.Query("select * from tfldata.posts order by id DESC;")
		var count string
		db.QueryRow("select count(*) from tfldata.posts;").Scan(&count)

		var dataStr string
		if err != nil {
			log.Fatal(err)
		}

		defer output.Close()
		if storedCount == count {
			for output.Next() {
				var postrows Postsrow

				if err := output.Scan(&postrows.Id, &postrows.Title, &postrows.Description, &postrows.File_name, &postrows.File_type); err != nil {
					log.Fatal(err)

				}
				if strings.Contains(postrows.File_type, "image") {
					dataStr = fmt.Sprintf("<div class='card my-4' style='border-radius: 14px;'><img src='https://the-family-loop-customer-hash.s3.amazonaws.com/posts/images/%s' style='border-radius: 14px;' alt='%s' /><div class='card-body'><h5 class='card-title'>%s</h5><p class='card-text'>%s</p><a href='#' class='btn btn-primary'>Open a post</a></div></div>", postrows.File_name, postrows.File_name, postrows.Title, postrows.Description)
				} else if strings.Contains(postrows.File_type, "video") {
					dataStr = fmt.Sprintf("<div class='card my-4' style='border-radius: 14px;'><video controls id='video'><source src='https://the-family-loop-customer-hash.s3.amazonaws.com/posts/videos/%s' type='%s'></video><div class='card-body'><h5 class='card-title'>%s</h5><p class='card-text'>%s</p><a href='#' class='btn btn-primary'>Open a post</a></div></div>", postrows.File_name, postrows.File_type, postrows.Title, postrows.Description)
				}

				postTmpl, tmerr = template.New("tem").Parse(dataStr)
				if tmerr != nil {
					fmt.Print(tmerr)
				}
				postTmpl.Execute(w, nil)

			}

		}

	}
	getPostCountHandler := func(w http.ResponseWriter, r *http.Request) {

		dataStr := "<script>dbCount = " + storedCount + "; returnCountOfPosts()</script>"
		tmp, err := template.New("but").Parse(dataStr)
		if err != nil {
			fmt.Println(err)
		}
		tmp.Execute(w, nil)
		var count string

		db.QueryRow("select count(*) from tfldata.posts;").Scan(&count)

		storedCount = count

	}
	h2 := func(w http.ResponseWriter, r *http.Request) {
		upload, filename, errfile := r.FormFile("file_name")

		if errfile != nil {
			log.Fatal(errfile)
		}

		// Returning a filetype from the createandupload function
		// somehow gets the right filetype
		filetype := createTFLBucketAndUpload(awskey, awskeysecret, false, upload, filename.Filename, r)

		//fmt.Println(filetype)

		_, err := db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"file_name\", \"file_type\") values('%s', '%s', '%s', '%s');", r.PostFormValue("title"), r.PostFormValue("description"), filename.Filename, filetype))

		if err != nil {
			log.Fatal(err)
		}
		defer upload.Close()

	}
	/*h3 := func(w http.ResponseWriter, r *http.Request) {
		upload, filename, err := r.FormFile("file_name")
		if err != nil {
			log.Fatal(err)
		}

		//uploadPostPhotoTos3(upload, filename.Filename, s3_client)

	}*/
	http.HandleFunc("/", pagesHandler)
	http.HandleFunc("/create-post", h2)

	http.HandleFunc("/get-posts", getPostsHandler)
	http.HandleFunc("/new-posts", getPostCountHandler)

	//http.HandleFunc("/upload-file", h3)
	//http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))
	log.Fatal(http.ListenAndServe(":80", nil))
}

func createTFLBucketAndUpload(k string, s string, bucketexists bool, f multipart.File, fn string, r *http.Request) string {

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(k, s, "")),
	)

	if err != nil {
		log.Fatal(err)
		os.Exit(2)
	}

	client := s3.NewFromConfig(cfg)

	listbuck, err := client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})

	if err != nil {
		log.Fatal(err)
	}
	for _, val := range listbuck.Buckets {
		if strings.EqualFold(*val.Name, *aws.String("the-family-loop" + "-customer-hash")) {
			//fmt.Println("Bucket exists!")
			bucketexists = true
		} else {
			//fmt.Println("lets create the bucket")
			bucketexists = false
		}
	}
	if !bucketexists {
		_, err := client.CreateBucket(context.TODO(),
			&s3.CreateBucketInput{
				Bucket: aws.String("the-family-loop" + "-customer-hash"),
			},
		)
		if err != nil {
			log.Fatal(err)
		}
	}

	_, err2 := client.PutPublicAccessBlock(context.TODO(),
		&s3.PutPublicAccessBlockInput{
			Bucket: aws.String("the-family-loop" + "-customer-hash"),
			PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
				BlockPublicPolicy:     false,
				BlockPublicAcls:       false,
				RestrictPublicBuckets: false,
				IgnorePublicAcls:      true,
			},
		})
	if err2 != nil {
		log.Fatal(err2)

	}
	_, err3 := client.PutBucketPolicy(context.TODO(),
		&s3.PutBucketPolicyInput{
			Bucket: aws.String("the-family-loop" + "-customer-hash"),
			Policy: aws.String(`{"Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Statement",
            "Effect": "Allow",
            "Principal": "*",
            "Action": [
                "s3:GetObject*",
                "s3:PutObject*"
            ],
            "Resource": "arn:aws:s3:::the-family-loop` + `-customer-hash/posts/*"
        }
    ]}`),
		})
	if err3 != nil {
		fmt.Println(err3)
	}

	defer f.Close()
	ourfile, fileHeader, errfile := r.FormFile("file_name")

	if errfile != nil {
		log.Fatal(errfile)
	}

	fileContents := make([]byte, fileHeader.Size)

	ourfile.Read(fileContents)
	filetype := http.DetectContentType(fileContents)
	//fmt.Println(filetype)
	defer ourfile.Close()

	//fileChan := make(chan s3.GetObjectAttributesOutput, 1)

	if strings.Contains(filetype, "image") {

		_, err4 := client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:      aws.String("the-family-loop" + "-customer-hash"),
			Key:         aws.String("posts/images/" + fn),
			Body:        f,
			ContentType: &filetype,
		})

		if err4 != nil {
			fmt.Println("error on upload")
			fmt.Println(err)
		}
		/*for range fileChan {
			go func() {
				nameOut, errname := client.GetObjectAttributes(context.TODO(), &s3.GetObjectAttributesInput{
					Bucket: aws.String("the-family-loop" + "-customer-hash"),
					Key:    aws.String("posts/images/" + fn),
					ObjectAttributes: []types.ObjectAttributes{
						"ObjectSize",
					},
				})
				if errname != nil {
					fmt.Println("Some sort of error :(")
					log.Fatal(errname)
				}
				fileChan <- *nameOut
			}()
		}*/
	} else if strings.Contains(filetype, "video") {

		_, err4 := client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:      aws.String("the-family-loop" + "-customer-hash"),
			Key:         aws.String("posts/videos/" + fn),
			Body:        f,
			ContentType: &filetype,
		})

		if err4 != nil {
			fmt.Println("error on upload")
			fmt.Println(err)
		}

		/* go func() {
			nameOut, errname := client.GetObjectAttributes(context.TODO(), &s3.GetObjectAttributesInput{
				Bucket: aws.String("the-family-loop" + "-customer-hash"),
				Key:    aws.String("posts/videos/" + fn),
				ObjectAttributes: []types.ObjectAttributes{
					"ObjectSize",
				},
			})
			if errname != nil {
				fmt.Println("Some sort of error :(")
				log.Fatal(errname)
			}
			fileChan <- *nameOut

		}()
		for val := range fileChan {
			fmt.Println(val.ObjectSize)

		}*/

	} else {
		fmt.Println("Unknown file type. How did this get here?")
	}
	return filetype
}

/*func uploadPostPhotoTos3(f multipart.File, fn string, client *s3.Client) {

}*/
