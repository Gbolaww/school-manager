package handlers

import (
	"net/http"
	"text/template"

	"school-manager/database"
	"school-manager/models"

	"github.com/gorilla/mux"
)

func ShowSubjects(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	subjectRows, err := database.DB.Query("SELECT id, name FROM subjects ORDER BY name ASC")
	if err != nil {
		http.Error(w, "Failed to load subjects", http.StatusInternalServerError)
		return
	}
	defer subjectRows.Close()

	var subjects []models.Subject
	for subjectRows.Next() {
		var s models.Subject
		subjectRows.Scan(&s.ID, &s.Name)
		subjects = append(subjects, s)
	}

	assignRows, err := database.DB.Query(`
		SELECT cs.id, c.name, s.name, COALESCE(u.full_name, 'Not assigned')
		FROM class_subjects cs
		JOIN classes c ON cs.class_id = c.id
		JOIN subjects s ON cs.subject_id = s.id
		LEFT JOIN users u ON cs.teacher_id = u.id
		ORDER BY c.name, s.name ASC
	`)
	if err != nil {
		http.Error(w, "Failed to load assignments", http.StatusInternalServerError)
		return
	}
	defer assignRows.Close()

	var assignments []models.ClassSubject
	for assignRows.Next() {
		var cs models.ClassSubject
		assignRows.Scan(&cs.ID, &cs.ClassName, &cs.SubjectName, &cs.TeacherName)
		assignments = append(assignments, cs)
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

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/subjects.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":        "Subjects",
		"Page":         "subjects",
		"UserName":     session.Values["user_name"],
		"UserInitials": getInitials(session.Values["user_name"].(string)),
		"Role":         session.Values["user_role"],
		"Term":         getCurrentTerm(),
		"Subjects":     subjects,
		"Assignments":  assignments,
		"Classes":      classes,
		"Teachers":     teachers,
	})
}

func HandleAddSubject(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Redirect(w, r, "/subjects", http.StatusSeeOther)
		return
	}

	_, err := database.DB.Exec("INSERT INTO subjects (name) VALUES ($1)", name)
	if err != nil {
		http.Error(w, "Failed to add subject. It may already exist.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/subjects", http.StatusSeeOther)
}

func HandleEditSubject(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	id := mux.Vars(r)["id"]
	name := r.FormValue("name")

	if name == "" {
		http.Redirect(w, r, "/subjects", http.StatusSeeOther)
		return
	}

	_, err := database.DB.Exec("UPDATE subjects SET name = $1 WHERE id = $2", name, id)
	if err != nil {
		http.Error(w, "Failed to update subject", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/subjects", http.StatusSeeOther)
}

func HandleDeleteSubject(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	id := mux.Vars(r)["id"]

	_, err := database.DB.Exec("DELETE FROM results WHERE subject_id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete subject results", http.StatusInternalServerError)
		return
	}

	_, err = database.DB.Exec("DELETE FROM class_subjects WHERE subject_id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete subject assignments", http.StatusInternalServerError)
		return
	}

	_, err = database.DB.Exec("DELETE FROM subjects WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete subject", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/subjects", http.StatusSeeOther)
}

func HandleAssignSubject(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	classID := r.FormValue("class_id")
	subjectID := r.FormValue("subject_id")
	teacherID := r.FormValue("teacher_id")

	if classID == "" || subjectID == "" {
		http.Redirect(w, r, "/subjects", http.StatusSeeOther)
		return
	}

	_, err := database.DB.Exec(
		"INSERT INTO class_subjects (class_id, subject_id, teacher_id) VALUES ($1, $2, $3) ON CONFLICT (class_id, subject_id) DO UPDATE SET teacher_id = $3",
		classID, subjectID, teacherID,
	)
	if err != nil {
		http.Error(w, "Failed to assign subject", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/subjects", http.StatusSeeOther)
}
