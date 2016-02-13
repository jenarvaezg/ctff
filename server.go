package main

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

const (
	setup = iota
	run   = iota
)

type user struct {
	Email    string
	Username string
	Created  string
	IsYou    bool
	Finished map[string]challenge_link
	Score    int
}

type challenge struct {
	Title       string
	Description template.HTML
	Category    string
	Id          int
	MaxScore    int
	/*Solution     string
	SolutionType string*/
	Hints      []string
	Alias      string
	Creator    string
	LaunchText template.HTML
}

type challenge_link struct {
	Title             string
	Id                int
	Score             int
	NSuccess          int
	NTries            int
	SuccessPercentage float32
}

func (c challenge) launch(user string) template.HTML {
	out, err := exec.Command(ChallengesPath+"/"+c.Alias+
		"/rc/start_challenge", user).Output()
	if err != nil {
		return template.HTML(err.Error() + string(out))
	}
	return template.HTML(out)
}

func (c challenge) stop(user string) {
	_, err := exec.Command(ChallengesPath+"/"+c.Alias+
		"/rc/stop_challenge", user).Output()
	if err != nil {
		log.Println(template.HTML(err.Error()))
	}
}

func (c challenge) checkSolution(solution, user string) bool {
	out, err := exec.Command(ChallengesPath+"/"+c.Alias+
		"/rc/check_solution", solution, user).Output()
	fmt.Println(out, err)
	return err == nil
}

func (c challenge) addToEnvironment() error {
	path := ChallengesPath + "/" + c.Alias
	err := os.MkdirAll(path, os.FileMode(0755))
	if err != nil {
		fmt.Println(err)
		return err
	}
	f, _ := os.Create(path + "/rc/start_challenge")
	f.Chmod(0777)
	f.Close()
	f, _ = os.Create(path + "/rc/stop_challenge")
	f.Chmod(0777)
	f.Close()
	f, _ = os.Create(path + "/rc/check_solution")
	f.Chmod(0777)
	f.Close()
	return nil
}

func challengeFromForm(form url.Values) (challenge, error) {
	var err error
	c := challenge{}
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
	//do later
	return true
}

func checkLogged(w http.ResponseWriter, r *http.Request) *sessions.Session {
	session, _ := store.Get(r, "session")
	if session.Values["username"] == nil {
		http.Redirect(w, r, "/", 301)
	}
	return session
}

