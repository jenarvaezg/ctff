package main

import (
	"crypto/sha512"
	"encoding/base64"	
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"html/template"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

type user struct {
	Email    string
	Username string
	Created  string
	IsYou    bool
	Finished map[string]challenge_link
	Score int
}

type challenge struct{
	Title string
	Description string
	Id int
	MaxScore int
	Solution string
	Hints []string
}

type challenge_link struct{
	Title string
	Id int
	Score int
}

func validEmail(email string) bool {
	match, _ := regexp.MatchString(
		`^([\w\.\_]{2,64})@(\w{1,}).([a-z]{2,4})$`, email)
	return match
}

func validCSRF(r *http.Request) bool {
	token := r.Form.Get("csrf")
	cookie, err := r.Cookie("csrf")
	if token == "" || err != nil || token != cookie.Value {
		return false
	}

	return true
}

func validPassword(password string) bool {

	if len(password) == 0 {
		return false
	}
	//do later
	return true
}


func giveFormTemplate(path string, w http.ResponseWriter) {
	crutime := time.Now().Unix()
	h := sha512.New()
	io.WriteString(h, strconv.FormatInt(crutime, 10))
	token := fmt.Sprintf("%x", h.Sum(nil))
	t, _ := template.ParseFiles(path)
	expiration := time.Now().Add(1 * time.Hour)
	cookie := http.Cookie{Name: "csrf", Value: token, Expires: expiration}
	http.SetCookie(w, &cookie)
	t.Execute(w, token)
}

func getSha512(s string) string {
	hasher := sha512.New()
	hasher.Write([]byte(s))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func handlerRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	session, _ := store.Get(r, "session")
	t, _ := template.ParseFiles("html/root.html")
	t.Execute(w, session.Values["username"] != nil)
}


func handlerCreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		giveFormTemplate("html/create_account.html", w)
	} else if r.Method == "POST" {
		r.ParseForm()

		if !validCSRF(r) {
			fmt.Fprintf(w, "ANTI CSRF GOT SOMETHING")
			return
		}
		email := string(template.HTMLEscapeString(r.Form.Get("email")))
		if !validEmail(email) {
			fmt.Fprintf(w, "Please input a valid email")
			return
		}
		username := string(template.HTMLEscapeString(r.Form.Get("username")))
		if username == "" {
			fmt.Fprintf(w, "Username can't be blank")
			return
		}
		password := r.Form.Get("password")
		if !validPassword(password) {
			fmt.Fprintln(w, "Given password does not comply with ",
				"given password directives")
			return
		}
		password = getSha512(r.Form.Get("password"))
		if err := addUser(email, password, username); err != nil {
			fmt.Fprintf(w, "Mail or username already in use or %s\n", err)
			return
		}
		http.Redirect(w, r, "/created", 301)
	}
}


func handlerLogin(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")

	if session.Values["username"] != nil {
		http.Redirect(w, r, "/", 301)
	}

	if r.Method == "GET" {
		giveFormTemplate("html/login.html", w)
	} else if r.Method == "POST" {
		r.ParseForm()
		if !validCSRF(r) {
			fmt.Fprintf(w, "ANTI CSRF GOT SOMETHING")
			return
		}
		identifier := r.Form.Get("identifier")
		password := getSha512(r.Form.Get("password"))
		username := verifyUser(identifier, password)

		if username == "" {
			fmt.Fprintf(w, "Wrong mail or password\n")
			return
		}
		session.Values["username"] = username
		session.Save(r, w)
		http.Redirect(w, r, "/user/"+username, 301)
	}
}

func handlerUsers(w http.ResponseWriter, r *http.Request){
	
	users := getUsernames()
	t, _ := template.ParseFiles("html/users.html")
	t.Execute(w, users)
}


