package main

import (
	"log"
	"net/http"
	"os"

	"school-manager/database"
	"school-manager/handlers"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	database.Connect()

	r := mux.NewRouter()

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.HandleFunc("/", handlers.ShowLogin).Methods("GET")
	r.HandleFunc("/login", handlers.HandleLogin).Methods("POST")
	r.HandleFunc("/dashboard", handlers.ShowDashboard).Methods("GET")
	r.HandleFunc("/logout", handlers.HandleLogout).Methods("GET")
	r.HandleFunc("/classes", handlers.ShowClasses).Methods("GET")
	r.HandleFunc("/classes/add", handlers.HandleAddClass).Methods("POST")
	r.HandleFunc("/teachers", handlers.ShowTeachers).Methods("GET")
	r.HandleFunc("/teachers/add", handlers.HandleAddTeacher).Methods("POST")
	r.HandleFunc("/students", handlers.ShowStudents).Methods("GET")
	r.HandleFunc("/students/add", handlers.HandleAddStudent).Methods("POST")
	r.HandleFunc("/subjects", handlers.ShowSubjects).Methods("GET")
	r.HandleFunc("/subjects/add", handlers.HandleAddSubject).Methods("POST")
	r.HandleFunc("/subjects/assign", handlers.HandleAssignSubject).Methods("POST")
	r.HandleFunc("/results", handlers.ShowResults).Methods("GET")
	r.HandleFunc("/results/save", handlers.HandleSaveResults).Methods("POST")
	r.HandleFunc("/reportcards", handlers.ShowReportCards).Methods("GET")
	r.HandleFunc("/reportcards/generate", handlers.GenerateReportCard).Methods("GET")
	r.HandleFunc("/classes/assign-teacher", handlers.HandleAssignFormTeacher).Methods("POST")
	r.HandleFunc("/teacher/dashboard", handlers.ShowTeacherDashboard).Methods("GET")
	r.HandleFunc("/teacher/results", handlers.ShowTeacherResults).Methods("GET")
	r.HandleFunc("/teacher/results/save", handlers.HandleSaveTeacherResults).Methods("POST")
	r.HandleFunc("/terms", handlers.ShowTerms).Methods("GET")
	r.HandleFunc("/terms/add", handlers.HandleAddTerm).Methods("POST")
	r.HandleFunc("/terms/set-current", handlers.HandleSetCurrentTerm).Methods("POST")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Server running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
