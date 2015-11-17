package main

import (
	"io"
	"time"
    "fmt"
    "strconv"
    "net/http"
    "html/template"
    "regexp"
    "strings"
    "log"
    "errors"
    "crypto/sha512"
    "encoding/base64"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "session"
    _ "session/providers/memory"
)

type user struct{
	Email string
	Username string
	Created string
	IsYou bool
}

func getUsernameSession(w http.ResponseWriter, r *http.Request) string {
	sess := globalSessions.SessionStart(w, r)
	username := sess.Get("username")
	if username == nil {
		return ""
	}

	return username.(string)
}

func handlerRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		fmt.Fprintf(w, "404 Not found")
		return
	}
	fmt.Println("ROOT??", r.URL.Path)
	t, _ := template.ParseFiles("html/root.html")
    t.Execute(w, getUsernameSession(w, r) != "")
}


func addUser(mail, password, username string) error {
	db, err := sql.Open("mysql", "root@/tfg?charset=utf8")
    checkErr(err)
    defer db.Close()

     //insert
    stmt, err := db.Prepare("INSERT userinfo SET email=?,password=?,created=?,username=?")
    checkErr(err)

    date := time.Now().String()

    _, err = stmt.Exec(mail, password, date, username)
    return err

}

func verifyUser(identifier, password string) (username string) {
	db, err := sql.Open("mysql", "root@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare(
		"SELECT username FROM userinfo WHERE (email=? or username=?) and password=?")
	checkErr(err)

	rows, err := stmt.Query(identifier, identifier, password)
	checkErr(err)
	if !rows.Next() {
		return ""
	}
	rows.Scan(&username)
    return 

}

func getUsers() (users []string) {
	db, err := sql.Open("mysql", "root@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare(
		"SELECT username FROM userinfo")
	checkErr(err)

	rows, err := stmt.Query()
	checkErr(err)

	for rows.Next() {
		var username string
		err = rows.Scan(&username)
		checkErr(err)
		users = append(users, username)
	}
    return
}

func findUser(username string) (u user, err error){

	db, err := sql.Open("mysql", "root@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare(
		"SELECT * FROM userinfo WHERE username=?")
	checkErr(err)

	rows, err := stmt.Query(username)
	

    if !rows.Next(){
    	err = errors.New("Not found")
    	return
    }

    var disposable string

    err = rows.Scan(&u.Email, &disposable, &u.Created, &u.Username)

    return 

}

func checkErr(err error) {
    if err != nil {
        log.Fatal(err)
    }
}

func validEmail(email string) bool {
	match , _ := regexp.MatchString(
		`^([\w\.\_]{2,10})@(\w{1,}).([a-z]{2,4})$`, email)
	return match
}

func validCSRF(r *http.Request) bool {
	token := r.Form.Get("csrf")
	cookie, err := r.Cookie("csrf")
	if token == ""  || err != nil || token != cookie.Value {
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

func handlerCreateAccount(w http.ResponseWriter, r *http.Request) {
    if r.Method == "GET" {
    	giveFormTemplate("html/create_account.html", w)
    } else if r.Method == "POST" {
        r.ParseForm()
        
        if !validCSRF(r){
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

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	globalSessions.SessionDestroy(w, r)
	handlerRoot(w, r)
}

func handlerLogin(w http.ResponseWriter, r *http.Request) {
	sess := globalSessions.SessionStart(w, r)
	if getUsernameSession(w, r) != "" {
		return
	}
	if r.Method == "GET" {
		giveFormTemplate("html/login.html", w)
	} else if r.Method == "POST" {
		r.ParseForm()
		if !validCSRF(r){
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
		sess.Set("username", username)
		http.Redirect(w, r, "/user/" + username, 301)
	}
}

func handlerUser(w http.ResponseWriter, r *http.Request) {
	username := getUsernameSession(w, r)
	if username == "" {
		fmt.Fprintf(w, "You must be logged in!\n")
		return
	}
	target := strings.Split(r.URL.Path[1:], "/")[1]

	if target == "" {
		users := getUsers()
		t, _ := template.ParseFiles("html/users.html")
		t.Execute(w, users)

	}else{
		user, err := findUser(target)
		if err != nil {
			fmt.Fprintf(w, "User %s not found :(\n", target)
			return
		}		
		if username == user.Username {
			user.IsYou = true
		}
		t, _ := template.ParseFiles("html/user.html")
    	t.Execute(w, user)	
    }
}

func handlerCreated(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("html/created.html")
    t.Execute(w, nil)
}

func handlerLogout(w http.ResponseWriter, r *http.Request) {
	fmt.Println("EEEEEh")
	globalSessions.SessionDestroy(w, r)
//	http.Redirect(w, r, "/", 301)
}

var globalSessions *session.Manager


func main() {
	var err error
	globalSessions, err = session.NewManager("memory", "gosessionid", 3600)
	
	if err != nil {
		fmt.Println(err)
		return
	}
	go globalSessions.GC()

    http.HandleFunc("/", handlerRoot) // set router
    http.HandleFunc("/create_account", handlerCreateAccount)
    http.HandleFunc("/login", handlerLogin)
    http.HandleFunc("/user*", handlerUser)
    http.HandleFunc("/created", handlerCreated)
    //http.HandleFunc("/logout/", handlerUser)
    err = http.ListenAndServe(":9090", nil) // set listen port


    if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
}
