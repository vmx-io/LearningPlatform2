package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
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

	// 2) Seed (if empty)
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

	// --- CORS: Allow GH Pages + any localhost:port ---
	const ghOriginHttps = "https://vmx-io.github.io" // change if you move to a custom domain
	const ghOriginHttp = "http://vmx-io.github.io"   // change if you move to a custom domain
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			if origin == ghOriginHttp || origin == ghOriginHttps {
				return true
			}
			// allow any http://localhost:PORT during development
			if strings.HasPrefix(origin, "http://localhost:") {
				return true
			}
			return false
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "X-Public-Id"},
		ExposeHeaders:    []string{"X-Public-Id"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// --- Cookie security config ---
	secureCookies := os.Getenv("SECURE_COOKIES") == "true" // set to true on Cloud Run / HTTPS

	// --- User middleware (supports X-Public-Id header in your updated usermw.go) ---
	r.Use(EnsureUser(db, secureCookies))

	// Optional health check
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })

	// --- API routes ---
	api := r.Group("/api/v1")
	{
		// Learn mode
		api.GET("/questions", ListQuestions(db))
		api.POST("/learn/answer", LearnAnswer(db))

		// Exam mode
		api.POST("/exams", StartExam(db))
		api.POST("/exams/:id/answer", ExamAnswer(db))
		api.POST("/exams/:id/finish", FinishExam(db))

		// User profile
		api.GET("/me", GetMe(db))
		api.PUT("/me", UpdateMe(db))
		api.GET("/me/export-key", ExportKey(db))
		api.POST("/me/restore", RestoreAccount(db, secureCookies))

		// History & stats
		api.GET("/exams", ListMyExams(db))
		api.GET("/exams/:id", GetMyExam(db))
		api.GET("/stats", Stats(db))
	}

	// --- Server ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Listening on :%s (SecureCookies=%v, GH Origin=%s)", port, secureCookies, ghOriginHttps)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("run: %v", err)
	}
}
