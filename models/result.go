package models

type Result struct {
	ID          int
	StudentID   int
	StudentName string
	SubjectID   int
	SubjectName string
	ClassID     int
	TermID      int
	CAScore     float64
	ExamScore   float64
	Total       float64
	Grade       string
	Position    int
}

type Term struct {
	ID        int
	Name      string
	Year      string
	IsCurrent bool
}
