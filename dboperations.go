package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
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
	UID         string
	MaxScore    int
	Hints       []string
	Alias       string
	Creator     string
	LaunchText  template.HTML
}

func (c Challenge) Launch(user string) template.HTML {
	os.Setenv("CHALLENGE_ID", c.UID)
	os.Setenv("CHALLENGE_PATH", ChallengesPath+"/"+c.Alias)
	cmd := exec.Command(ChallengesPath+"/"+c.Alias+
		"/rc/start_challenge", user)
	if c.Title == "Telegram PoC" {
		err := cmd.Start()
		if err != nil {
			return template.HTML(err.Error())
		}
		return template.HTML("Challenge launched")
	}
	out, err := cmd.Output()
	if err != nil {
		return template.HTML(err.Error() + string(out))
	}
	return template.HTML(out)

}

func (c Challenge) Stop(user string) {
	os.Setenv("CHALLENGE_ID", c.UID)
	os.Setenv("CHALLENGE_PATH", ChallengesPath+"/"+c.Alias)
	_, err := exec.Command(ChallengesPath+"/"+c.Alias+
		"/rc/stop_challenge", user).Output()
	if err != nil {
		log.Println(template.HTML(err.Error()))
	}
}

func (c Challenge) CheckSolution(solution, user string) bool {
	os.Setenv("CHALLENGE_ID", c.UID)
	os.Setenv("CHALLENGE_PATH", ChallengesPath+"/"+c.Alias)
	out, err := exec.Command(ChallengesPath+"/"+c.Alias+
		"/rc/check_solution", solution, user).Output()
	fmt.Println(string(out), err)
	return err == nil
}

func (c Challenge) AddToEnvironment() error {
	filesToCreate := []string{"/rc/start_challenge", "/rc/stop_challenge", "/rc/check_solution"}
	path := ChallengesPath + "/" + c.Alias
	err := os.MkdirAll(path+"/rc", os.FileMode(0755))
	if err != nil {
		fmt.Println(err)
		return err
	}
	_ = os.MkdirAll(path+"/static", os.FileMode(0755))

	for _, fName := range filesToCreate {
		if _, err := os.Stat(path + fName); os.IsNotExist(err) {
			f, err := os.Create(path + fName)
			if err != nil {
				fmt.Println("AQUI!!!")
				fmt.Println(err)
				return err
			}
			f.Chmod(0777)
			f.Close()
		}
	}
	fmt.Println(c.UID)
	f, err := os.Create(path + "/your_ID_is_" + c.UID)
	f.Close()
	return nil
}

func (c Challenge) addChallenge() error {
	if c.Alias == "test" {
		return nil
	}
	if err := c.AddToEnvironment(); err != nil {
		return err
	}
	AddChallenge(c)
	fmt.Println("Challenge", c.Title, "succesfully added")
	return nil
}

type Challenge_link struct {
	Title string
	//Id                int
	Score             int
	NSuccess          int
	NTries            int
	SuccessPercentage float32
	UID               string
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
	scores, uids := GetSuccesfulAttempts(u.Email)
	db, _ = sql.Open("mysql", DBLoginString)
	u.Finished = make(map[string]Challenge_link)
	for i := 0; i < len(uids); i++ {
		stmt, err = db.Prepare("SELECT Title FROM challenges WHERE " +
			"UID=?")
		rows, err = stmt.Query(uids[i])
		checkErr(err)
		rows.Next()
		var title string
		rows.Scan(&title)
		u.Finished[title] = Challenge_link{Title: title, Score: scores[i],
			UID: uids[i]}
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
		"SELECT Title, MaxScore, Ntries, NSuccess, UID  FROM challenges")
	checkErr(err)

	rows, err := stmt.Query()
	checkErr(err)

	for rows.Next() {
		var c Challenge_link
		err = rows.Scan(&c.Title, &c.Score,
			&c.NTries, &c.NSuccess, &c.UID)
		checkErr(err)
		if c.NTries != 0 {
			c.SuccessPercentage = 100 * float32(c.NSuccess) / float32(c.NTries)
		}
		challenges = append(challenges, c)
	}
	return
}

