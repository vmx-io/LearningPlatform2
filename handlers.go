package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

/*** DTOs shared across handlers ***/

type QuestionDTO struct {
	ID           string      `json:"id"`
	QuestionText string      `json:"questionText"`
	MultiSelect  bool        `json:"multiSelect"`
	Options      []OptionDTO `json:"options"`
}

type OptionDTO struct {
	ID   string `json:"id"`   // "a"/"b"/"c"/"d"
	Text string `json:"text"` // EN for now
}

type ExpDTO struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

/*** Learning mode ***/

type LearnAnswerReq struct {
	QuestionID string   `json:"questionId"`
	Selected   []string `json:"selected"`
	Lang       string   `json:"lang"` // "en" | "pl"
}

const passThreshold = 61.0 // percent

func passedPtr(score *float64) *bool {
	if score == nil {
		return nil // exam not finished yet
	}
	v := *score >= passThreshold
	return &v
}

func ListQuestions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var qs []Question
		if err := db.Preload("Options").Order("id").Find(&qs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}
		out := make([]QuestionDTO, 0, len(qs))
		for _, q := range qs {
			opts := make([]OptionDTO, 0, len(q.Options))
			for _, o := range q.Options {
				opts = append(opts, OptionDTO{ID: o.OptionKey, Text: o.TextEN})
			}
			out = append(out, QuestionDTO{
				ID: q.ID, QuestionText: q.TextEN, MultiSelect: q.MultiSelect, Options: opts,
			})
		}
		c.JSON(http.StatusOK, out)
	}
}

func LearnAnswer(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LearnAnswerReq
		if err := c.BindJSON(&req); err != nil || req.QuestionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
			return
		}
		for i := range req.Selected {
			req.Selected[i] = strings.ToLower(req.Selected[i])
		}

		// ensure question exists
		var q Question
		if err := db.First(&q, "id = ?", req.QuestionID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "question not found"})
			return
		}

		correct, err := computeCorrectKeys(db, req.QuestionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}
		if len(correct) == 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "question has no options/correct answers in DB"})
			return
		}

		ok := isCorrectAllOrNothing(req.Selected, correct)

		lang := "en"
		if strings.ToLower(req.Lang) == "pl" {
			lang = "pl"
		}
		var expl []Explanation
		if err := db.Where("question_id = ? AND lang = ?", req.QuestionID, lang).Find(&expl).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}
		byKey := map[string]ExpDTO{}
		for _, e := range expl {
			byKey[e.OptionKey] = ExpDTO{Text: e.Text, URL: e.URL}
		}

		c.JSON(http.StatusOK, gin.H{
			"isCorrect":        ok,
			"correctOptionIds": correct,
			"explanations":     byKey,
		})
	}
}

/*** Exam mode ***/

type StartExamReq struct {
	Count       int    `json:"count"`       // default 80
	DurationSec int    `json:"durationSec"` // default 10800
	Seed        *int64 `json:"seed"`        // optional for reproducibility
}

func StartExam(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req StartExamReq
		_ = c.BindJSON(&req)
		if req.Count <= 0 {
			req.Count = 80
		}
		if req.DurationSec <= 0 {
			req.DurationSec = 3 * 60 * 60
		}

		var ids []string
		if err := db.Model(&Question{}).Pluck("id", &ids).Error; err != nil || len(ids) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no questions"})
			return
		}
		drawn := drawQuestions(ids, req.Count, req.Seed)

		// bind current user (if any)
		var userID *uint
		if v, ok := c.Get("userDBID"); ok {
			if id, ok2 := v.(uint); ok2 {
				userID = &id
			}
		}

		examID := uuid.New().String()
		exam := Exam{
			ID:              examID,
			Type:            "exam",
			StartedAt:       time.Now(),
			DurationSeconds: req.DurationSec,
			Seed:            req.Seed,
			UserID:          userID,
		}
		if err := db.Create(&exam).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}
		for i, qid := range drawn {
			eq := ExamQuestion{ExamID: examID, QuestionID: qid, Position: i + 1}
			if err := db.Create(&eq).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
				return
			}
		}

		// fetch questions & keep original order
		var qs []Question
		if err := db.Preload("Options").Where("id IN ?", drawn).Find(&qs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}
		index := map[string]Question{}
		for _, q := range qs {
			index[q.ID] = q
		}
		out := make([]QuestionDTO, 0, len(drawn))
		for _, id := range drawn {
			q := index[id]
			opts := make([]OptionDTO, 0, len(q.Options))
			for _, o := range q.Options {
				opts = append(opts, OptionDTO{ID: o.OptionKey, Text: o.TextEN})
			}
			out = append(out, QuestionDTO{
				ID: q.ID, QuestionText: q.TextEN, MultiSelect: q.MultiSelect, Options: opts,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"examId":      examID,
			"durationSec": req.DurationSec,
			"questions":   out,
		})
	}
}

