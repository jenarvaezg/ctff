package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

const (
	setup = iota
	run
	install
	export
)

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
	r.HandleFunc("/challenge/{challenge_id}/static/{static_element}", handlerStatic)

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
	} else if mode == install {
		installChallenges(os.Args[2:])
	} else if mode == export {
		exportChallenges(os.Args[2:])
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
	if len(os.Args) < 3 {
		usage()
	}
	if os.Args[1] == "install" {
		return install
	}
	if os.Args[1] == "export" {
		return export
	}
	return -1
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ctff [setup|run|install ctff_file| export challenge_alias]")
	os.Exit(1)
}