func GetChallenge(UID string) (c Challenge, err error) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()
	stmt, err := db.Prepare(
		"SELECT Title, Description, MaxScore, Alias, Category, UID, Creator " +
			"FROM challenges WHERE UID=?")
	checkErr(err)

	rows, err := stmt.Query(UID)
	checkErr(err)
	if !rows.Next() {
		err = errors.New("Not found")
		return
	}
	var s string
	err = rows.Scan(&c.Title, &s, &c.MaxScore, &c.Alias, &c.Category, &c.UID, &c.Creator)
	c.Description = template.HTML(s)
	return
}

func ChallengeExists(UID string) bool {
	_, err := GetChallenge(UID)
	return err == nil
}

func AddChallenge(c Challenge) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()
	stmt, err := db.Prepare("INSERT challenges SET " +
		"Title=?, Description=?, MaxScore=?, Nhints=?, " +
		"Category=?, Creator=?, Alias=?, UID=?")
	checkErr(err)
	_, err = stmt.Exec(c.Title, string(c.Description), c.MaxScore, 0,
		c.Category, c.Creator, c.Alias, c.UID)
	checkErr(err)
}

func getChallengeByAlias(alias string) (c Challenge, err error) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()
	stmt, err := db.Prepare(
		"SELECT Title, Description, MaxScore, Alias, Category, UID, Creator " +
			"FROM challenges WHERE Alias=?")
	checkErr(err)

	rows, err := stmt.Query(alias)
	if !rows.Next() {
		err = errors.New("Not found")
		return
	}
	var s string
	err = rows.Scan(&c.Title, &s, &c.MaxScore, &c.Alias, &c.Category, &c.UID, &c.Creator)
	c.Description = template.HTML(s)
	return
}

func RemoveChallenge(UID string) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()
	stmt, err := db.Prepare("DELETE FROM challenges WHERE UID = ?")
	checkErr(err)
	_, err = stmt.Exec(UID)
	checkErr(err)
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

func AddAtempt(email string, UID string, succesful bool, score int) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("INSERT attempts SET " +
		"Date=?,u_email=?,UID=?,succesful=?,Score=?")
	checkErr(err)

	date := time.Now().Format("20060102")
	fmt.Println(date)

	_, err = stmt.Exec(date, email, UID, succesful, score)
	checkErr(err)
	stmt, _ = db.Prepare("SELECT NTries from challenges WHERE " +
		"UID=?")
	rows, _ := stmt.Query(UID)
	rows.Next()
	var ntries int
	rows.Scan(&ntries)
	ntries++
	stmt, _ = db.Prepare("UPDATE challenges SET NTries=? WHERE UID=?")
	stmt.Exec(ntries, UID)

	if succesful {
		stmt, _ = db.Prepare("SELECT NSuccess from challenges WHERE " +
			"UID=?")
		rows, _ := stmt.Query(UID)
		rows.Next()
		var nsuccess int
		rows.Scan(&nsuccess)
		nsuccess++
		stmt, err = db.Prepare("UPDATE challenges SET NSuccess=? WHERE UID=?")
		stmt.Exec(nsuccess, UID)
	}

}

func GetSuccesfulAttempts(email string) (scores []int, uids []string) {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("SELECT score, UID FROM attempts WHERE " +
		"succesful = true and u_email = ?")
	checkErr(err)
	rows, err := stmt.Query(email)
	checkErr(err)

	for rows.Next() {
		var score int
		var UID string
		err = rows.Scan(&score, &UID)
		scores = append(scores, score)
		uids = append(uids, UID)
	}
	return scores, uids
}

func UserFinishedChallenge(email string, UID string) bool {
	db, err := sql.Open("mysql", DBLoginString)
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("SELECT * FROM attempts WHERE " +
		"succesful = true and u_email = ? and UID=?")
	checkErr(err)
	rows, err := stmt.Query(email, UID)
	checkErr(err)

	return rows.Next()
}
