package models

type Subject struct {
	ID   int
	Name string
}

type ClassSubject struct {
	ID          int
	ClassName   string
	SubjectName string
	TeacherName string
	ClassID     int
	SubjectID   int
	TeacherID   int
}
