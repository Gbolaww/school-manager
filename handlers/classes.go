package handlers

import (
	"net/http"
	"text/template"

	"school-manager/database"
	"school-manager/models"
)

func ShowClasses(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	rows, err := database.DB.Query(`
		SELECT c.id, c.name, COALESCE(u.full_name, 'Not assigned')
		FROM classes c
		LEFT JOIN users u ON c.form_teacher_id = u.id
		ORDER BY c.name ASC
	`)
	if err != nil {
		http.Error(w, "Failed to load classes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ClassRow struct {
		ID           int
		Name         string
		FormTeacher  string
		StudentCount int
	}

	var classes []ClassRow
	for rows.Next() {
		var c ClassRow
		rows.Scan(&c.ID, &c.Name, &c.FormTeacher)
		database.DB.QueryRow("SELECT COUNT(*) FROM students WHERE class_id = $1", c.ID).Scan(&c.StudentCount)
		classes = append(classes, c)
	}

	teacherRows, err := database.DB.Query("SELECT id, full_name FROM users WHERE role = 'teacher' ORDER BY full_name ASC")
	if err != nil {
		http.Error(w, "Failed to load teachers", http.StatusInternalServerError)
		return
	}
	defer teacherRows.Close()

	var teachers []models.User
	for teacherRows.Next() {
		var t models.User
		teacherRows.Scan(&t.ID, &t.FullName)
		teachers = append(teachers, t)
	}

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/classes.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":        "Classes",
		"Page":         "classes",
		"UserName":     session.Values["user_name"],
		"UserInitials": getInitials(session.Values["user_name"].(string)),
		"Role":         session.Values["user_role"],
		"Term":         getCurrentTerm(),
		"Classes":      classes,
		"Teachers":     teachers,
	})
}

func HandleAddClass(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Redirect(w, r, "/classes", http.StatusSeeOther)
		return
	}

	_, err := database.DB.Exec("INSERT INTO classes (name) VALUES ($1)", name)
	if err != nil {
		http.Error(w, "Failed to add class", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/classes", http.StatusSeeOther)
}

func HandleAssignFormTeacher(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	classID := r.FormValue("class_id")
	teacherID := r.FormValue("teacher_id")

	if classID == "" || teacherID == "" {
		http.Redirect(w, r, "/classes", http.StatusSeeOther)
		return
	}

	_, err := database.DB.Exec(
		"UPDATE classes SET form_teacher_id = $1 WHERE id = $2",
		teacherID, classID,
	)
	if err != nil {
		http.Error(w, "Failed to assign form teacher", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/classes", http.StatusSeeOther)
}