func checkNotLogged(w http.ResponseWriter, r *http.Request) *sessions.Session {
	session, _ := store.Get(r, "session")
	if session.Values["username"] != nil {
		http.Redirect(w, r, "/", 301)
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
	t, _ := template.ParseFiles("static/root.html")
	t.Execute(w, session.Values["username"] != nil)
}

func handlerCreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		giveFormTemplate("static/create_account.html", w)
	} else if r.Method == "POST" {
		r.ParseForm()

		/*if !validCSRF(r) {
			fmt.Fprintf(w, "ANTI CSRF GOT SOMETHING")
			return
		}*/
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
	session := checkNotLogged(w, r)
	if r.Method == "GET" {
		giveFormTemplate("static/login.html", w)
	} else if r.Method == "POST" {

		r.ParseForm()
		/*if !validCSRF(r) {
			fmt.Fprintf(w, "ANTI CSRF GOT SOMETHING")
			return
		}*/
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

func handlerUsers(w http.ResponseWriter, r *http.Request) {
	users := getUsernames()
	t, _ := template.ParseFiles("static/users.html")
	t.Execute(w, users)
}

func handlerUser(w http.ResponseWriter, r *http.Request) {
	session := checkLogged(w, r)
	username := session.Values["username"]
	target := mux.Vars(r)["username"]
	user, err := getUser(target)
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
	http.Redirect(w, r, "/", 301)
}

func handlerChallenges(w http.ResponseWriter, r *http.Request) {
	checkLogged(w, r)
	challenges := getChallengesLinks()
	t, _ := template.ParseFiles("static/challenges.html")
	t.Execute(w, challenges)
}

func handlerChallenge(w http.ResponseWriter, r *http.Request) {
	session := checkLogged(w, r)
	username := session.Values["username"]
	target, err := strconv.Atoi(mux.Vars(r)["challenge_id"])
	user, _ := getUser(username.(string))
	challenge, err := getChallenge(target)
	if err != nil {
		fmt.Fprintf(w, "challenge %d not found :(\n", target)
		fmt.Fprintln(w, err)
		return
	}
	if userFinishedChallenge(user.Email, target) {
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
			challenge.LaunchText = challenge.launch(user.Username)
			t, err := template.ParseFiles("static/challenge.html")
			if err != nil {
				fmt.Fprint(w, err)
				return
			}
			t.Execute(w, challenge)
		} else if r.Form.Get("stop") != "" {
			challenge.stop(user.Username)
			challenge.LaunchText = ""
			t, err := template.ParseFiles("static/challenge.html")
			if err != nil {
				fmt.Fprint(w, err)
				return
			}
			t.Execute(w, challenge)
		} else {
			solution := r.Form.Get("solution")
			succesful := challenge.checkSolution(solution, user.Username)
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
}

func handlerSuccess(w http.ResponseWriter, r *http.Request) {
	checkLogged(w, r)
	session, _ := store.Get(r, "challenge")
	id, ok := session.Values["challenge"].(int)
	if !ok {
		fmt.Println("NOT OK")
		http.Redirect(w, r, "/", 301)
	}
	session.Options = &sessions.Options{MaxAge: -1, Path: "/"}
	session.Save(r, w)
	session = checkLogged(w, r)
	fmt.Println(session)
	username := session.Values["username"].(string)
	fmt.Println("OK")
	challenge, err := getChallenge(id)
	if err != nil {
		fmt.Println(err)
	}
	defer challenge.stop(username)
	t, _ := template.ParseFiles("static/success.html")
	t.Execute(w, challenge)
}

func handlerRanking(w http.ResponseWriter, r *http.Request) {
	checkLogged(w, r)
	ranking := getRanking()
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
		challenge.addToEnvironment()
		addChallenge(challenge)
		t, err := template.ParseFiles("static/challenge_added.html")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("EEEEEEEEEh")
		t.Execute(w, challenge)
	}
}

func handlerStatic(w http.ResponseWriter, r *http.Request) {
	stream, err := ioutil.ReadFile(r.URL.Path[1:])

	if err != nil {
		fmt.Println(err)
		fmt.Fprintf(w, "File %s does not exist\n", r.URL.Path[1:])
	}

	b := bytes.NewBuffer(stream)

	if _, err := b.WriteTo(w); err != nil { // <----- here!
		fmt.Fprintf(w, "%s", err)
	}
}

var store = sessions.NewCookieStore([]byte("EEEEH"))

func setupRouter(r *mux.Router) {
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
	r.HandleFunc("/add_challenge", handlerAddChallenge)
	r.HandleFunc("/static/{folder}/{element}", handlerStatic)

}

func addNewChallenges() {
	fileInfos, err := ioutil.ReadDir(ChallengesPath)
	if err != nil {
		log.Fatal(err)
	}
	aliases := getAllChallengeAliases()
	old_challenges := make(map[string]bool)
	for _, fileInfo := range fileInfos {
		old_challenges[fileInfo.Name()] = false
		for _, alias := range aliases {
			if fileInfo.Name() == alias {
				old_challenges[alias] = true
				break
			}
		}
	}
	var wg sync.WaitGroup
	for k, v := range old_challenges {
		if !v {
			wg.Add(1)
			go func(dirname string) {
				f, err := os.Open(dirname + "/info.json")
				if err != nil {
					fmt.Println(err)
					return
				}
				defer f.Close()
				fmt.Println("OK")
				wg.Done()
			}(ChallengesPath + "/" + k)
		}
	}
	wg.Wait()
}

func main() {
	var err error

	if err != nil {
		log.Fatal(err)
	}
	if mode := checkArgs(); mode == run {
		r := mux.NewRouter()
		setupRouter(r)
		log.Print("RUNNING")
		err = http.ListenAndServeTLS(":9090", "server.pem", "server.key", r)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else if mode == setup {
		addNewChallenges()
	}

}

func checkArgs() int {

	if len(os.Args) < 2 {
		usage()
	}
	if os.Args[1] == "setup" {
		return setup
	}
	if os.Args[1] == "run" {
		return run
	}
	return -1
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ctff [setup|run]")
	os.Exit(1)
}
