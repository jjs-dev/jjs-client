package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Page struct {
	Message template.HTML
}

func getAuthCookie(r *http.Request) string {
	res, err := r.Cookie("auth")
	if err != nil {
		return ""
	}
	return res.Value
}

func (apiClient *Api) write500Error(w http.ResponseWriter, err error) {
	w.WriteHeader(500)
	message := "500 Internal Server Error"
	if apiClient.debug {
		message += "( " + err.Error() + ")"
	}
	_, _ = fmt.Fprintf(w, message)
}

func loadTemplate(path string) (*template.Template, error) {
	t, err := template.ParseFiles("./templates/" + path, "./templates/base.html")
	return t, err
}

func (apiClient *Api) renderPage(w http.ResponseWriter, templatePath string, page Page) {
	t, err := loadTemplate(templatePath)
	if err == nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		if pusher, ok := w.(http.Pusher); ok {
			if err := pusher.Push("/bootstrap.min.css", nil); err != nil {
				if apiClient.debug {
					apiClient.logger.Println("Error while pushing Bootstrap css: " + err.Error())
				}
			}
		}
		err = t.ExecuteTemplate(w, "base", page)
		if err != nil {
			apiClient.write500Error(w, err)
		}
	} else {
		w.WriteHeader(404) // TODO: throw 500 error when file has been found, but there was an actual error
		if apiClient.debug {
			fmt.Println("Error during template " + templatePath + " load: " + err.Error())
		}
		_, _ = fmt.Fprintf(w, "404 Not Found")
	}
}

func renderMessage(query *url.Values) Page {
	message := query.Get("message")
	if message == "" {
		return Page{Message: ""}
	} else {
		color := query.Get("color")
		if color == "" {
			color = "primary"
		}
		return Page{Message: "<div class=\"alert alert-" + template.HTML(color) + "\" role=\"alert\">" +
			template.HTML(message) +
			"</div>"}
	}
}

func (apiClient *Api) authorizeHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			if apiClient.debug {
				apiClient.logger.Println("Error during parsing authorize POST form: " + err.Error())
			}
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "400 Bad Request")
		}
		login := r.FormValue("login")
		password := r.FormValue("password")
		key, err := apiClient.authorize(login, password)
		if err == nil {
			cookie := http.Cookie{
				Name: "auth",
				Value: key,
				Path: "/",
				Expires: time.Now().Add(time.Hour * 24),
			}
			http.SetCookie(w, &cookie)
			http.Redirect(w, r, "/", 301)
		} else {
			message := "Check credentials"
			if apiClient.debug {
				message += " (" + err.Error() + ")"
			}
			http.Redirect(w, r, "/login?message=" + message + "&color=danger", 301)
		}
	} else {
		values := r.URL.Query()
		apiClient.renderPage(w, "login.html", renderMessage(&values))
	}
}

func (apiClient *Api) authenticateHandle(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("auth")
	if err == nil {
		res, err := apiClient.getApiVersion(cookie.Value)
		if err != nil {
			apiClient.write500Error(w, err)
		}
		w.WriteHeader(200)
		_, err = w.Write([]byte(res))
		if err != nil {
			apiClient.write500Error(w, err)
		}
	} else {
		apiClient.write500Error(w, err)
	}
}

func (apiClient *Api) createUserHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			apiClient.logger.Println("Error during parsing createUser POST form: " + err.Error())
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "400 Bad Request")
		}
		login := r.FormValue("login")
		password := r.FormValue("password")
		groups := []string{"Participants"}
		customGroup := r.FormValue("group")
		if r.FormValue("groupNeeded") != "" && customGroup != "" {
			groups = append(groups, customGroup)
		}
		_, err := apiClient.createUser(getAuthCookie(r), login, password, groups)
		if err == nil {
			http.Redirect(w, r, "/createUser?message=Done!&color=success", 301)
		} else {
			http.Redirect(w, r, "/createUser?message=Failed to create user&color=danger", 301)
		}
	} else {
		values := r.URL.Query()
		apiClient.renderPage(w, "createUser.html", renderMessage(&values))
	}
}

func (apiClient *Api) staticContentHandle(w http.ResponseWriter, r *http.Request) {
	finalPath := r.URL.Path[strings.Index(r.URL.Path, "/"):]
	data, err := ioutil.ReadFile("./static" + finalPath)
	if err != nil {
		data, err = ioutil.ReadFile("./static/icons" + finalPath)
	}
	if err != nil {
		if apiClient.debug {
			apiClient.logger.Println("Not found path: " +  r.URL.Path)
		}
		w.WriteHeader(404)
		_, _ = fmt.Fprintf(w, "404 Not Found")
	} else {
		if strings.HasSuffix(r.URL.Path, "css") {
			w.Header().Set("Content-Type", "text/css")
		} else if strings.HasSuffix(r.URL.Path, "js") {
			w.Header().Set("Content-Type", "application/javascript")
		} else if strings.HasSuffix(r.URL.Path, "svg") {
			w.Header().Set("Content-Type", "image/svg+xml")
		} else if strings.HasSuffix(r.URL.Path, "ico") {
			w.Header().Set("Content-Type", "image/x-icon")
		} else {
			w.Header().Set("Content-Type", "text/plain")
		}
		w.Header().Set("Cache-Control", "max-age=3600")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}
}

func main() {
	apiURL, found := os.LookupEnv("JJS_API_URL")
	if !found {
		log.Fatal("Please set the JJS_API_URL environmental variable")
	}
	logLocation, found := os.LookupEnv("LOG_LOCATION")
	var logFile *os.File
	if !found {
		log.Println("No LOG_LOCATION found, writing to stdout")
		logFile = os.Stdout
	} else {
		innerLogFile, err := os.OpenFile(logLocation, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Println("Failed to open LOG_LOCATION file, writing to stdout")
			log.Println(err.Error())
			logFile = os.Stdout
		} else {
			logFile = innerLogFile
			defer innerLogFile.Close()
		}
	}
	_, found = os.LookupEnv("DEBUG")
	client := initialize(apiURL, logFile, found)
	http.HandleFunc("/authenticate", client.authenticateHandle)
	http.HandleFunc("/login", client.authorizeHandle)
	http.HandleFunc("/createUser", client.createUserHandle)
	http.HandleFunc("/", client.staticContentHandle)
	address := ":80"
	if client.debug {
		address = ":8080"
	}
	certFile, foundCert := os.LookupEnv("CERT_FILE")
	keyFile, foundKey := os.LookupEnv("KEY_FILE")
	if foundCert && foundKey {
		err := http.ListenAndServeTLS(":443", certFile, keyFile, nil)
		if err != nil {
			client.logger.Panic(err)
		}
	}
	err := http.ListenAndServe(address, nil)
	if err != nil {
		client.logger.Panic(err)
	}
}
