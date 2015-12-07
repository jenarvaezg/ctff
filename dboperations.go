package main

import(
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"errors"
	"time"
	"log"
)


func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}


/***********************
* User operations      *
***********************/

func addUser(mail, password, username string) error {
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	//insert
	stmt, err := db.Prepare("INSERT userinfo SET " +
		"email=?,password=?,created=?,username=?,score=?")
	checkErr(err)

	date := time.Now().String()

	_, err = stmt.Exec(mail, password, date, username, 0)
	return err

}

func verifyUser(identifier, password string) (username string) {
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
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

func getUsernames() (users []string) {
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
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



func getUser(username string) (u user, err error) {
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
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
	err = rows.Scan(&u.Email, &disposable, &u.Created, &u.Username,
			&u.Score)
	if(err != nil) {
		return
	}
	db.Close()
	scores, ids := getSuccesfulAttempts(u.Email)
	db, _ = sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
	u.Finished = make(map[string]challenge_link)
	for i := 0 ; i < len(ids); i++ {
		stmt, err = db.Prepare("SELECT Title FROM challenges WHERE " +
			"C_id=?")
		rows, err = stmt.Query(ids[i])
		checkErr(err)
		rows.Next()
		var title string
		rows.Scan(&title)
		u.Finished[title] = challenge_link{Title: title, Score: scores[i],
						Id: ids[i]}
	}
	return
}

func updateScore(email string, score int) {
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
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

func getRanking() (users []user) {
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()	
	stmt, err := db.Prepare("SELECT username, Score from userinfo " +
		"ORDER BY Score DESC")
	checkErr(err)
	rows, err := stmt.Query()
	for rows.Next() {
		var u user
		err = rows.Scan(&u.Username, &u.Score)
		checkErr(err)
		users = append(users, u)
	}
	return

}


/***********************
* Challenge operations *
***********************/

func getChallengesLinks() (challenges []challenge_link) {
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare(
		"SELECT Title, C_Id, MaxScore, Ntries, NSuccess  FROM challenges")
	checkErr(err)

	rows, err := stmt.Query()
	checkErr(err)

	for rows.Next() {
		var c challenge_link
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




func getChallenge(id int) (c challenge, err error){
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()
	stmt, err := db.Prepare(
		"SELECT Title, Description, MaxScore, Solution, C_Id " +
		"FROM challenges WHERE C_Id=?")
	checkErr(err)

	rows, err := stmt.Query(id)
	if !rows.Next() {
		err = errors.New("Not found")
		return
	}
	err = rows.Scan(&c.Title, &c.Description, &c.MaxScore,
					&c.Solution, &c.Id)
	return
}


/***********************
* Attempts operations  *
***********************/

func addAtempt(email string, c_id int, succesful bool, score int) {
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("INSERT attempts SET " +
		"Date=?,u_email=?,C_Id=?,succesful=?,Score=?")
	checkErr(err)

	date := time.Now().String()

	_, _ = stmt.Exec(date, email, c_id, succesful, score)
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


func getSuccesfulAttempts(email string) (scores []int, ids []int) {
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("SELECT score, C_Id FROM attempts WHERE " +
				"succesful = true and u_email = ?");
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

func userFinishedChallenge(email string, c_id int) bool {
	db, err := sql.Open("mysql", "tfg:passwordtfg@/tfg?charset=utf8")
	checkErr(err)
	defer db.Close()

	stmt, err := db.Prepare("SELECT * FROM attempts WHERE " +
				"succesful = true and u_email = ? and C_Id=?");
	checkErr(err)
	rows, err := stmt.Query(email, c_id)
	checkErr(err)
	
	return rows.Next()
}
