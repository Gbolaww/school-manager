package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"text/template"

	"school-manager/database"
	"school-manager/models"
)

func ShowTeacherDashboard(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil || session.Values["user_role"] != "teacher" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	userID := session.Values["user_id"].(int)

	assignRows, err := database.DB.Query(`
		SELECT cs.class_id, cs.subject_id, c.name, s.name
		FROM class_subjects cs
		JOIN classes c ON cs.class_id = c.id
		JOIN subjects s ON cs.subject_id = s.id
		WHERE cs.teacher_id = $1
		ORDER BY c.name, s.name ASC
	`, userID)
	if err != nil {
		http.Error(w, "Failed to load assignments", http.StatusInternalServerError)
		return
	}
	defer assignRows.Close()

	var assignments []models.ClassSubject
	for assignRows.Next() {
		var cs models.ClassSubject
		assignRows.Scan(&cs.ClassID, &cs.SubjectID, &cs.ClassName, &cs.SubjectName)
		assignments = append(assignments, cs)
	}

	tmpl := template.Must(template.ParseFiles("templates/teacher_layout.html", "templates/teacher_dashboard.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":        "Dashboard",
		"Page":         "dashboard",
		"UserName":     session.Values["user_name"],
		"UserInitials": getInitials(session.Values["user_name"].(string)),
		"Term":         "First term · 2025/2026",
		"Assignments":  assignments,
	})
}

func ShowTeacherResults(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil || session.Values["user_role"] != "teacher" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	classID := r.URL.Query().Get("class_id")
	subjectID := r.URL.Query().Get("subject_id")

	var selectedClass, selectedSubject string
	database.DB.QueryRow("SELECT name FROM classes WHERE id = $1", classID).Scan(&selectedClass)
	database.DB.QueryRow("SELECT name FROM subjects WHERE id = $1", subjectID).Scan(&selectedSubject)

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
	`, subjectID, termID, classID)
	if err != nil {
		http.Error(w, "Failed to load students", http.StatusInternalServerError)
		return
	}
	defer studentRows.Close()

	var results []models.Result
	for studentRows.Next() {
		var res models.Result
		studentRows.Scan(&res.StudentID, &res.StudentName, &res.CAScore, &res.ExamScore, &res.Total, &res.Grade)
		results = append(results, res)
	}

	tmpl := template.Must(template.ParseFiles("templates/teacher_layout.html", "templates/teacher_results.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":             "Enter results",
		"Page":              "results",
		"UserName":          session.Values["user_name"],
		"UserInitials":      getInitials(session.Values["user_name"].(string)),
		"Term":              "First term · 2025/2026",
		"Results":           results,
		"SelectedClassID":   classID,
		"SelectedSubjectID": subjectID,
		"SelectedClass":     selectedClass,
		"SelectedSubject":   selectedSubject,
	})
}

func HandleSaveTeacherResults(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil || session.Values["user_role"] != "teacher" {
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

	http.Redirect(w, r, fmt.Sprintf("/teacher/results?class_id=%s&subject_id=%s", classID, subjectID), http.StatusSeeOther)
}
