package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

type Page struct {
	Again string
}

func write500Error(w http.ResponseWriter, err error) {
	w.WriteHeader(500)
	_, _ = fmt.Fprintf(w, "500 Internal Server Error (" + err.Error() + ")")
}

func loadTemplate(path string) (*template.Template, error) {
	t, err := template.ParseFiles("./templates" + path[strings.Index(path, "/"):] + ".html")
	return t, err
}

func renderPage(w http.ResponseWriter, r *http.Request, page Page) {
	t, err := loadTemplate(r.URL.Path)
	if err == nil {
		_ = t.Execute(w, page)
	} else {
		w.WriteHeader(404)
		_, _ = fmt.Fprintf(w, "404 Not Found")
	}
}

func (apiClient Api) authorizeHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			log.Println(err.Error())
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "400 Bad Request")
		}
		login := r.FormValue("login")
		password := r.FormValue("password")
		key, err := apiClient.authorize(login, password)
		if err == nil {
			cookie := http.Cookie{Name: "auth", Value: key}
			http.SetCookie(w, &cookie)
			http.Redirect(w, r, "/authenticate", 301)
		} else {
			http.Redirect(w, r, "/login?again=1", 301)
		}
	} else {
		if r.URL.Query().Get("again") == "" {
			renderPage(w, r, Page{Again: ""})
		} else {
			renderPage(w, r, Page{Again: "Check credentials"})
		}
	}
}

func (apiClient Api) authenticateHandle(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("auth")
	if err == nil {
		res, err := apiClient.authenticate(cookie.Value)
		if err != nil {
			write500Error(w, err)
		}
		w.WriteHeader(200)
		if res {
			_, _ = w.Write([]byte("Ok."))
		} else {
			_, _ = w.Write([]byte("Not ok."))
		}
	} else {
		write500Error(w, err)
	}
}

func (apiClient Api) createUserHandle(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	login := query.Get("login")
	password := query.Get("password")
	id, err := apiClient.createUser(login, password, []string{"Participants"})
	if err == nil {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(id))
	} else {
		write500Error(w, err)
	}
}


func main() {
	apiURL, found := os.LookupEnv("JJS_API_URL")
	if !found {
		log.Fatal("Please set the JJS_API_URL environmental variable")
	}
	client := initialize(apiURL)
	http.HandleFunc("/authenticate", client.authenticateHandle)
	http.HandleFunc("/login", client.authorizeHandle)
	http.HandleFunc("/createUser", client.createUserHandle)
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Panic(err)
	}
}
