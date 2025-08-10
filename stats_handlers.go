package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StatsResponse struct {
	TotalExams      int64              `json:"totalExams"`
	CompletedExams  int64              `json:"completedExams"`
	AverageScore    *float64           `json:"averageScore,omitempty"`
	TotalAnswers    int64              `json:"totalAnswers"`
	CorrectAnswers  int64              `json:"correctAnswers"`
	AccuracyOverall *float64           `json:"accuracyOverall,omitempty"`
	AnswersLast30d  int64              `json:"answersLast30d"`
	CorrectLast30d  int64              `json:"correctLast30d"`
	AccuracyLast30d *float64           `json:"accuracyLast30d,omitempty"`
	AccuracyByTag   map[string]float64 `json:"accuracyByTag,omitempty"` // tag -> percent
	AnsweredByTag   map[string]int64   `json:"answeredByTag,omitempty"` // tag -> count
	PassedExams     int64              `json:"passedExams"`
	FailedExams     int64              `json:"failedExams"`
	PassRate        *float64           `json:"passRate,omitempty"`
}

func Stats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// auth
		v, ok := c.Get("userDBID")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user"})
			return
		}
		uid := v.(uint)

		resp := StatsResponse{
			AccuracyByTag: make(map[string]float64),
			AnsweredByTag: make(map[string]int64),
		}

		// exams counts
		if err := db.Model(&Exam{}).Where("user_id = ?", uid).Count(&resp.TotalExams).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}
		if err := db.Model(&Exam{}).
			Where("user_id = ? AND finished_at IS NOT NULL", uid).
			Count(&resp.CompletedExams).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}

		// pass/fail (cut score = 61%)
		const passCut = 61.0
		if err := db.Model(&Exam{}).
			Where("user_id = ? AND score_percent IS NOT NULL AND score_percent >= ?", uid, passCut).
			Count(&resp.PassedExams).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}
		if err := db.Model(&Exam{}).
			Where("user_id = ? AND score_percent IS NOT NULL AND score_percent < ?", uid, passCut).
			Count(&resp.FailedExams).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
			return
		}
		if resp.CompletedExams > 0 {
			pr := float64(resp.PassedExams) * 100.0 / float64(resp.CompletedExams)
			resp.PassRate = &pr
		}

		// average score (only finished exams)
		type RowAvg struct{ Avg *float64 }
		var rowAvg RowAvg
		_ = db.Table("exams").
			Where("user_id = ? AND score_percent IS NOT NULL", uid).
			Select("AVG(score_percent) as avg").
			Scan(&rowAvg).Error
		resp.AverageScore = rowAvg.Avg

		// overall answers & correct (join answers->exams to filter by user)
		type RowCnt struct{ C int64 }
		var total RowCnt
		_ = db.Table("answers a").
			Joins("JOIN exams e ON e.id = a.exam_id").
			Where("e.user_id = ?", uid).
			Select("COUNT(*) as c").Scan(&total).Error
		resp.TotalAnswers = total.C

		var corr RowCnt
		_ = db.Table("answers a").
			Joins("JOIN exams e ON e.id = a.exam_id").
			Where("e.user_id = ? AND a.is_correct = 1", uid).
			Select("COUNT(*) as c").Scan(&corr).Error
		resp.CorrectAnswers = corr.C

		if resp.TotalAnswers > 0 {
			acc := float64(resp.CorrectAnswers) * 100.0 / float64(resp.TotalAnswers)
			resp.AccuracyOverall = &acc
		}

		// last 30 days
		since := time.Now().Add(-30 * 24 * time.Hour)
		var tot30 RowCnt
		_ = db.Table("answers a").
			Joins("JOIN exams e ON e.id = a.exam_id").
			Where("e.user_id = ? AND a.answered_at >= ?", uid, since).
			Select("COUNT(*) as c").Scan(&tot30).Error
		resp.AnswersLast30d = tot30.C

		var cor30 RowCnt
		_ = db.Table("answers a").
			Joins("JOIN exams e ON e.id = a.exam_id").
			Where("e.user_id = ? AND a.answered_at >= ? AND a.is_correct = 1", uid, since).
			Select("COUNT(*) as c").Scan(&cor30).Error
		resp.CorrectLast30d = cor30.C

		if resp.AnswersLast30d > 0 {
			acc30 := float64(resp.CorrectLast30d) * 100.0 / float64(resp.AnswersLast30d)
			resp.AccuracyLast30d = &acc30
		}

		// accuracy per tag (CSV in questions.Tags)
		// Load answers + their questions' tags, then aggregate in Go.
		type AnsJoin struct {
			IsCorrect bool
			Tags      *string // CSV or JSON-ish string; we treat it as CSV "OMS,SOLR"
		}
		var rows []AnsJoin
		_ = db.Table("answers a").
			Select("a.is_correct as is_correct, q.tags as tags").
			Joins("JOIN exams e ON e.id = a.exam_id").
			Joins("JOIN questions q ON q.id = a.question_id").
			Where("e.user_id = ?", uid).
			Scan(&rows).Error

		tagTotals := map[string]int64{}
		tagCorrect := map[string]int64{}
		for _, r := range rows {
			if r.Tags == nil || *r.Tags == "" {
				continue
			}
			// split CSV (e.g., "OMS, Backoffice")
			parts := strings.Split(*r.Tags, ",")
			seen := map[string]bool{}
			for _, p := range parts {
				tag := strings.TrimSpace(p)
				if tag == "" || seen[tag] {
					continue
				}
				seen[tag] = true
				tagTotals[tag]++
				if r.IsCorrect {
					tagCorrect[tag]++
				}
			}
		}
		for tag, tot := range tagTotals {
			resp.AnsweredByTag[tag] = tot
			if tot > 0 {
				resp.AccuracyByTag[tag] = float64(tagCorrect[tag]) * 100.0 / float64(tot)
			}
		}

		c.JSON(http.StatusOK, resp)
	}
}
