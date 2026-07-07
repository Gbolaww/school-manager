package handlers

import (
	"fmt"
	"net/http"
	"text/template"

	"school-manager/database"
	"school-manager/models"

	"github.com/jung-kurt/gofpdf"
)

func ShowReportCards(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	classRows, err := database.DB.Query("SELECT id, name FROM classes ORDER BY name ASC")
	if err != nil {
		http.Error(w, "Failed to load classes", http.StatusInternalServerError)
		return
	}
	defer classRows.Close()

	var classes []models.Class
	for classRows.Next() {
		var c models.Class
		classRows.Scan(&c.ID, &c.Name)
		classes = append(classes, c)
	}

	selectedClassID := r.URL.Query().Get("class_id")
	var students []models.Student
	var selectedClass string

	if selectedClassID != "" {
		database.DB.QueryRow("SELECT name FROM classes WHERE id = $1", selectedClassID).Scan(&selectedClass)

		studentRows, err := database.DB.Query(`
			SELECT id, full_name, admission_number FROM students
			WHERE class_id = $1 ORDER BY full_name ASC
		`, selectedClassID)
		if err == nil {
			defer studentRows.Close()
			for studentRows.Next() {
				var s models.Student
				studentRows.Scan(&s.ID, &s.FullName, &s.AdmissionNumber)
				students = append(students, s)
			}
		}
	}

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/reportcards.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":           "Report cards",
		"Page":            "reportcards",
		"UserName":        session.Values["user_name"],
		"UserInitials":    getInitials(session.Values["user_name"].(string)),
		"Role":            session.Values["user_role"],
		"Term":            "First term · 2025/2026",
		"Classes":         classes,
		"Students":        students,
		"SelectedClassID": selectedClassID,
		"SelectedClass":   selectedClass,
	})
}

func GenerateReportCard(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	studentID := r.URL.Query().Get("student_id")
	if studentID == "" {
		http.Redirect(w, r, "/reportcards", http.StatusSeeOther)
		return
	}

	var student models.Student
	var className string
	database.DB.QueryRow(`
		SELECT s.id, s.full_name, s.admission_number, COALESCE(c.name, '')
		FROM students s
		LEFT JOIN classes c ON s.class_id = c.id
		WHERE s.id = $1
	`, studentID).Scan(&student.ID, &student.FullName, &student.AdmissionNumber, &className)

	var termID int
	var termName, termYear string
	database.DB.QueryRow("SELECT id, name, year FROM terms WHERE is_current = TRUE LIMIT 1").Scan(&termID, &termName, &termYear)

	resultRows, err := database.DB.Query(`
		SELECT s.name, r.ca_score, r.exam_score, r.total, r.grade
		FROM results r
		JOIN subjects s ON r.subject_id = s.id
		WHERE r.student_id = $1 AND r.term_id = $2
		ORDER BY s.name ASC
	`, studentID, termID)
	if err != nil {
		http.Error(w, "Failed to load results", http.StatusInternalServerError)
		return
	}
	defer resultRows.Close()

	type SubjectResult struct {
		Subject string
		CA      float64
		Exam    float64
		Total   float64
		Grade   string
	}

	var results []SubjectResult
	var grandTotal float64
	for resultRows.Next() {
		var res SubjectResult
		resultRows.Scan(&res.Subject, &res.CA, &res.Exam, &res.Total, &res.Grade)
		results = append(results, res)
		grandTotal += res.Total
	}

	var average float64
	if len(results) > 0 {
		average = grandTotal / float64(len(results))
	}

	overallGrade := gradeFromTotal(average)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFillColor(26, 111, 191)
	pdf.Rect(0, 0, 210, 35, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(10, 8)
	pdf.Cell(190, 10, "School Manager")

	pdf.SetFont("Arial", "", 11)
	pdf.SetXY(10, 20)
	pdf.Cell(190, 8, "Student Report Card")

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(10, 42)
	pdf.Cell(60, 6, fmt.Sprintf("Student: %s", student.FullName))
	pdf.SetXY(10, 50)
	pdf.Cell(60, 6, fmt.Sprintf("Admission No: %s", student.AdmissionNumber))
	pdf.SetXY(110, 42)
	pdf.Cell(60, 6, fmt.Sprintf("Class: %s", className))
	pdf.SetXY(110, 50)
	pdf.Cell(60, 6, fmt.Sprintf("Term: %s term, %s", termName, termYear))

	pdf.SetXY(10, 62)
	pdf.SetFillColor(240, 246, 255)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(80, 8, "Subject", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, "CA (40)", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Exam (60)", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Total", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Grade", "1", 1, "C", true, 0, "")

	pdf.SetFont("Arial", "", 10)
	pdf.SetFillColor(255, 255, 255)

	for i, res := range results {
		if i%2 == 0 {
			pdf.SetFillColor(248, 252, 255)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.CellFormat(80, 7, res.Subject, "1", 0, "L", true, 0, "")
		pdf.CellFormat(25, 7, fmt.Sprintf("%.1f", res.CA), "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 7, fmt.Sprintf("%.1f", res.Exam), "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 7, fmt.Sprintf("%.1f", res.Total), "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 7, res.Grade, "1", 1, "C", true, 0, "")
	}

	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(26, 111, 191)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(80, 8, "Average", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, "", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, fmt.Sprintf("%.1f", average), "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, overallGrade, "1", 1, "C", true, 0, "")

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(10, pdf.GetY()+10)
	pdf.Cell(190, 6, "Grading: A = 70-100  |  B = 60-69  |  C = 50-59  |  D = 45-49  |  E = 40-44  |  F = Below 40")

	pdf.SetXY(10, pdf.GetY()+16)
	pdf.Line(10, pdf.GetY(), 80, pdf.GetY())
	pdf.SetXY(10, pdf.GetY()+2)
	pdf.Cell(70, 6, "Class Teacher's Signature")

	pdf.SetXY(130, pdf.GetY()-2)
	pdf.Line(130, pdf.GetY(), 200, pdf.GetY())
	pdf.SetXY(130, pdf.GetY()+2)
	pdf.Cell(70, 6, "Principal's Signature")

	filename := fmt.Sprintf("reportcard_%s.pdf", student.AdmissionNumber)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	pdf.Output(w)
}
