package main

import (
	"io"
	"time"
    "fmt"
    "strconv"
    "net/http"
    "html/template"
//     "strings"
    "regexp"
    "log"
    "crypto/sha512"
    "crypto/md5"
    "encoding/base64"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)


func handlerRoot(w http.ResponseWriter, r *http.Request) {
    t, _ := template.ParseFiles("html/root.html")
    t.Execute(w, nil)
    //fmt.Fprintf(w, "Hello andrei!") // send data to client side
}

func addUser(user, password string) {
	db, err := sql.Open("mysql", "root@/tfg?charset=utf8")
    checkErr(err)
    defer db.Close()

     //insert
    stmt, err := db.Prepare("INSERT userinfo SET email=?,password=?,created=?")
    checkErr(err)

    date := time.Now().String()

    res, err := stmt.Exec(user, password, date)
    checkErr(err)

    id, err := res.LastInsertId()
    checkErr(err)

    fmt.Println(id)

}

func getUser(email, password string) {
	db, err := sql.Open("mysql", "root@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("SELECT * FROM userinfo WHERE email=? and password=?")
	checkErr(err)

	fmt.Printf("Trying with %s:%s\n", email, password)
	rows, err := stmt.Query(email, password)
	checkErr(err)

	for rows.Next() {
        var email string
        var password string
        var created string
        err = rows.Scan(&email, &password, &created)
        checkErr(err)
        fmt.Println(email)
        fmt.Println(password)
        fmt.Println(created)
    }

}

func checkErr(err error) {
    if err != nil {
        log.Fatal(err)
    }
}

func validEmail(email string) bool {
	match , _ := regexp.MatchString(`^([\w\.\_]{2,10})@(\w{1,}).([a-z]{2,4})$`, email)
	return match
}

func validCSRF(token string) bool {
	if token == "" {
        	return false
	}
	//do later
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
    h := md5.New()
    io.WriteString(h, strconv.FormatInt(crutime, 10))
    token := fmt.Sprintf("%x", h.Sum(nil))
    t, _ := template.ParseFiles(path)
    t.Execute(w, token)
}

func getSha512(s string) string {
	hasher := sha512.New()
    hasher.Write([]byte(s))
    fmt.Printf("Going to sha: %s\n", s)
    return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func handlerCreateAccount(w http.ResponseWriter, r *http.Request) {
    if r.Method == "GET" {
    	giveFormTemplate("html/create_account.html", w)
    } else if r.Method == "POST" {
        r.ParseForm()
        
        if token := r.Form.Get("csrf"); !validCSRF(token){
        	fmt.Fprintf(w, "ANTI CSRF GOT SOMETHING")
        	return
        }        
        email := string(template.HTMLEscapeString(r.Form.Get("email")))
        if !validEmail(email) {
        	fmt.Fprintf(w, "Please input a valid email")
        	return
        }
        password := r.Form.Get("password")
        if !validPassword(password) {
        	fmt.Fprintln(w, "Given password does not comply with ",
        				"given password directives")
        	return
        }

		password = getSha512(r.Form.Get("password"))        

        addUser(email, password)

        
        fmt.Fprintln(w, "OK")
    }
}

func handlerLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		giveFormTemplate("html/login.html", w)
	} else if r.Method == "POST" {
		r.ParseForm()
		if token := r.Form.Get("csrf"); !validCSRF(token){
			fmt.Fprintf(w, "ANTI CSRF GOT SOMETHING")
			return
		}
		email := r.Form.Get("email")
		password := getSha512(r.Form.Get("password"))

		getUser(email, password)
	}
}

func main() {
    http.HandleFunc("/", handlerRoot) // set router
    http.HandleFunc("/create_account", handlerCreateAccount)
    http.HandleFunc("/login", handlerLogin)
    err := http.ListenAndServe(":9090", nil) // set listen port
    if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
}
