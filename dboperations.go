package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type User struct {
	Email    string
	Username string
	Created  string
	IsYou    bool
	Finished map[string]Challenge_link
	Score    int
}

type Challenge struct {
	Title       string
	Description template.HTML
	Category    string
	Id          int
	MaxScore    int
	Hints       []string
	Alias       string
	Creator     string
	LaunchText  template.HTML
}

func (c Challenge) Launch(user string) template.HTML {
	out, err := exec.Command(ChallengesPath+"/"+c.Alias+
		"/rc/start_challenge", user).Output()
	if err != nil {
		return template.HTML(err.Error() + string(out))
	}
	return template.HTML(out)
}

func (c Challenge) Stop(user string) {
	_, err := exec.Command(ChallengesPath+"/"+c.Alias+
		"/rc/stop_challenge", user).Output()
	if err != nil {
		log.Println(template.HTML(err.Error()))
	}
}

func (c Challenge) CheckSolution(solution, user string) bool {
	out, err := exec.Command(ChallengesPath+"/"+c.Alias+
		"/rc/check_solution", solution, user).Output()
	fmt.Println(out, err)
	return err == nil
}

func (c Challenge) AddToEnvironment() error {
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
	f, err = os.Create(path + "/your_ID_is_" + strconv.Itoa(c.Id))
	f.Close()
	return nil
}

type Challenge_link struct {
	Title             string
	Id                int
	Score             int
	NSuccess          int
	NTries            int
	SuccessPercentage float32
}

/***********************
* User operations      *
***********************/

func AddUser(mail, password, username string) error {
	db, err := sql.Open("mysql", DBLoginString)
	//insert
	stmt, err := db.Prepare("INSERT userinfo SET " +
		"email=?,password=?,created=?,username=?,score=?")
	checkErr(err)
	date := time.Now().Format("20060102")
	_, err = stmt.Exec(mail, password, date, username, 0)
	return err

}

func VerifyUser(identifier, password string) (username string) {
	db, err := sql.Open("mysql", DBLoginString)
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

func GetUsernames() (users []string) {
	db, err := sql.Open("mysql", DBLoginString)
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

func GetUser(username string) (u User, err error) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()
	stmt, err := db.Prepare("SELECT * FROM userinfo WHERE username=?")
	checkErr(err)
	rows, err := stmt.Query(username)
	if !rows.Next() {
		err = errors.New("Not found")
		return
	}
	var disposable string
	err = rows.Scan(&u.Email, &disposable, &u.Created, &u.Username,
		&u.Score)
	if err != nil {
		return
	}
	db.Close()
	scores, ids := GetSuccesfulAttempts(u.Email)
	db, _ = sql.Open("mysql", DBLoginString)
	u.Finished = make(map[string]Challenge_link)
	for i := 0; i < len(ids); i++ {
		stmt, err = db.Prepare("SELECT Title FROM challenges WHERE " +
			"C_id=?")
		rows, err = stmt.Query(ids[i])
		checkErr(err)
		rows.Next()
		var title string
		rows.Scan(&title)
		u.Finished[title] = Challenge_link{Title: title, Score: scores[i],
			Id: ids[i]}
	}
	return
}

func UpdateScore(email string, score int) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("SELECT Score from userinfo WHERE " +
		"email=?")
	checkErr(err)

	rows, err := stmt.Query(email)
	rows.Next()
	var prevScore int
	rows.Scan(&prevScore)

	score += prevScore

	stmt, err = db.Prepare("UPDATE userinfo SET Score=? WHERE email=?")
	checkErr(err)
	_, _ = stmt.Exec(score, email)

}

func GetRanking() (users []User) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()
	stmt, err := db.Prepare("SELECT username, Score from userinfo " +
		"ORDER BY Score DESC")
	checkErr(err)
	rows, err := stmt.Query()
	for rows.Next() {
		var u User
		err = rows.Scan(&u.Username, &u.Score)
		checkErr(err)
		users = append(users, u)
	}
	return

}

/***********************
* Challenge operations *
***********************/

