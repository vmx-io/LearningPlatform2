package main

import (
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"gorm.io/gorm"
)

func drawQuestions(allIDs []string, count int, seed *int64) []string {
	var r *rand.Rand
	if seed != nil {
		r = rand.New(rand.NewSource(*seed))
	} else {
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	out := append([]string(nil), allIDs...)
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	if count > len(out) {
		count = len(out)
	}
	return out[:count]
}

func isCorrectAllOrNothing(selected, correct []string) bool {
	if len(selected) != len(correct) {
		return false
	}
	// treat both inputs as sets to avoid accepting duplicates in `selected`
	selSet := make(map[string]struct{}, len(selected))
	for _, k := range selected {
		selSet[k] = struct{}{}
	}
	// if there were duplicates in `selected`, the set will be smaller
	if len(selSet) != len(correct) {
		return false
	}
	for _, k := range correct {
		if _, ok := selSet[k]; !ok {
			return false
		}
	}
	return true
}

func computeExamScore(db *gorm.DB, examID string) (float64, int, int, error) {
	// 1) Total questions in the exam
	var totalQ int64
	if err := db.Model(&ExamQuestion{}).
		Where("exam_id = ?", examID).
		Count(&totalQ).Error; err != nil {
		return 0, 0, 0, err
	}
	if totalQ == 0 {
		return 0, 0, 0, errors.New("exam has no questions")
	}

	// 2) Load all answers ordered by time; keep latest per question
	var answers []Answer
	if err := db.Where("exam_id = ?", examID).
		Order("answered_at ASC, id ASC").
		Find(&answers).Error; err != nil {
		return 0, 0, 0, err
	}

	// Latest answer per question wins
	latest := map[string]Answer{}
	for _, a := range answers {
		latest[a.QuestionID] = a
	}

	// 3) Count correct using latest answers; unanswered count as wrong
	correct := 0
	for _, a := range latest {
		if a.IsCorrect {
			correct++
		}
	}
	wrong := int(totalQ) - correct
	score := float64(correct) * 100.0 / float64(totalQ)

	return score, correct, wrong, nil
}

func computeCorrectKeys(db *gorm.DB, qid string) ([]string, error) {
	var opts []Option
	if err := db.Where("question_id = ?", qid).Find(&opts).Error; err != nil {
		return nil, err
	}
	var keys []string
	for _, o := range opts {
		if o.IsCorrect {
			keys = append(keys, o.OptionKey)
		}
	}
	return keys, nil
}

func jsonArray(v []string) string {
	b, _ := json.Marshal(v)
	return string(b)
}
