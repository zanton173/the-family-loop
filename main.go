package main

import (
	"html/template"
	"log"
	"net/http"
)

func main() {

	pagesHandler := func(w http.ResponseWriter, r *http.Request) {
		///http.Handle("/images/", http.FileServer(http.Dir("images")))
		switch r.URL.Path {
		case "/home":
			tmpl := template.Must(template.ParseFiles("index.html"))
			tmpl.Execute(w, nil)
		default:
			http.Redirect(w, r, "/home", 301)
		}

	}

	http.HandleFunc("/", pagesHandler)
	http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))
	log.Fatal(http.ListenAndServe(":80", nil))
}
