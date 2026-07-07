package handlers

import (
	"net/http"
	"text/template"

	"school-manager/database"
	"school-manager/models"
)

func ShowStudents(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	rows, err := database.DB.Query(`
		SELECT s.id, s.full_name, s.admission_number, s.parent_phone, 
		       COALESCE(c.name, 'Unassigned') 
		FROM students s
		LEFT JOIN classes c ON s.class_id = c.id
		ORDER BY s.full_name ASC
	`)
	if err != nil {
		http.Error(w, "Failed to load students", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var students []models.Student
	for rows.Next() {
		var s models.Student
		rows.Scan(&s.ID, &s.FullName, &s.AdmissionNumber, &s.ParentPhone, &s.ClassName)
		students = append(students, s)
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

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/students.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":        "Students",
		"Page":         "students",
		"UserName":     session.Values["user_name"],
		"UserInitials": getInitials(session.Values["user_name"].(string)),
		"Role":         session.Values["user_role"],
		"Term":         "First term · 2025/2026",
		"Students":     students,
		"Classes":      classes,
	})
}

func HandleAddStudent(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	fullName := r.FormValue("full_name")
	admissionNumber := r.FormValue("admission_number")
	classID := r.FormValue("class_id")
	parentPhone := r.FormValue("parent_phone")

	if fullName == "" || admissionNumber == "" || classID == "" {
		http.Redirect(w, r, "/students", http.StatusSeeOther)
		return
	}

	_, err := database.DB.Exec(
		"INSERT INTO students (full_name, admission_number, class_id, parent_phone) VALUES ($1, $2, $3, $4)",
		fullName, admissionNumber, classID, parentPhone,
	)
	if err != nil {
		http.Error(w, "Failed to add student. Admission number may already exist.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/students", http.StatusSeeOther)
}
