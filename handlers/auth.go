package handlers

import (
	"log"
	"net/http"
	"strings"
	"text/template"

	"school-manager/database"
	"school-manager/models"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

var store = sessions.NewCookieStore([]byte("school-secret-key"))

func ShowLogin(w http.ResponseWriter, r *http.Request) {
	hasError := r.URL.Query().Get("error") == "invalid"
	tmpl := template.Must(template.ParseFiles("templates/login.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Error": hasError,
	})
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	log.Println("Email received:", email)

	var user models.User
	err := database.DB.QueryRow(
		"SELECT id, full_name, email, password_hash, role FROM users WHERE email = $1",
		email,
	).Scan(&user.ID, &user.FullName, &user.Email, &user.PasswordHash, &user.Role)

	if err != nil {
		log.Println("User not found:", err)
		http.Redirect(w, r, "/?error=invalid", http.StatusSeeOther)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		log.Println("Wrong password")
		http.Redirect(w, r, "/?error=invalid", http.StatusSeeOther)
		return
	}

	session, _ := store.Get(r, "session")
	session.Values["user_id"] = user.ID
	session.Values["user_name"] = user.FullName
	session.Values["user_role"] = user.Role
	session.Save(r, w)

	if user.Role == "teacher" {
		http.Redirect(w, r, "/teacher/dashboard", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

func getInitials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	if len(parts) == 1 {
		return strings.ToUpper(string(parts[0][0]))
	}
	return strings.ToUpper(string(parts[0][0])) + strings.ToUpper(string(parts[len(parts)-1][0]))
}

func ShowDashboard(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	userName := session.Values["user_name"]

	if userName == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var totalStudents, totalClasses, totalTeachers, totalReportCards int

	database.DB.QueryRow("SELECT COUNT(*) FROM students").Scan(&totalStudents)
	database.DB.QueryRow("SELECT COUNT(*) FROM classes").Scan(&totalClasses)
	database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'teacher'").Scan(&totalTeachers)

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/dashboard.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":            "Dashboard",
		"Page":             "dashboard",
		"UserName":         userName,
		"UserInitials":     getInitials(userName.(string)),
		"Role":             session.Values["user_role"],
		"Term":             "First term · 2025/2026",
		"TotalStudents":    totalStudents,
		"TotalClasses":     totalClasses,
		"TotalTeachers":    totalTeachers,
		"TotalReportCards": totalReportCards,
	})
}

func HandleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
