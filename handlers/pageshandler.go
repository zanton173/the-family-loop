package pages

import (
	"net/http"
	"os"
	"strings"
	"text/template"
)

func PagesHandler(w http.ResponseWriter, r *http.Request) {

	bs, _ := os.ReadFile("navigation.html")
	navtmple := template.New("Navt")
	tm, _ := navtmple.Parse(string(bs))

	switch {
	case r.URL.Path == "/":
		tmpl := template.Must(template.ParseFiles("index.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case r.URL.Path == "/groupchat":
		tmpl := template.Must(template.ParseFiles("groupchat.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case r.URL.Path == "/home":
		tmpl := template.Must(template.ParseFiles("index.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case r.URL.Path == "/calendar":
		tmpl := template.Must(template.ParseFiles("calendar.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case r.URL.Path == "/time-capsule":
		tmpl := template.Must(template.ParseFiles("timecapsule.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case r.URL.Path == "/customersupport":
		tmpl := template.Must(template.ParseFiles("bugreport.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case r.URL.Path == "/games/simple-shades":
		tmpl := template.Must(template.ParseFiles("simpleshades.html"))
		tmpl.Execute(w, nil)
	case r.URL.Path == "/games/stackerz":
		tmpl := template.Must(template.ParseFiles("stackerz.html"))
		tmpl.Execute(w, nil)
	case r.URL.Path == "/games/catchit":
		tmpl := template.Must(template.ParseFiles("catchit.html"))
		tmpl.Execute(w, nil)
	case r.URL.Path == "/games/pong":
		tmpl := template.Must(template.ParseFiles("pong.html"))
		tmpl.Execute(w, nil)
	case r.URL.Path == "/admin-dashboard":
		tmpl := template.Must(template.ParseFiles("admindash.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case strings.HasSuffix(r.URL.Path, ".css") || strings.HasSuffix(r.URL.Path, ".ttf") || strings.HasSuffix(r.URL.Path, ".js") || strings.HasSuffix(r.URL.Path, ".png"):
		w.Header().Set("Cache-Control", "public, max-age=7776000")
		http.ServeFile(w, r, strings.SplitAfterN(r.URL.Path, "/", 2)[1])
	case strings.HasSuffix(r.URL.Path, ".js"):
		if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Cache-Control", "public, max-age=7776000")
			http.ServeFile(w, r, strings.SplitAfterN(r.URL.Path, "/", 2)[1])
		}
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}

}
