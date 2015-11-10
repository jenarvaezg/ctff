package main

import (
    "fmt"
    "net/http"
    "html/template"
    "strings"
    "log"
    "crypto/sha512"
    "encoding/base64"
)


func sayhelloName(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()  // parse arguments, you have to call this by yourself
    fmt.Println(r.Form)  // print form information in server side
    for k, v := range r.Form {
        fmt.Println("key:", k)
        fmt.Println("val:", strings.Join(v, ""))
    }
    fmt.Fprintf(w, "Hello andrei!") // send data to client side
}

func createAccount(w http.ResponseWriter, r *http.Request) {
    if r.Method == "GET" {
        t, _ := template.ParseFiles("html/create_account.gtpl")
        t.Execute(w, nil)
    } else {
        r.ParseForm()
        mail := string(template.HTMLEscapeString(r.Form.Get("mail")))
        hasher := sha512.New()
        hasher.Write([]byte(r.Form.Get("password")))
        password := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
        fmt.Fprintf(w, "%s\n", mail)
        fmt.Fprintf(w, "%s\n", password)
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