func handlerUser(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	username := session.Values["username"]
	if username == nil {
		fmt.Fprintf(w, "You must be logged in!\n")
		return
	}
	target := mux.Vars(r)["username"]
	user, err := getUser(target)
	if err != nil {
		fmt.Fprintf(w, "User %s not found :(\n%s", target, err)
		return
	}
	if username == user.Username {
		user.IsYou = true
	}
	t, _ := template.ParseFiles("html/user.html")
	t.Execute(w, user)
}

func handlerCreated(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("html/created.html")
	t.Execute(w, nil)
}

func handlerLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	session.Options = &sessions.Options{MaxAge: -1, Path: "/"}
	session.Save(r, w)
	http.Redirect(w, r, "/", 301)
}

func handlerChallenges(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	username := session.Values["username"]
	if username == nil {
		fmt.Fprintf(w, "You must be logged in!\n")
		return
	}
	challenges := getChallengesLinks()
	t, _ := template.ParseFiles("html/challenges.html")
	t.Execute(w, challenges)
}

func handlerChallenge(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	username := session.Values["username"]
	if username == nil {
		fmt.Fprintf(w, "You must be logged in!\n")
		return
	}
	target, err:= strconv.Atoi(mux.Vars(r)["challenge_id"])
	user, _ := getUser(username.(string))
	challenge, err := getChallenge(target)
	if err != nil {
		fmt.Fprintf(w, "challenge %d not found :(\n", target)
		return
	}
	if userFinishedChallenge(user.Email, target) {
		fmt.Fprintf(w, "You already finished this challenge!")
		return
	}
	if r.Method == "GET" {
		t, _ := template.ParseFiles("html/challenge.html")
		t.Execute(w, challenge)
	} else if r.Method == "POST" {
		r.ParseForm()
		solution := r.Form.Get("solution")
		succesful := solution == challenge.Solution
		addAtempt(user.Email, challenge.Id, succesful, challenge.MaxScore) /*TODO HINTS AFFECT MAX SCORE */
		session, _ := store.Get(r, "challenge")
		if succesful {
			session.Values["challenge"] = challenge.Id //TODO CHANGE THIS TOO
			session.Save(r, w)
			updateScore(user.Email, challenge.MaxScore)
			http.Redirect(w, r, "/success", 301)
		}
		fmt.Fprintf(w, "Wrong answer :(")
	}
}

func handlerSuccess(w http.ResponseWriter, r *http.Request){
	session, _ := store.Get(r, "challenge")
	if  session.Values["challenge"] == nil {
		http.Redirect(w, r, "/", 301)
		return
	} 

	challenge, _ := getChallenge(session.Values["challenge"].(int))
	session.Options = &sessions.Options{MaxAge: -1, Path: "/"}
	session.Save(r, w)
	t, _ := template.ParseFiles("html/success.html")
	t.Execute(w, challenge)
}

func handlerRanking(w http.ResponseWriter, r *http.Request){
	session, _ := store.Get(r, "session")
        if session.Values["username"] == nil {
                http.Redirect(w, r, "/", 301)
        }

	ranking := getRanking()
	t, _ := template.ParseFiles("html/ranking.html")
	t.Execute(w, ranking)
}

var store = sessions.NewCookieStore([]byte("EEEEH"))

func main() {
	var err error

	if err != nil {
		fmt.Println(err)
		return
	}

	r := mux.NewRouter()
	r.HandleFunc("/", handlerRoot)
	r.HandleFunc("/create_account", handlerCreateAccount)
	r.HandleFunc("/login", handlerLogin)
	r.HandleFunc("/user", handlerUsers)
	r.HandleFunc("/challenge", handlerChallenges)
	r.HandleFunc("/user/{username}", handlerUser)
	r.HandleFunc("/challenge/{challenge_id}", handlerChallenge)
	r.HandleFunc("/created", handlerCreated)
	r.HandleFunc("/logout", handlerLogout)
	r.HandleFunc("/success", handlerSuccess)
	r.HandleFunc("/ranking", handlerRanking)

	err = http.ListenAndServeTLS(":9090", "server.pem", "server.key", r) // set listen port

	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
