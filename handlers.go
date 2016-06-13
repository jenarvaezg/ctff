package main

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

type user struct {
	Email    string
	Username string
	Created  string
	IsYou    bool
	Finished map[string]Challenge_link
	Score    int
}

var store = sessions.NewCookieStore([]byte("EEEEH"))

func challengeFromForm(form url.Values) (Challenge, error) {
	var err error
	c := Challenge{}
	fmt.Println(form)
	c.Title = form.Get("title")
	if c.Title == "" {
		return c, errors.New("No Title")
	}
	c.Description = template.HTML(form.Get("description"))
	if c.Title == "" {
		return c, errors.New("No Description")
	}
	c.Alias = form.Get("path")
	if c.Alias == "" {
		return c, errors.New("No path")
	}
	c.MaxScore, err = strconv.Atoi(form.Get("points"))
	if err != nil || c.MaxScore <= 0 || c.MaxScore > MaxChallengeScore {
		return c, errors.New("Wrong Points Format or Missing")
	}
	c.Category = form.Get("category")
	if c.Category == "" {
		return c, errors.New("No Category")
	}
	return c, nil
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
	return true
}

func checkLogged(w http.ResponseWriter, r *http.Request) *sessions.Session {
	session, _ := store.Get(r, "session")
	if session.Values["username"] == nil {
		http.Redirect(w, r, "/", http.StatusFound)
	}
	return session
}

func checkNotLogged(w http.ResponseWriter, r *http.Request) *sessions.Session {
	session, _ := store.Get(r, "session")
	if session.Values["username"] != nil {
		http.Redirect(w, r, "/", http.StatusFound)
	}
	return session
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

func getSha512Hex(s string) string {
	hasher := sha512.New()
	hasher.Write([]byte(s))
	return hex.EncodeToString(hasher.Sum(nil))
}

func getSha512B64(s string) string {
	hasher := sha512.New()
	hasher.Write([]byte(s))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func handlerRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		fmt.Println(r.URL.Path)
		http.NotFound(w, r)
		return
	}
	session, _ := store.Get(r, "session")
	t, _ := template.ParseFiles("static/root.html")
	t.Execute(w, session.Values["username"] != nil)
}

func handlerCreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		giveFormTemplate("static/create_account.html", w)
	} else if r.Method == "POST" {
		r.ParseForm()
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
		password = getSha512B64(r.Form.Get("password"))
		if err := AddUser(email, password, username); err != nil {
			fmt.Fprintf(w, "Mail or username already in use or %s\n", err)
			return
		}
		http.Redirect(w, r, "/created", http.StatusFound)
	}
}

func handlerLogin(w http.ResponseWriter, r *http.Request) {
	session := checkNotLogged(w, r)
	if r.Method == "GET" {
		giveFormTemplate("static/login.html", w)
	} else if r.Method == "POST" {

		r.ParseForm()
		identifier := r.Form.Get("identifier")
		password := getSha512B64(r.Form.Get("password"))
		username := VerifyUser(identifier, password)

		if username == "" {
			fmt.Fprintf(w, "Wrong mail or password\n")
			return
		}
		session.Values["username"] = username
		session.Save(r, w)
		http.Redirect(w, r, "/user/"+username, http.StatusFound)
	}
}

func handlerUsers(w http.ResponseWriter, r *http.Request) {
	users := GetUsernames()
	t, _ := template.ParseFiles("static/users.html")
	t.Execute(w, users)
}

func handlerUser(w http.ResponseWriter, r *http.Request) {
	session := checkLogged(w, r)
	username := session.Values["username"]
	target := mux.Vars(r)["username"]
	user, err := GetUser(target)
	if err != nil {
		fmt.Fprintf(w, "User %s not found :(\n%s", target, err)
		return
	}
	if username == user.Username {
		user.IsYou = true
	}
	t, _ := template.ParseFiles("static/user.html")
	t.Execute(w, user)
}

func handlerCreated(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("static/created.html")
	t.Execute(w, nil)
}

func handlerLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	session.Options = &sessions.Options{MaxAge: -1, Path: "/"}
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func handlerChallenges(w http.ResponseWriter, r *http.Request) {
	checkLogged(w, r)
	challenges := GetChallengesLinks()
	t, _ := template.ParseFiles("static/challenges.html")
	t.Execute(w, challenges)
}