func GetChallengesLinks() (challenges []Challenge_link) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare(
		"SELECT Title, C_Id, MaxScore, Ntries, NSuccess  FROM challenges")
	checkErr(err)

	rows, err := stmt.Query()
	checkErr(err)

	for rows.Next() {
		var c Challenge_link
		err = rows.Scan(&c.Title, &c.Id, &c.Score,
			&c.NTries, &c.NSuccess)
		checkErr(err)
		if c.NTries != 0 {
			c.SuccessPercentage = 100 * float32(c.NSuccess) / float32(c.NTries)
		}
		challenges = append(challenges, c)
	}
	return
}

func GetChallenge(id int) (c Challenge, err error) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()
	stmt, err := db.Prepare(
		"SELECT Title, Description, MaxScore, C_Id, Alias, Category " +
			"FROM challenges WHERE C_Id=?")
	checkErr(err)

	rows, err := stmt.Query(id)
	if !rows.Next() {
		err = errors.New("Not found")
		return
	}
	var s string
	err = rows.Scan(&c.Title, &s, &c.MaxScore, &c.Id, &c.Alias, &c.Category)
	c.Description = template.HTML(s)
	return
}

func AddChallenge(c Challenge) int {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()
	stmt, err := db.Prepare("INSERT challenges SET " +
		"Title=?, Description=?, MaxScore=?, Nhints=?, " +
		"Category=?, Creator=?, Alias=?")
	checkErr(err)
	_, err = stmt.Exec(c.Title, string(c.Description), c.MaxScore, 0,
		c.Category, c.Creator, c.Alias)
	checkErr(err)
	stmt, err = db.Prepare("SELECT C_Id FROM challenges WHERE Alias =?")
	checkErr(err)
	rows, err := stmt.Query(c.Alias)
	rows.Scan(&c.Id)
	return c.Id
}

func GetAllChallengeAliases() (aliases []string) {

	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()
	stmt, err := db.Prepare("SELECT alias from challenges")
	checkErr(err)
	rows, err := stmt.Query()
	for rows.Next() {
		var s string
		err = rows.Scan(&s)
		checkErr(err)
		aliases = append(aliases, s)
	}
	return

}

/***********************
* Attempts operations  *
***********************/

func AddAtempt(email string, c_id int, succesful bool, score int) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("INSERT attempts SET " +
		"Date=?,u_email=?,C_Id=?,succesful=?,Score=?")
	checkErr(err)

	date := time.Now().Format("20060102")
	fmt.Println(date)

	_, err = stmt.Exec(date, email, c_id, succesful, score)
	checkErr(err)
	stmt, _ = db.Prepare("SELECT NTries from challenges WHERE " +
		"c_id=?")
	rows, _ := stmt.Query(c_id)
	rows.Next()
	var ntries int
	rows.Scan(&ntries)
	ntries++
	stmt, _ = db.Prepare("UPDATE challenges SET NTries=? WHERE C_Id=?")
	stmt.Exec(ntries, c_id)

	if succesful {
		stmt, _ = db.Prepare("SELECT NSuccess from challenges WHERE " +
			"c_id=?")
		rows, _ := stmt.Query(c_id)
		rows.Next()
		var nsuccess int
		rows.Scan(&nsuccess)
		nsuccess++
		stmt, err = db.Prepare("UPDATE challenges SET NSuccess=? WHERE C_Id=?")
		stmt.Exec(nsuccess, c_id)
	}

}

func GetSuccesfulAttempts(email string) (scores []int, ids []int) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("SELECT score, C_Id FROM attempts WHERE " +
		"succesful = true and u_email = ?")
	checkErr(err)
	rows, err := stmt.Query(email)
	checkErr(err)

	for rows.Next() {
		var score, c_id int
		err = rows.Scan(&score, &c_id)
		scores = append(scores, score)
		ids = append(ids, c_id)
	}
	return scores, ids
}

func UserFinishedChallenge(email string, c_id int) bool {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("SELECT * FROM attempts WHERE " +
		"succesful = true and u_email = ? and C_Id=?")
	checkErr(err)
	rows, err := stmt.Query(email, c_id)
	checkErr(err)

	return rows.Next()
}
