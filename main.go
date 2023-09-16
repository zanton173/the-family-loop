package main

import (
	"html/template"
	"log"
	"net/http"
)

func main() {
	h1 := func(w http.ResponseWriter, r *http.Request) {

		switch r.URL.Path {
		case "/home":
			tmpl := template.Must(template.ParseFiles("./index.html"))
			tmpl.Execute(w, nil)

		case "/":
			tmpl := template.Must(template.ParseFiles("./404.html"))
			tmpl.Execute(w, nil)
		}

	}

	http.HandleFunc("/", h1)
	log.Fatal(http.ListenAndServe(":8040", nil))
}
