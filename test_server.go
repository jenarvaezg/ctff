package main

import (
	"io"
	"time"
    "fmt"
    "strconv"
    "net/http"
    "html/template"
//     "strings"
    "log"
    "crypto/sha512"
    "crypto/md5"
    "encoding/base64"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)


func sayhelloName(w http.ResponseWriter, r *http.Request) {
    t, _ := template.ParseFiles("html/root.gtpl")
    t.Execute(w, nil)
    //fmt.Fprintf(w, "Hello andrei!") // send data to client side
}

func createAccount(w http.ResponseWriter, r *http.Request) {
    if r.Method == "GET" {
    	crutime := time.Now().Unix()
    	h := md5.New()
    	io.WriteString(h, strconv.FormatInt(crutime, 10))
    	token := fmt.Sprintf("%x", h.Sum(nil))
        t, _ := template.ParseFiles("html/create_account.gtpl")
        t.Execute(w, token)
    } else {
        r.ParseForm()
        token := r.Form.Get("csrf")
        if token != "" {
        	//check later
        } else {
        	fmt.Fprintf(w, "No CSRF Token\n")
        	return
        }
        if len(r.Form.Get("email")) == 0 || len(r.Form.Get("password")) == 0 {
        	fmt.Fprintf(w, "Email and password can't be empty\n")
        	return
        }
        email := string(template.HTMLEscapeString(r.Form.Get("email")))
        fmt.Println(email)
        hasher := sha512.New()
        hasher.Write([]byte(r.Form.Get("password")))
        password := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
        fmt.Println(password)
        fmt.Fprintln(w, "OK")
    }
}

func main() {
    http.HandleFunc("/", sayhelloName) // set router
    http.HandleFunc("/create_account", createAccount)
    err := http.ListenAndServe(":9090", nil) // set listen port
    if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
}
