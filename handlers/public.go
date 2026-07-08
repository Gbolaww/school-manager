package handlers

import (
	"net/http"
	"strings"
	"text/template"

	"school-manager/database"
)

type PublicSubjectResult struct {
	Subject string
	CA      float64
	Exam    float64
	Total   float64
	Grade   string
}

func ShowCheckResults(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/check_results.html"))
	tmpl.Execute(w, nil)
}

func HandleCheckResults(w http.ResponseWriter, r *http.Request) {
	admissionNumber := strings.TrimSpace(r.FormValue("admission_number"))

	tmpl := template.Must(template.ParseFiles("templates/check_results.html"))

	if admissionNumber == "" {
		tmpl.Execute(w, map[string]interface{}{
			"Error": "Please enter an admission number.",
		})
		return
	}

	var studentID int
	var studentName, className string
	err := database.DB.QueryRow(`
		SELECT s.id, s.full_name, COALESCE(c.name, '')
		FROM students s
		LEFT JOIN classes c ON s.class_id = c.id
		WHERE LOWER(s.admission_number) = LOWER($1)
	`, admissionNumber).Scan(&studentID, &studentName, &className)

	if err != nil {
		tmpl.Execute(w, map[string]interface{}{
			"Error":           "No student found with that admission number. Please check and try again.",
			"AdmissionNumber": admissionNumber,
		})
		return
	}

	var termID int
	var termName, termYear string
	database.DB.QueryRow("SELECT id, name, year FROM terms WHERE is_current = TRUE LIMIT 1").Scan(&termID, &termName, &termYear)

	resultRows, err := database.DB.Query(`
		SELECT sub.name, r.ca_score, r.exam_score, r.total, r.grade
		FROM results r
		JOIN subjects sub ON r.subject_id = sub.id
		WHERE r.student_id = $1 AND r.term_id = $2
		ORDER BY sub.name ASC
	`, studentID, termID)

	var results []PublicSubjectResult
	var grandTotal float64
	if err == nil {
		defer resultRows.Close()
		for resultRows.Next() {
			var res PublicSubjectResult
			resultRows.Scan(&res.Subject, &res.CA, &res.Exam, &res.Total, &res.Grade)
			results = append(results, res)
			grandTotal += res.Total
		}
	}

	var average float64
	if len(results) > 0 {
		average = grandTotal / float64(len(results))
	}
	overallGrade := ""
	if len(results) > 0 {
		overallGrade = gradeFromTotal(average)
	}

	tmpl.Execute(w, map[string]interface{}{
		"Found":           true,
		"StudentID":       studentID,
		"StudentName":     studentName,
		"ClassName":       className,
		"TermLabel":       termName + " term, " + termYear,
		"Results":         results,
		"Average":         average,
		"OverallGrade":    overallGrade,
		"AdmissionNumber": admissionNumber,
	})
}
