package handlers

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"

	"school-manager/database"
	"school-manager/models"

	"github.com/gorilla/mux"
)

func ShowStudents(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	search := r.URL.Query().Get("search")

	var rows *sql.Rows
	var err error

	if search != "" {
		rows, err = database.DB.Query(`
			SELECT s.id, s.full_name, s.admission_number, s.parent_phone,
			       COALESCE(c.name, 'Unassigned')
			FROM students s
			LEFT JOIN classes c ON s.class_id = c.id
			WHERE LOWER(s.full_name) LIKE LOWER($1)
			   OR LOWER(s.admission_number) LIKE LOWER($1)
			ORDER BY s.full_name ASC
		`, "%"+search+"%")
	} else {
		rows, err = database.DB.Query(`
			SELECT s.id, s.full_name, s.admission_number, s.parent_phone,
			       COALESCE(c.name, 'Unassigned')
			FROM students s
			LEFT JOIN classes c ON s.class_id = c.id
			ORDER BY s.full_name ASC
		`)
	}

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
		"Term":         getCurrentTerm(),
		"Students":     students,
		"Classes":      classes,
		"Search":       search,
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

func HandleEditStudent(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	id := mux.Vars(r)["id"]
	fullName := r.FormValue("full_name")
	admissionNumber := r.FormValue("admission_number")
	classID := r.FormValue("class_id")
	parentPhone := r.FormValue("parent_phone")

	_, err := database.DB.Exec(
		"UPDATE students SET full_name = $1, admission_number = $2, class_id = $3, parent_phone = $4 WHERE id = $5",
		fullName, admissionNumber, classID, parentPhone, id,
	)
	if err != nil {
		http.Error(w, "Failed to update student", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/students", http.StatusSeeOther)
}

func HandleDeleteStudent(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	id := mux.Vars(r)["id"]

	_, err := database.DB.Exec("DELETE FROM results WHERE student_id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete student results", http.StatusInternalServerError)
		return
	}

	_, err = database.DB.Exec("DELETE FROM students WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete student", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/students", http.StatusSeeOther)
}

func ShowImport(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/import.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":        "Import students",
		"Page":         "students",
		"UserName":     session.Values["user_name"],
		"UserInitials": getInitials(session.Values["user_name"].(string)),
		"Role":         session.Values["user_role"],
		"Term":         getCurrentTerm(),
	})
}

func HandleImport(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if session.Values["user_name"] == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	r.ParseMultipartForm(10 << 20)
	file, _, err := r.FormFile("csv_file")
	if err != nil {
		http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Skip header row
	_, err = reader.Read()
	if err != nil {
		http.Error(w, "Failed to read CSV header", http.StatusBadRequest)
		return
	}

	imported := 0
	skipped := 0
	var errors []string

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			skipped++
			continue
		}

		if len(record) < 3 {
			skipped++
			errors = append(errors, fmt.Sprintf("Row skipped — not enough columns: %v", record))
			continue
		}

		fullName := strings.TrimSpace(record[0])
		admissionNumber := strings.TrimSpace(record[1])
		className := strings.TrimSpace(record[2])
		parentPhone := ""
		if len(record) >= 4 {
			parentPhone = strings.TrimSpace(record[3])
		}

		if fullName == "" || admissionNumber == "" || className == "" {
			skipped++
			errors = append(errors, fmt.Sprintf("Row skipped — missing required fields: %s", admissionNumber))
			continue
		}

		var classID int
		err = database.DB.QueryRow("SELECT id FROM classes WHERE LOWER(name) = LOWER($1)", className).Scan(&classID)
		if err != nil {
			skipped++
			errors = append(errors, fmt.Sprintf("Row skipped — class not found: %s", className))
			continue
		}

		_, err = database.DB.Exec(
			"INSERT INTO students (full_name, admission_number, class_id, parent_phone) VALUES ($1, $2, $3, $4)",
			fullName, admissionNumber, classID, parentPhone,
		)
		if err != nil {
			skipped++
			errors = append(errors, fmt.Sprintf("Row skipped — duplicate admission number: %s", admissionNumber))
			continue
		}

		imported++
	}

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/import.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":        "Import students",
		"Page":         "students",
		"UserName":     session.Values["user_name"],
		"UserInitials": getInitials(session.Values["user_name"].(string)),
		"Role":         session.Values["user_role"],
		"Term":         getCurrentTerm(),
		"Imported":     imported,
		"Skipped":      skipped,
		"Errors":       errors,
		"Done":         true,
	})
}

func DownloadTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=students_template.csv")

	writer := csv.NewWriter(w)
	writer.Write([]string{"full_name", "admission_number", "class_name", "parent_phone"})
	writer.Write([]string{"Chidi Okonkwo", "GFS/2024/001", "JSS 1A", "08012345678"})
	writer.Write([]string{"Amaka Eze", "GFS/2024/002", "JSS 1A", "08098765432"})
	writer.Write([]string{"Bola Adeyemi", "GFS/2024/003", "SS 2B", "07011223344"})
	writer.Flush()
}
