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

type postsrow struct {
	id          int64
	title       string
	description string
	image_name  string
}

var awskey string
var awskeysecret string

func main() {
	err := godotenv.Load()
	var homeposts []postsrow
	if err != nil {
		log.Fatal("Error loading .env file")
		os.Exit(1)
	}
	dbpass := os.Getenv("DB_PASS")
	awskey = os.Getenv("AWS_ACCESS_KEY")
	awskeysecret = os.Getenv("AWS_ACCESS_SECRET")

	connStr := fmt.Sprintf("postgresql://tfldbrole:%s@localhost/tfl?sslmode=disable", dbpass)
	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	output, err := db.Query("select * from tfldata.posts;")
	if err != nil {
		log.Fatal(err)
	}
	defer output.Close()

	for output.Next() {
		var postrows postsrow
		if err := output.Scan(&postrows.id, &postrows.title, &postrows.description, &postrows.image_name); err != nil {
			log.Fatal(err)
		}
		homeposts = append(homeposts, postrows)
		//fmt.Println(len(postrow))

	}
	//fmt.Println(homeposts[0].title)
	pagesHandler := func(w http.ResponseWriter, r *http.Request) {
		//tmpl := template.Must(template.ParseFiles("index.html"))
		//tmpl.Execute(w, nil)
		switch r.URL.Path {
		case "/home":
			tmpl := template.Must(template.ParseFiles("index.html"))

			tmpl.Execute(w, homeposts)
		default:
			http.Redirect(w, r, "/home", http.StatusPermanentRedirect)
		}

	}

	h2 := func(w http.ResponseWriter, r *http.Request) {
		upload, filename, _ := r.FormFile("image_name")
		createTFLBucketAndUpload(awskey, awskeysecret, false, upload, filename.Filename)
		_, err := db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"image_name\") values('%s', '%s', '%s');", r.PostFormValue("title"), r.PostFormValue("description"), filename.Filename))
		if err != nil {
			log.Fatal(err)
		}
		//fmt.Println(resp)
	}
	/*h3 := func(w http.ResponseWriter, r *http.Request) {
		upload, filename, err := r.FormFile("image_name")
		if err != nil {
			log.Fatal(err)
		}

		//uploadPostPhotoTos3(upload, filename.Filename, s3_client)

	}*/
	http.HandleFunc("/", pagesHandler)
	http.HandleFunc("/create-post", h2)
	//http.HandleFunc("/upload-file", h3)
	//http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))
	log.Fatal(http.ListenAndServe(":80", nil))
}

func createTFLBucketAndUpload(k string, s string, bucketexists bool, f multipart.File, fn string) {

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
                "s3:GetObject",
                "s3:PutObject"
            ],
            "Resource": "arn:aws:s3:::the-family-loop` + `-customer-hash/posts/*"
        }
    ]}`),
		})
	if err3 != nil {
		fmt.Println(err3)
	}
	_, err4 := client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String("the-family-loop" + "-customer-hash"),
		Key:    aws.String("posts/" + fn),
		Body:   f,
	})

	if err4 != nil {
		fmt.Println("error on upload")
		fmt.Println(err)
	}
	defer f.Close()
}

/*func uploadPostPhotoTos3(f multipart.File, fn string, client *s3.Client) {

}*/
