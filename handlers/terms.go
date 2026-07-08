package handlers

import (
	"net/http"
	"text/template"

	"school-manager/database"
	"school-manager/models"
)

func ShowTerms(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	rows, err := database.DB.Query(
		"SELECT id, name, year, is_current FROM terms ORDER BY id DESC",
	)
	if err != nil {
		http.Error(w, "Failed to load terms", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var terms []models.Term
	for rows.Next() {
		var t models.Term
		rows.Scan(&t.ID, &t.Name, &t.Year, &t.IsCurrent)
		terms = append(terms, t)
	}

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/terms.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":        "Terms",
		"Page":         "terms",
		"UserName":     session.Values["user_name"],
		"UserInitials": getInitials(session.Values["user_name"].(string)),
		"Role":         session.Values["user_role"],
		"Term":         getCurrentTerm(),
		"Terms":        terms,
	})
}

func HandleAddTerm(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	name := r.FormValue("name")
	year := r.FormValue("year")

	if name == "" || year == "" {
		http.Redirect(w, r, "/terms", http.StatusSeeOther)
		return
	}

	_, err := database.DB.Exec(
		"INSERT INTO terms (name, year, is_current) VALUES ($1, $2, FALSE)",
		name, year,
	)
	if err != nil {
		http.Error(w, "Failed to add term", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/terms", http.StatusSeeOther)
}

func HandleSetCurrentTerm(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	termID := r.FormValue("term_id")
	if termID == "" {
		http.Redirect(w, r, "/terms", http.StatusSeeOther)
		return
	}

	_, err := database.DB.Exec("UPDATE terms SET is_current = FALSE")
	if err != nil {
		http.Error(w, "Failed to update terms", http.StatusInternalServerError)
		return
	}

	_, err = database.DB.Exec("UPDATE terms SET is_current = TRUE WHERE id = $1", termID)
	if err != nil {
		http.Error(w, "Failed to set current term", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/terms", http.StatusSeeOther)
}
