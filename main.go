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
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type postsrow struct {
	id          int64
	title       string
	description string
	image_name  string
}

func main() {
	err := godotenv.Load()
	var homeposts []postsrow
	if err != nil {
		log.Fatal("Error loading .env file")
		os.Exit(1)
	}
	dbpass := os.Getenv("DB_PASS")
	awskey := os.Getenv("AWS_ACCESS_KEY")
	awskeysecret := os.Getenv("AWS_ACCESS_SECRET")

	s3_client := createTFLBucket(awskey, awskeysecret, false)

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
	fmt.Println(homeposts[0])
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
	h2 := func(w http.ResponseWriter, r *http.Request) {
		_, filename, _ := r.FormFile("image_name")
		fmt.Println(r.PostFormValue("title"))
		fmt.Println(r.PostFormValue("description"))
		_, err := db.Exec(fmt.Sprintf("insert into tfldata.posts(\"title\", \"description\", \"image_name\") values('%s', '%s', '%s');", r.PostFormValue("title"), r.PostFormValue("description"), filename.Filename))
		if err != nil {
			log.Fatal(err)
		}
		//fmt.Println(resp)
	}
	h3 := func(w http.ResponseWriter, r *http.Request) {
		upload, filename, err := r.FormFile("image_name")
		if err != nil {
			log.Fatal(err)
		}

		uploadPostPhotoTos3(upload, filename.Filename, s3_client)
	}
	http.HandleFunc("/", pagesHandler)
	http.HandleFunc("/create-post", h2)
	http.HandleFunc("/upload-file", h3)
	http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))
	log.Fatal(http.ListenAndServe(":80", nil))
}

func createTFLBucket(k string, s string, bucketexists bool) *s3.Client {

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
		if strings.Contains(*val.Name, *aws.String("the-family-loop" + "-customer-hash")) {
			//fmt.Println("Bucket exists!")
			bucketexists = true
		} else {
			//fmt.Println("lets create the bucket")
			bucketexists = false
		}
	}
	if !bucketexists {
		result, err := client.CreateBucket(context.TODO(),
			&s3.CreateBucketInput{
				Bucket: aws.String("the-family-loop" + "-customer-hash"),
			},
		)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(result)
	}
	return client

}
func uploadPostPhotoTos3(f multipart.File, fn string, client *s3.Client) {
	defer f.Close()

	client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String("the-family-loop" + "-customer-hash"),
		Key:    aws.String("posts/" + fn),
		Body:   f,
	})
}
