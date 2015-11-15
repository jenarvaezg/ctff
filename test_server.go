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
)

type user struct{
	Email string
	Username string
	Created string
	IsYou bool
}



func handlerRoot(w http.ResponseWriter, r *http.Request) {
    t, _ := template.ParseFiles("html/root.html")
    t.Execute(w, nil)
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

func verifyUser(identifier, password string) (u user, err error) {
	db, err := sql.Open("mysql", "root@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare(
		"SELECT * FROM userinfo WHERE (email=? or username=?) and password=?")
	checkErr(err)

	rows, err := stmt.Query(identifier, identifier, password)
	checkErr(err)

	if !rows.Next() {
		return
	}

	var disposable string

	err = rows.Scan(&u.Email, &disposable, &u.Created, &u.Username)

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

func handlerLogin(w http.ResponseWriter, r *http.Request) {
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

		user, err := verifyUser(identifier, password)

		if err != nil {
			fmt.Fprintf(w, "Wrong mail or password, %s\n", err)
			return
		}
		expiration := time.Now().Add(7 * 24 * time.Hour)
		cookie := http.Cookie{Name: "session", Value: user.Username, Expires: expiration,
							HttpOnly: true}
		http.SetCookie(w, &cookie)
		http.Redirect(w, r, "/user/" + user.Username, 301)

	}
}

func handlerUser(w http.ResponseWriter, r *http.Request) {
	username := strings.Split(r.URL.Path[1:], "/")[1]
	if username == "" {
		fmt.Fprintf(w, "User directory goes here\n")
		return
	}
	user, err := findUser(username)
	if err != nil {
		fmt.Fprintf(w, "User %s not found :(\n", username)
		return
	}
	

	cookie, err := r.Cookie("session")
	if err != nil {
		fmt.Fprintf(w, "You must be logged in!\n")
		return
	}
	if cookie.Value == user.Username {
		user.IsYou = true
	}
	t, _ := template.ParseFiles("html/user.html")
    t.Execute(w, user)	

}

func handlerCreated(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("html/created.html")
    t.Execute(w, nil)
}

func main() {
    http.HandleFunc("/", handlerRoot) // set router
    http.HandleFunc("/create_account", handlerCreateAccount)
    http.HandleFunc("/login", handlerLogin)
    http.HandleFunc("/user/", handlerUser)
    http.HandleFunc("/created", handlerCreated)
    err := http.ListenAndServe(":9090", nil) // set listen port
    if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
}
