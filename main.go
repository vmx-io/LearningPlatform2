package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1) DB
	db, err := OpenDB("quiz.db")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// 2) Seed (jeśli pusto)
	if isEmpty, _ := IsQuestionTableEmpty(db); isEmpty {
		path := "data/questions.json"
		if _, err := os.Stat(path); err == nil {
			if err := SeedFromJSON(db, path); err != nil {
				log.Fatalf("seed: %v", err)
			}
			log.Printf("Seeded questions from %s", path)
		} else {
			log.Printf("No seed file at %s; running with empty DB", path)
		}
	}

	// 3) Router
	r := gin.Default()

	// secureCookies: w dev zwykle false; w prod za HTTPS → true
	r.Use(EnsureUser(db, false))

	api := r.Group("/api/v1")
	{
		api.GET("/questions", ListQuestions(db))                  // tryb nauki: pobierz pytania (paginacja/tagi w kolejnych iteracjach)
		api.POST("/learn/answer", LearnAnswer(db))                // tryb nauki: odpowiedź -> od razu feedback + wyjaśnienia
		api.POST("/exams", StartExam(db))                         // start egzaminu (80 pytań domyślnie)
		api.POST("/exams/:id/answer", ExamAnswer(db))             // zapis odpowiedzi, bez ujawniania poprawności
		api.POST("/exams/:id/finish", FinishExam(db))             // wynik + raport
		api.GET("/me", GetMe(db))
    	api.PUT("/me", UpdateMe(db))
		api.GET("/me/export-key", ExportKey(db))
		api.POST("/me/restore", RestoreAccount(db, false)) 		  // prod: true
		api.GET("/exams", ListMyExams(db))
		api.GET("/exams/:id", GetMyExam(db))
		api.GET("/stats", Stats(db))
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("run: %v", err)
	}
}
