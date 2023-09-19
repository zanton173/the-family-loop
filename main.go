package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

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
	if err != nil {
		log.Fatal("Error loading .env file")
		os.Exit(1)
	}
	dbpass := os.Getenv("DB_PASS")

	connStr := fmt.Sprintf("postgresql://tfldbrole:%s@localhost/tfl?sslmode=disable", dbpass)
	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	/*output, err := db.Query("select count(*) from tfldata.posts;")
	if err != nil {
		log.Fatal(err)
	}
	defer output.Close()
	resp := []postsrow{}
	for output.Next() {
		var row postsrow
		if err := output.Scan(&row.id, &row.title, &row.description, &row.image_name); err != nil {
			log.Fatal(err)
		}
		resp = append(resp, row)

	}
	fmt.Println(resp)*/
	pagesHandler := func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("index.html"))
		tmpl.Execute(w, nil)
		/*
			switch r.URL.Path {
			case "/home":
				tmpl := template.Must(template.ParseFiles("index.html"))
				tmpl.Execute(w, nil)

			default:
				fmt.Println("nothing here")
				//http.Redirect(w, r, "/home", http.StatusPermanentRedirect)
			}
		*/
	}
	h2 := func(w http.ResponseWriter, r *http.Request) {
		log.Print(r.Header.Get("HX-Request"))
		fmt.Println(r.PostFormValue("title"))
		fmt.Println(r.PostFormValue("description"))
		fmt.Println(r.PostFormValue("image_name"))
	}
	http.HandleFunc("/", pagesHandler)
	http.HandleFunc("/create-post", h2)
	http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))
	log.Fatal(http.ListenAndServe(":80", nil))
}