func handlerChallenge(w http.ResponseWriter, r *http.Request) {
	session := checkLogged(w, r)
	username := session.Values["username"]
	target := mux.Vars(r)["challenge_id"]
	user, _ := GetUser(username.(string))
	challenge, err := GetChallenge(target)
	if err != nil {
		fmt.Fprintf(w, "challenge %d not found :(\n", target)
		fmt.Fprintln(w, err)
		return
	}
	if UserFinishedChallenge(user.Email, target) {
		fmt.Fprintf(w, "You already finished this challenge!")
		return
	}
	if r.Method == "GET" {
		t, err := template.ParseFiles("static/challenge.html")
		if err != nil {
			fmt.Fprint(w, err)
			return
		}
		t.Execute(w, challenge)
	} else if r.Method == "POST" {
		r.ParseForm()
		if r.Form.Get("launch") != "" {
			challenge.LaunchText = challenge.Launch(user.Username)
			t, err := template.ParseFiles("static/challenge.html")
			if err != nil {
				fmt.Fprint(w, err)
				return
			}
			t.Execute(w, challenge)
		} else if r.Form.Get("stop") != "" {
			challenge.Stop(user.Username)
			challenge.LaunchText = ""
			t, err := template.ParseFiles("static/challenge.html")
			if err != nil {
				fmt.Fprint(w, err)
				return
			}
			t.Execute(w, challenge)
		} else {
			solution := r.Form.Get("solution")
			succesful := challenge.CheckSolution(solution, user.Username)
			AddAtempt(user.Email, challenge.UID, succesful, challenge.MaxScore) /*TODO HINTS AFFECT MAX SCORE */
			session, _ := store.Get(r, "challenge")
			if succesful {
				session.Values["challenge"] = challenge.UID //TODO CHANGE THIS TOO
				session.Save(r, w)
				UpdateScore(user.Email, challenge.MaxScore)
				http.Redirect(w, r, "/success", http.StatusFound)
			}
			fmt.Fprintf(w, "Wrong answer :(")
		}
	}
}

func handlerSuccess(w http.ResponseWriter, r *http.Request) {
	checkLogged(w, r)
	session, _ := store.Get(r, "challenge")
	UID, ok := session.Values["challenge"].(string)
	if !ok {
		http.Redirect(w, r, "/", http.StatusFound)
	}
	session.Options = &sessions.Options{MaxAge: -1, Path: "/"}
	session.Save(r, w)
	session = checkLogged(w, r)
	username := session.Values["username"].(string)
	challenge, err := GetChallenge(UID)
	if err != nil {
		fmt.Println(err)
	}
	defer challenge.Stop(username)
	t, _ := template.ParseFiles("static/success.html")
	t.Execute(w, challenge)
}

func handlerRanking(w http.ResponseWriter, r *http.Request) {
	checkLogged(w, r)
	ranking := GetRanking()
	t, _ := template.ParseFiles("static/ranking.html")
	t.Execute(w, ranking)
}

func handlerAddChallenge(w http.ResponseWriter, r *http.Request) {
	session := checkLogged(w, r)
	if r.Method == "GET" {
		t, _ := template.ParseFiles("static/add_challenge.html")
		t.Execute(w, nil)
	} else if r.Method == "POST" {
		r.ParseForm()
		challenge, err := challengeFromForm(r.Form)
		if err != nil {
			fmt.Fprintln(w, err)
			return
		}
		challenge.Creator = session.Values["username"].(string)
		challenge.AddToEnvironment()
		AddChallenge(challenge)
		t, err := template.ParseFiles("static/challenge_added.html")
		if err != nil {
			log.Fatal(err)
		}
		t.Execute(w, challenge)
	}
}

func handlerStatic(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, "..") {
		http.NotFound(w, r)
		return
	}
	vars := mux.Vars(r)
	_, ok := vars["challenge_id"]
	if !ok {
		log.Println(CTF2Path + r.URL.Path)
		http.ServeFile(w, r, CTF2Path+r.URL.Path)
		return
	}
	UID := vars["challenge_id"]
	resource, ok := vars["static_element"]
	c, err := GetChallenge(UID)
	if err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, ChallengesPath+"/"+c.Alias+"/static/"+resource)
}
