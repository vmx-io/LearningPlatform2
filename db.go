package main

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func OpenDB(path string) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(path), &gorm.Config{})
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},        // nowy model użytkownika
		&Question{},
		&Option{},
		&Explanation{},
		&Exam{},
		&ExamQuestion{},
		&Answer{},
	)
}

func IsQuestionTableEmpty(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Model(&Question{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}
