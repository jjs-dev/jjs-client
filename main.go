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
    "sync"
    "time"
)

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
    t, err := template.ParseFiles("./templates/"+path, "./templates/base.html")
    return t, err
}

func (apiClient *Api) renderPage(w http.ResponseWriter, templatePath string, page interface{}) {
    t, err := loadTemplate(templatePath)
    if err == nil {
        w.Header().Set("Content-Type", "text/html")
        w.WriteHeader(200)
        if pusher, ok := w.(http.Pusher); ok {
            if err := pusher.Push("/static/bootstrap.min.css", nil); err != nil {
                if apiClient.debug {
                    apiClient.logger.Println("Error while pushing Bootstrap css: " + err.Error())
                }
            }
        }
        err = t.ExecuteTemplate(w, "base", page)
        if err != nil {
            apiClient.logger.Println(err.Error())
        }
    } else {
        w.WriteHeader(404) // TODO: throw 500 error when file has been found, but there was an actual error
        if apiClient.debug {
            fmt.Println("Error during template " + templatePath + " load: " + err.Error())
        }
        _, _ = fmt.Fprintf(w, "404 Not Found")
    }
}

func renderMessage(query *url.Values) SimplePage {
    message := query.Get("message")
    if message == "" {
        return SimplePage{Message: ""}
    } else {
        color := query.Get("color")
        if color == "" {
            color = "primary"
        }
        return SimplePage{Message: "<div class=\"alert alert-" + template.HTML(color) + "\" role=\"alert\">" +
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
            return
        }
        login := r.FormValue("login")
        password := r.FormValue("password")
        key, err := apiClient.authorize(login, password)
        if err == nil {
            cookie := http.Cookie{
                Name:    "auth",
                Value:   key,
                Path:    "/",
                Expires: time.Now().Add(time.Hour * 24),
            }
            http.SetCookie(w, &cookie)
            http.Redirect(w, r, "/", 301)
        } else {
            message := "Check credentials"
            if apiClient.debug {
                message += " (" + err.Error() + ")"
            }
            http.Redirect(w, r, "/login?message="+message+"&color=danger", 301)
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
            return
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
            return
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

func (apiClient *Api) submitRunHandle(w http.ResponseWriter, r *http.Request, contest *Contest, problemId string) {
    if r.Method == "POST" {
        if err := r.ParseMultipartForm(32 << 20); err != nil {
            apiClient.logger.Println("Error during parsing createUser POST form: " + err.Error())
            w.WriteHeader(400)
            _, _ = fmt.Fprintf(w, "400 Bad Request")
            return
        }
        file, _, err := r.FormFile("code")
        if err != nil {
            apiClient.write500Error(w, err)
            return
        }
        defer file.Close()
        runCode, err := ioutil.ReadAll(file)
        if err != nil {
            apiClient.write500Error(w, err)
            return
        }
        key := getAuthCookie(r)
        toolchainId := r.FormValue("toolchainID")
        id, err := apiClient.sendRun(key, toolchainId, runCode, problemId, contest.Id)
        if err != nil {
            apiClient.write500Error(w, err)
        } else {
            w.WriteHeader(200)
            _, _ = fmt.Fprintf(w, "Done! Your run ID: %d", id)
        }
    } else {
        toolchains, err := apiClient.listToolChains(getAuthCookie(r))
        if err != nil {
            apiClient.write500Error(w, err)
            return
        }
        var problem Problem
        for _, problem = range contest.Problems {
            if problem.Id == problemId {
                break
            }
        }
        apiClient.renderPage(w, "sendRun.html", ProblemPage{Contest: contest, ToolChains: toolchains, Problem: &problem})
    }
}

func (apiClient *Api) contestNameHandle(w http.ResponseWriter, r *http.Request, contestName, problemId string) {
    contest, err := apiClient.findContest(getAuthCookie(r), contestName)
    if err != nil {
        apiClient.write500Error(w, err)
        return
    }
    if problemId == "" {
        apiClient.renderPage(w, "contestMain.html", ContestMainPage{Contest: contest})
        return
    } else {
        apiClient.submitRunHandle(w, r, contest, problemId)
    }
}

func (apiClient *Api) contestHandle(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path == "/contest" || r.URL.Path == "/contest/" {
        contests, err := apiClient.listContests(getAuthCookie(r))
        if err != nil {
            apiClient.write500Error(w, err)
        } else {
            apiClient.renderPage(w, "contestSelect.html", ContestSelectPage{Contests: contests})
        }
    } else {
        contestName := r.URL.Path[len("/contest/"):]
        index := strings.Index(contestName, "/")
        if index == -1 {
            http.Redirect(w, r, r.URL.Path+"/", 301)
            return
        }
        problemID := contestName[index + 1:]
        contestName = contestName[:index]
        apiClient.contestNameHandle(w, r, contestName, problemID)
    }
}

func (apiClient *Api) mainHandle(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        if apiClient.debug {
            apiClient.logger.Println("Not found path: " + r.URL.Path)
        }
        http.NotFound(w, r)
        return
    }
    key := getAuthCookie(r)
    if key == "" {
        http.Redirect(w, r, "/login", 301)
    } else {
        _, err := apiClient.getApiVersion(key)
        if err != nil {
            http.Redirect(w, r, "/login", 301)
        } else {
            http.Redirect(w, r, "/contest/", 301)
        }
    }
}

func (apiClient *Api) ListenAndServeAndHandleError(TLS bool, address, keyFile, certFile string) {
    if TLS && keyFile != "" && certFile != "" {
        err := http.ListenAndServeTLS(address, keyFile, certFile, nil)
        if err != nil {
            apiClient.logger.Panic(err)
        }
    } else if !TLS {
        err := http.ListenAndServe(address, nil)
        if err != nil {
            apiClient.logger.Panic(err)
        }
    }
}

func maxAgeHandler(maxAge int, handler http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r* http.Request) {
        w.Header().Add("Cache-Control", fmt.Sprintf("Max-Age: %d, Public, Must-Revalidate, Proxy-Revalidate", maxAge))
        handler.ServeHTTP(w, r)
    })
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
    http.HandleFunc("/contest/", client.contestHandle)
    http.Handle("/static/", maxAgeHandler(60 * 60 * 24, http.FileServer(http.Dir("."))))
    http.HandleFunc("/", client.mainHandle)
    address := ":80"
    if client.debug {
        address = ":8080"
    }
    certFile, _ := os.LookupEnv("CERT_FILE")
    keyFile, _ := os.LookupEnv("KEY_FILE")
    var wg sync.WaitGroup
    wg.Add(2)
    go client.ListenAndServeAndHandleError(false, address, "", "")
    go client.ListenAndServeAndHandleError(true, ":443", certFile, keyFile)
    wg.Wait()
}
