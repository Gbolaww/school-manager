package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"text/template"

	"school-manager/database"
	"school-manager/models"
)

func gradeFromTotal(total float64) string {
	switch {
	case total >= 70:
		return "A"
	case total >= 60:
		return "B"
	case total >= 50:
		return "C"
	case total >= 45:
		return "D"
	case total >= 40:
		return "E"
	default:
		return "F"
	}
}

func ShowResults(w http.ResponseWriter, r *http.Request) {
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
	selectedSubjectID := r.URL.Query().Get("subject_id")

	var subjects []models.Subject
	var results []models.Result
	var selectedClass, selectedSubject string

	if selectedClassID != "" {
		subjectRows, err := database.DB.Query(`
			SELECT s.id, s.name FROM subjects s
			JOIN class_subjects cs ON cs.subject_id = s.id
			WHERE cs.class_id = $1
			ORDER BY s.name ASC
		`, selectedClassID)
		if err == nil {
			defer subjectRows.Close()
			for subjectRows.Next() {
				var s models.Subject
				subjectRows.Scan(&s.ID, &s.Name)
				subjects = append(subjects, s)
			}
		}
		database.DB.QueryRow("SELECT name FROM classes WHERE id = $1", selectedClassID).Scan(&selectedClass)
	}

	if selectedClassID != "" && selectedSubjectID != "" {
		database.DB.QueryRow("SELECT name FROM subjects WHERE id = $1", selectedSubjectID).Scan(&selectedSubject)

		var termID int
		database.DB.QueryRow("SELECT id FROM terms WHERE is_current = TRUE LIMIT 1").Scan(&termID)

		studentRows, err := database.DB.Query(`
			SELECT s.id, s.full_name,
				COALESCE(r.ca_score, 0),
				COALESCE(r.exam_score, 0),
				COALESCE(r.total, 0),
				COALESCE(r.grade, '')
			FROM students s
			LEFT JOIN results r ON r.student_id = s.id
				AND r.subject_id = $1
				AND r.term_id = $2
			WHERE s.class_id = $3
			ORDER BY s.full_name ASC
		`, selectedSubjectID, termID, selectedClassID)
		if err != nil {
			http.Error(w, "Failed to load results", http.StatusInternalServerError)
			return
		}
		defer studentRows.Close()

		for studentRows.Next() {
			var res models.Result
			studentRows.Scan(&res.StudentID, &res.StudentName, &res.CAScore, &res.ExamScore, &res.Total, &res.Grade)
			results = append(results, res)
		}
	}

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/results.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":             "Results",
		"Page":              "results",
		"UserName":          session.Values["user_name"],
		"UserInitials":      getInitials(session.Values["user_name"].(string)),
		"Role":              session.Values["user_role"],
		"Term":              getCurrentTerm(),
		"Classes":           classes,
		"Subjects":          subjects,
		"Results":           results,
		"SelectedClassID":   selectedClassID,
		"SelectedSubjectID": selectedSubjectID,
		"SelectedClass":     selectedClass,
		"SelectedSubject":   selectedSubject,
	})
}

func HandleSaveResults(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	classID := r.FormValue("class_id")
	subjectID := r.FormValue("subject_id")

	r.ParseForm()

	var termID int
	database.DB.QueryRow("SELECT id FROM terms WHERE is_current = TRUE LIMIT 1").Scan(&termID)

	for key, values := range r.Form {
		if len(values) == 0 {
			continue
		}

		var studentID int
		_, err := fmt.Sscanf(key, "ca_%d", &studentID)
		if err != nil {
			continue
		}

		caScore, err := strconv.ParseFloat(values[0], 64)
		if err != nil || caScore < 0 {
			continue
		}

		examScore := 0.0
		examKey := fmt.Sprintf("exam_%d", studentID)
		if examVals, ok := r.Form[examKey]; ok && len(examVals) > 0 {
			examScore, _ = strconv.ParseFloat(examVals[0], 64)
		}

		total := caScore + examScore
		grade := gradeFromTotal(total)

		database.DB.Exec(`
			INSERT INTO results (student_id, subject_id, class_id, term_id, ca_score, exam_score, total, grade)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (student_id, subject_id, term_id)
			DO UPDATE SET ca_score = $5, exam_score = $6, total = $7, grade = $8
		`, studentID, subjectID, classID, termID, caScore, examScore, total, grade)
	}

	http.Redirect(w, r, fmt.Sprintf("/results?class_id=%s&subject_id=%s", classID, subjectID), http.StatusSeeOther)
}
