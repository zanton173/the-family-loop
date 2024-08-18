package pages

import (
	"net/http"
	"os"
	"text/template"
)

func PagesHandler(w http.ResponseWriter, r *http.Request) {

	bs, _ := os.ReadFile("navigation.html")
	navtmple := template.New("Navt")
	tm, _ := navtmple.Parse(string(bs))

	switch r.URL.Path {
	case "/groupchat":
		tmpl := template.Must(template.ParseFiles("groupchat.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case "/home":
		tmpl := template.Must(template.ParseFiles("index.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case "/calendar":
		tmpl := template.Must(template.ParseFiles("calendar.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case "/time-capsule":
		tmpl := template.Must(template.ParseFiles("timecapsule.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case "/customersupport":
		tmpl := template.Must(template.ParseFiles("bugreport.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	case "/games/simple-shades":
		tmpl := template.Must(template.ParseFiles("simpleshades.html"))
		tmpl.Execute(w, nil)
	case "/games/stackerz":
		tmpl := template.Must(template.ParseFiles("stackerz.html"))
		tmpl.Execute(w, nil)
	case "/games/catchit":
		tmpl := template.Must(template.ParseFiles("catchit.html"))
		tmpl.Execute(w, nil)
	case "/games/newgame":
		tmpl := template.Must(template.ParseFiles("newgame.html"))
		tmpl.Execute(w, nil)
	case "/admin-dashboard":
		tmpl := template.Must(template.ParseFiles("admindash.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	default:
		tmpl := template.Must(template.ParseFiles("index.html"))
		tmpl.Execute(w, nil)
		tm.Execute(w, nil)
	}

}