type ExamAnswerReq struct {
	Selected []string `json:"selected"`
}

func ExamAnswer(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		examID := c.Param("id")
		qid := c.Query("questionId")
		if examID == "" || qid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing examId/questionId"})
			return
		}
		var exam Exam
		if err := db.First(&exam, "id = ?", examID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "exam not found"})
			return
		}
		var req ExamAnswerReq
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
			return
		}
		for i := range req.Selected {
			req.Selected[i] = strings.ToLower(req.Selected[i])
		}
		correct, err := computeCorrectKeys(db, qid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}
		ok := isCorrectAllOrNothing(req.Selected, correct)

		raw, _ := json.Marshal(req.Selected)
		ans := Answer{
			ExamID:      examID,
			QuestionID:  qid,
			SelectedRaw: string(raw),
			IsCorrect:   ok,
			AnsweredAt:  time.Now(),
		}
		if err := db.Create(&ans).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}
		// Do NOT reveal correctness during exam
		c.JSON(http.StatusOK, gin.H{"saved": true})
	}
}

func FinishExam(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		examID := c.Param("id")
		var exam Exam
		if err := db.First(&exam, "id = ?", examID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "exam not found"})
			return
		}
		score, correct, wrong, err := computeExamScore(db, examID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		now := time.Now()
		exam.FinishedAt = &now
		exam.ScorePercent = &score
		if err := db.Save(&exam).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}

		type ReviewRow struct {
			QuestionID     string            `json:"questionId"`
			QuestionText   string            `json:"questionText"`
			Selected       []string          `json:"selected"`
			Correct        []string          `json:"correct"`
			ExplanationsEn map[string]ExpDTO `json:"explanationsEn"`
			ExplanationsPl map[string]ExpDTO `json:"explanationsPl"`
			WasCorrect     bool              `json:"wasCorrect"`
		}

		var eqs []ExamQuestion
		if err := db.Where("exam_id = ?", examID).Order("position").Find(&eqs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}

		toMap := func(xs []Explanation) map[string]ExpDTO {
			m := map[string]ExpDTO{}
			for _, e := range xs {
				m[e.OptionKey] = ExpDTO{Text: e.Text, URL: e.URL}
			}
			return m
		}

		review := []ReviewRow{}
		for _, eq := range eqs {
			var q Question
			if err := db.First(&q, "id = ?", eq.QuestionID).Error; err != nil {
				continue
			}
			var a Answer
			_ = db.Where("exam_id = ? AND question_id = ?", examID, q.ID).
				Order("answered_at DESC, id DESC").
				First(&a).Error

			var selected []string
			_ = json.Unmarshal([]byte(a.SelectedRaw), &selected)

			correctKeys, _ := computeCorrectKeys(db, q.ID)

			var exEN, exPL []Explanation
			_ = db.Where("question_id = ? AND lang = 'en'", q.ID).Find(&exEN).Error
			_ = db.Where("question_id = ? AND lang = 'pl'", q.ID).Find(&exPL).Error

			review = append(review, ReviewRow{
				QuestionID:     q.ID,
				QuestionText:   q.TextEN,
				Selected:       selected,
				Correct:        correctKeys,
				ExplanationsEn: toMap(exEN),
				ExplanationsPl: toMap(exPL),
				WasCorrect:     a.IsCorrect,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"scorePercent": score,
			"correct":      correct,
			"wrong":        wrong,
			"passed":       passedPtr(exam.ScorePercent),
			"items":        review,
		})
	}
}

// ===== Exam history: list & detail (read-only) =====

