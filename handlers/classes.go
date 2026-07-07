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

	rows, err := database.DB.Query("SELECT id, name FROM classes ORDER BY name ASC")
	if err != nil {
		http.Error(w, "Failed to load classes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var classes []models.Class
	for rows.Next() {
		var c models.Class
		rows.Scan(&c.ID, &c.Name)
		classes = append(classes, c)
	}

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/classes.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":        "Classes",
		"Page":         "classes",
		"UserName":     session.Values["user_name"],
		"UserInitials": getInitials(session.Values["user_name"].(string)),
		"Role":         session.Values["user_role"],
		"Term":         "First term · 2025/2026",
		"Classes":      classes,
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
