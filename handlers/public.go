package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"school-manager/database"
	"school-manager/models"

	"github.com/jung-kurt/gofpdf"
)

type PublicSubjectResult struct {
	Subject string
	CA      float64
	Exam    float64
	Total   float64
	Grade   string
}

func loadAllTerms() []models.Term {
	rows, err := database.DB.Query("SELECT id, name, year, is_current FROM terms ORDER BY id DESC")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var terms []models.Term
	for rows.Next() {
		var t models.Term
		rows.Scan(&t.ID, &t.Name, &t.Year, &t.IsCurrent)
		terms = append(terms, t)
	}
	return terms
}

// normalizePhone strips spaces, dashes, and a leading +234/234 so
// "+234 801 234 5678", "0801-234-5678", and "8012345678" all match.
func normalizePhone(phone string) string {
	p := strings.ReplaceAll(phone, " ", "")
	p = strings.ReplaceAll(p, "-", "")
	p = strings.TrimPrefix(p, "+234")
	p = strings.TrimPrefix(p, "234")
	p = strings.TrimPrefix(p, "0")
	return p
}

func ShowCheckResults(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/check_results.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Terms": loadAllTerms(),
	})
}

func HandleCheckResults(w http.ResponseWriter, r *http.Request) {
	admissionNumber := strings.TrimSpace(r.FormValue("admission_number"))
	parentPhone := strings.TrimSpace(r.FormValue("parent_phone"))
	selectedTermID := r.FormValue("term_id")

	tmpl := template.Must(template.ParseFiles("templates/check_results.html"))
	terms := loadAllTerms()

	if admissionNumber == "" || parentPhone == "" {
		tmpl.Execute(w, map[string]interface{}{
			"Error": "Please enter both the admission number and parent's phone number.",
			"Terms": terms,
		})
		return
	}

	var studentID int
	var studentName, className, storedPhone string
	err := database.DB.QueryRow(`
		SELECT s.id, s.full_name, COALESCE(c.name, ''), COALESCE(s.parent_phone, '')
		FROM students s
		LEFT JOIN classes c ON s.class_id = c.id
		WHERE LOWER(s.admission_number) = LOWER($1)
	`, admissionNumber).Scan(&studentID, &studentName, &className, &storedPhone)

	// Same generic error for "not found" and "wrong phone" — don't reveal
	// which part was wrong, so a wrong-guesser can't narrow it down.
	invalidMsg := "We couldn't find a match for that admission number and phone number. Please check both and try again."

	if err != nil || normalizePhone(storedPhone) == "" || normalizePhone(storedPhone) != normalizePhone(parentPhone) {
		tmpl.Execute(w, map[string]interface{}{
			"Error":           invalidMsg,
			"AdmissionNumber": admissionNumber,
			"Terms":           terms,
		})
		return
	}

	var termID int
	var termName, termYear string

	if selectedTermID != "" {
		if id, convErr := strconv.Atoi(selectedTermID); convErr == nil {
			database.DB.QueryRow("SELECT id, name, year FROM terms WHERE id = $1", id).Scan(&termID, &termName, &termYear)
		}
	}
	if termID == 0 {
		database.DB.QueryRow("SELECT id, name, year FROM terms WHERE is_current = TRUE LIMIT 1").Scan(&termID, &termName, &termYear)
	}

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
		"StudentName":     studentName,
		"ClassName":       className,
		"TermLabel":       termName + " term, " + termYear,
		"Results":         results,
		"Average":         average,
		"OverallGrade":    overallGrade,
		"AdmissionNumber": admissionNumber,
		"ParentPhone":     parentPhone,
		"SelectedTermID":  termID,
		"Terms":           terms,
	})
}

func PublicDownloadReportCard(w http.ResponseWriter, r *http.Request) {
	admissionNumber := strings.TrimSpace(r.URL.Query().Get("admission_number"))
	parentPhone := strings.TrimSpace(r.URL.Query().Get("parent_phone"))
	termIDParam := r.URL.Query().Get("term_id")

	if admissionNumber == "" || parentPhone == "" {
		http.Redirect(w, r, "/check-results", http.StatusSeeOther)
		return
	}

	var student models.Student
	var className, storedPhone string
	err := database.DB.QueryRow(`
		SELECT s.id, s.full_name, s.admission_number, COALESCE(c.name, ''), COALESCE(s.parent_phone, '')
		FROM students s
		LEFT JOIN classes c ON s.class_id = c.id
		WHERE LOWER(s.admission_number) = LOWER($1)
	`, admissionNumber).Scan(&student.ID, &student.FullName, &student.AdmissionNumber, &className, &storedPhone)

	if err != nil || normalizePhone(storedPhone) == "" || normalizePhone(storedPhone) != normalizePhone(parentPhone) {
		http.Error(w, "Verification failed. Go back and check the admission number and phone number.", http.StatusForbidden)
		return
	}

	var termID int
	var termName, termYear string

	if termIDParam != "" {
		if id, convErr := strconv.Atoi(termIDParam); convErr == nil {
			database.DB.QueryRow("SELECT id, name, year FROM terms WHERE id = $1", id).Scan(&termID, &termName, &termYear)
		}
	}
	if termID == 0 {
		database.DB.QueryRow("SELECT id, name, year FROM terms WHERE is_current = TRUE LIMIT 1").Scan(&termID, &termName, &termYear)
	}

	resultRows, err := database.DB.Query(`
		SELECT s.name, r.ca_score, r.exam_score, r.total, r.grade
		FROM results r
		JOIN subjects s ON r.subject_id = s.id
		WHERE r.student_id = $1 AND r.term_id = $2
		ORDER BY s.name ASC
	`, student.ID, termID)
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
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(80, 8, "Subject", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, "CA (40)", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Exam (60)", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Total", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Grade", "1", 1, "C", true, 0, "")

	pdf.SetFont("Arial", "", 10)
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