type ExamSummaryDTO struct {
	ID            string     `json:"id"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	DurationSec   int        `json:"durationSec"`
	ScorePercent  *float64   `json:"scorePercent,omitempty"`
	QuestionCount int        `json:"questionCount"`
}

// ListMyExams returns user exams with pagination.
// Query params: ?limit=20&offset=0  (limit default 20, max 100)
func ListMyExams(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// auth
		v, ok := c.Get("userDBID")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user"})
			return
		}
		uid := v.(uint)

		// pagination
		limit := 20
		offset := 0
		if l := c.Query("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				if n > 100 {
					n = 100
				}
				limit = n
			}
		}
		if o := c.Query("offset"); o != "" {
			if n, err := strconv.Atoi(o); err == nil && n >= 0 {
				offset = n
			}
		}

		// total
		var total int64
		if err := db.Model(&Exam{}).Where("user_id = ?", uid).Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}

		// page items
		var exams []Exam
		if err := db.Where("user_id = ?", uid).
			Order("started_at DESC").
			Limit(limit).Offset(offset).
			Find(&exams).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}

		// count questions per exam
		ids := make([]string, 0, len(exams))
		for _, e := range exams {
			ids = append(ids, e.ID)
		}

		counts := map[string]int{}
		if len(ids) > 0 {
			type Row struct {
				ExamID string
				C      int
			}
			var rows []Row
			if err := db.Table("exam_questions").
				Select("exam_id as exam_id, COUNT(*) as c").
				Where("exam_id IN ?", ids).
				Group("exam_id").
				Scan(&rows).Error; err == nil {
				for _, r := range rows {
					counts[r.ExamID] = r.C
				}
			}
		}

		type ExamSummaryDTO struct {
			ID            string     `json:"id"`
			StartedAt     time.Time  `json:"startedAt"`
			FinishedAt    *time.Time `json:"finishedAt,omitempty"`
			DurationSec   int        `json:"durationSec"`
			ScorePercent  *float64   `json:"scorePercent,omitempty"`
			QuestionCount int        `json:"questionCount"`
			Passed        *bool      `json:"passed,omitempty"`
		}

		items := make([]ExamSummaryDTO, 0, len(exams))
		for _, e := range exams {
			items = append(items, ExamSummaryDTO{
				ID:            e.ID,
				StartedAt:     e.StartedAt,
				FinishedAt:    e.FinishedAt,
				DurationSec:   e.DurationSeconds,
				ScorePercent:  e.ScorePercent,
				QuestionCount: counts[e.ID],
				Passed:        passedPtr(e.ScorePercent),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"items":  items,
		})
	}
}

func GetMyExam(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		examID := c.Param("id")

		// auth & ownership
		v, ok := c.Get("userDBID")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user"})
			return
		}
		uid := v.(uint)

		var exam Exam
		if err := db.First(&exam, "id = ?", examID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "exam not found"})
			return
		}
		if exam.UserID == nil || *exam.UserID != uid {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		// Build the same review payload as in FinishExam (read-only)
		type ReviewRow struct {
			QuestionID     string            `json:"questionId"`
			QuestionText   string            `json:"questionText"`
			Selected       []string          `json:"selected"`
			Correct        []string          `json:"correct"`
			ExplanationsEn map[string]ExpDTO `json:"explanationsEn"`
			ExplanationsPl map[string]ExpDTO `json:"explanationsPl"`
			WasCorrect     bool              `json:"wasCorrect"`
		}

		// order of questions
		var eqs []ExamQuestion
		if err := db.Where("exam_id = ?", examID).Order("position").Find(&eqs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}

		// helper
		toMap := func(xs []Explanation) map[string]ExpDTO {
			m := map[string]ExpDTO{}
			for _, e := range xs {
				m[e.OptionKey] = ExpDTO{Text: e.Text, URL: e.URL}
			}
			return m
		}

		review := []ReviewRow{}
		correctCount := 0
		for _, eq := range eqs {
			var q Question
			if err := db.First(&q, "id = ?", eq.QuestionID).Error; err != nil {
				continue
			}
			var a Answer
			_ = db.Where("exam_id = ? AND question_id = ?", examID, q.ID).
				Order("answered_at DESC, id DESC").
				First(&a).Error

			var selected []string
			_ = json.Unmarshal([]byte(a.SelectedRaw), &selected)
			correctKeys, _ := computeCorrectKeys(db, q.ID)
			if a.IsCorrect {
				correctCount++
			}

			var exEN, exPL []Explanation
			_ = db.Where("question_id = ? AND lang = 'en'", q.ID).Find(&exEN).Error
			_ = db.Where("question_id = ? AND lang = 'pl'", q.ID).Find(&exPL).Error

			review = append(review, ReviewRow{
				QuestionID:     q.ID,
				QuestionText:   q.TextEN,
				Selected:       selected,
				Correct:        correctKeys,
				ExplanationsEn: toMap(exEN),
				ExplanationsPl: toMap(exPL),
				WasCorrect:     a.IsCorrect,
			})
		}

		var totalQ int64
		_ = db.Model(&ExamQuestion{}).Where("exam_id = ?", examID).Count(&totalQ).Error

		c.JSON(http.StatusOK, gin.H{
			"examId":       exam.ID,
			"startedAt":    exam.StartedAt,
			"finishedAt":   exam.FinishedAt,
			"durationSec":  exam.DurationSeconds,
			"scorePercent": exam.ScorePercent,
			"passed":       passedPtr(exam.ScorePercent),
			"correct":      correctCount,
			"wrong":        int(totalQ) - correctCount,
			"items":        review,
		})
	}
}
