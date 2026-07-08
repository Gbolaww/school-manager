package handlers

import (
	"net/http"
	"text/template"

	"school-manager/database"
	"school-manager/models"

	"github.com/gorilla/mux"

	"golang.org/x/crypto/bcrypt"
)

func ShowTeachers(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	rows, err := database.DB.Query(
		"SELECT id, full_name, email, role FROM users WHERE role = 'teacher' ORDER BY full_name ASC",
	)
	if err != nil {
		http.Error(w, "Failed to load teachers", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var teachers []models.User
	for rows.Next() {
		var t models.User
		rows.Scan(&t.ID, &t.FullName, &t.Email, &t.Role)
		teachers = append(teachers, t)
	}

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/teachers.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":        "Teachers",
		"Page":         "teachers",
		"UserName":     session.Values["user_name"],
		"UserInitials": getInitials(session.Values["user_name"].(string)),
		"Role":         session.Values["user_role"],
		"Term":         getCurrentTerm(),
		"Teachers":     teachers,
	})
}

func HandleAddTeacher(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	fullName := r.FormValue("full_name")
	email := r.FormValue("email")
	password := r.FormValue("password")

	if fullName == "" || email == "" || password == "" {
		http.Redirect(w, r, "/teachers", http.StatusSeeOther)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to process password", http.StatusInternalServerError)
		return
	}

	_, err = database.DB.Exec(
		"INSERT INTO users (full_name, email, password_hash, role) VALUES ($1, $2, $3, 'teacher')",
		fullName, email, string(hashedPassword),
	)
	if err != nil {
		http.Error(w, "Failed to add teacher. Email may already exist.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/teachers", http.StatusSeeOther)
}

func HandleEditTeacher(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	id := mux.Vars(r)["id"]
	fullName := r.FormValue("full_name")
	email := r.FormValue("email")
	password := r.FormValue("password")

	if fullName == "" || email == "" {
		http.Redirect(w, r, "/teachers", http.StatusSeeOther)
		return
	}

	if password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Failed to process password", http.StatusInternalServerError)
			return
		}
		_, err = database.DB.Exec(
			"UPDATE users SET full_name = $1, email = $2, password_hash = $3 WHERE id = $4",
			fullName, email, string(hashedPassword), id,
		)
		if err != nil {
			http.Error(w, "Failed to update teacher", http.StatusInternalServerError)
			return
		}
	} else {
		_, err := database.DB.Exec(
			"UPDATE users SET full_name = $1, email = $2 WHERE id = $3",
			fullName, email, id,
		)
		if err != nil {
			http.Error(w, "Failed to update teacher", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/teachers", http.StatusSeeOther)
}

func HandleDeleteTeacher(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	id := mux.Vars(r)["id"]

	_, err := database.DB.Exec("UPDATE classes SET form_teacher_id = NULL WHERE form_teacher_id = $1", id)
	if err != nil {
		http.Error(w, "Failed to unassign form teacher", http.StatusInternalServerError)
		return
	}

	_, err = database.DB.Exec("UPDATE class_subjects SET teacher_id = NULL WHERE teacher_id = $1", id)
	if err != nil {
		http.Error(w, "Failed to unassign subject teacher", http.StatusInternalServerError)
		return
	}

	_, err = database.DB.Exec("DELETE FROM users WHERE id = $1 AND role = 'teacher'", id)
	if err != nil {
		http.Error(w, "Failed to delete teacher", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/teachers", http.StatusSeeOther)
}
