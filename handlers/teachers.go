package handlers

import (
	"net/http"
	"text/template"

	"school-manager/database"
	"school-manager/models"

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
		"Term":         "First term · 2025/2026",
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
