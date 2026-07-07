package models

type User struct {
	ID           int
	FullName     string
	Email        string
	PasswordHash string
	Role         string
}
