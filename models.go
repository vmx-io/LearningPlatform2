package main

import (
	"time"
)
// --- User ---

type User struct {
	ID          uint      `gorm:"primaryKey"`
	PublicID    string    `gorm:"uniqueIndex;size:36;not null"` // UUID widoczny w cookie
	DisplayName *string
	Email       *string   `gorm:"uniqueIndex"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
  }

// --- Pytania ---

type Question struct {
	ID          string    `gorm:"primaryKey;size:64" json:"id"`
	TextEN      string    `gorm:"not null" json:"questionText"`
	TextPL      *string   `json:"questionTextPl,omitempty"`
	MultiSelect bool      `gorm:"not null" json:"multiSelect"`
	Difficulty  *int      `json:"difficulty,omitempty"`
	Tags        *string   `json:"tags,omitempty"` // CSV albo JSON (na razie prosty string)
	Version     int       `gorm:"not null;default:1" json:"version"`
	Options     []Option  `json:"options"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Option struct {
	ID         uint      `gorm:"primaryKey"`
	QuestionID string    `gorm:"index;not null"`
	OptionKey  string    `gorm:"size:4;not null"` // "a","b","c","d"
	TextEN     string    `gorm:"not null"`
	TextPL     *string
	IsCorrect  bool      `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Explanation struct {
	ID         uint      `gorm:"primaryKey"`
	QuestionID string    `gorm:"index;not null"`
	OptionKey  string    `gorm:"size:4;not null"`
	Lang       string    `gorm:"size:2;not null"` // "en" | "pl"
	Text       string    `gorm:"not null"`
	URL        string	 `gorm:""`  
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// --- Egzamin ---

type Exam struct {
	ID              string          `gorm:"primaryKey;size:36" json:"id"`
	UserID          *uint      		`gorm:"index" json:"-"`
	Type            string          `gorm:"not null;size:16" json:"type"` // "exam" | "learn"
	StartedAt       time.Time       `gorm:"not null"`
	FinishedAt      *time.Time
	DurationSeconds int             `gorm:"not null"` // np. 10800 (3h)
	ScorePercent    *float64
	Seed            *int64
	Questions       []ExamQuestion
	Answers         []Answer
}

type ExamQuestion struct {
	ID         uint   `gorm:"primaryKey"`
	ExamID     string `gorm:"index;not null"`
	QuestionID string `gorm:"not null"`
	Position   int    `gorm:"not null"` // 1..N
}

type Answer struct {
	ID          uint      `gorm:"primaryKey"`
	ExamID      string    `gorm:"index;not null"`
	QuestionID  string    `gorm:"not null"`
	SelectedRaw string    `gorm:"not null"` // JSON: ["a","c"]
	IsCorrect   bool      `gorm:"not null"`
	AnsweredAt  time.Time `gorm:"not null"`
}
