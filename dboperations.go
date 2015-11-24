package tfg

import(
	"database/sql"
	"errors"
	_ "github.com/go-sql-driver/mysql"
	"time"
)

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

func findUser(username string) (u user, err error) {

	db, err := sql.Open("mysql", "root@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare(
		"SELECT * FROM userinfo WHERE username=?")
	checkErr(err)

	rows, err := stmt.Query(username)

	if !rows.Next() {
		err = errors.New("Not found")
		return
	}

	var disposable string

	err = rows.Scan(&u.Email, &disposable, &u.Created, &u.Username)

	return

}