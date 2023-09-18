package main

import (
	"html/template"
	"log"
	"net/http"
)

func main() {

	indexHandler := func(w http.ResponseWriter, r *http.Request) {
		///http.Handle("/images/", http.FileServer(http.Dir("images")))
		switch r.URL.Path {
		case "/home":
			tmpl := template.Must(template.ParseFiles("index.html"))
			tmpl.Execute(w, nil)
		default:
			http.Redirect(w, r, "/home", 301)
		}

	}

	http.HandleFunc("/", indexHandler)
	http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))
	log.Fatal(http.ListenAndServe(":83", nil))
}
