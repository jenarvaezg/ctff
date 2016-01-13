package main

type FormCSRF struct {
	token string
}

type CreateAccountForm struct {
	FormCSRF
	Email    string
	Username string
	Password string
}

type AddChallengeForm struct {
	FormCSRF
	Title        string
	Description  string
	Points       int
	SolutionType string
}
