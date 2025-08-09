package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"
)

// ==== JSON input structures ====

type ExplanationItem struct {
	ID  string `json:"id"`
	Text string `json:"text"`
	URL  string `json:"url"` // optional in JSON; empty string if not provided
}

type OptionsExplanation struct {
	EN []ExplanationItem `json:"en"`
	PL []ExplanationItem `json:"pl"`
}

type QInputOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type QInput struct {
	ID               string             `json:"id"`
	QuestionText     string             `json:"questionText"`
	Options          []QInputOption     `json:"options"`
	OptionsExplanation OptionsExplanation `json:"optionsExplanation"`
	MultiSelect      bool               `json:"multiSelect"`
	CorrectOptionIds []string           `json:"correctOptionIds"`
}

// ==== Seeder ====

func SeedFromJSON(db *gorm.DB, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Accept either: [ ... ] or { "questions": [ ... ] }
	var wrapper struct {
		Questions []QInput `json:"questions"`
	}
	var arr []QInput

	if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper.Questions) > 0 {
		arr = wrapper.Questions
	} else if err := json.Unmarshal(raw, &arr); err != nil {
		return fmt.Errorf("json parse: %w", err)
	}

	// Basic validation: unique question IDs
	seen := map[string]bool{}
	dups := []string{}
	for _, q := range arr {
		if seen[q.ID] {
			dups = append(dups, q.ID)
		}
		seen[q.ID] = true
	}
	if len(dups) > 0 {
		return fmt.Errorf("duplicate question IDs in JSON: %v", dups)
	}

	// Seed transactionally
	return db.Transaction(func(tx *gorm.DB) error {
		for _, in := range arr {
			q := Question{
				ID:          in.ID,
				TextEN:      in.QuestionText,
				MultiSelect: in.MultiSelect,
				Version:     1,
			}
			if err := tx.Create(&q).Error; err != nil {
				return err
			}

			// Build set of correct option keys (lowercased "a".."d")
			correctSet := map[string]bool{}
			for _, k := range in.CorrectOptionIds {
				correctSet[stringsLower(k)] = true
			}

			// Insert options
			for _, o := range in.Options {
				ok := correctSet[stringsLower(o.ID)]
				option := Option{
					QuestionID: q.ID,
					OptionKey:  stringsLower(o.ID),
					TextEN:     o.Text,
					IsCorrect:  ok,
				}
				if err := tx.Create(&option).Error; err != nil {
					return err
				}
			}

			// Insert explanations (EN)
			for _, e := range in.OptionsExplanation.EN {
				ex := Explanation{
					QuestionID: q.ID,
					OptionKey:  stringsLower(e.ID),
					Lang:       "en",
					Text:       e.Text,
					URL:        strings.TrimSpace(e.URL),
				}
				if err := tx.Create(&ex).Error; err != nil {
					return err
				}
			}
			// Insert explanations (PL)
			for _, e := range in.OptionsExplanation.PL {
				ex := Explanation{
					QuestionID: q.ID,
					OptionKey:  stringsLower(e.ID),
					Lang:       "pl",
					Text:       e.Text,
					URL:        strings.TrimSpace(e.URL),
				}
				if err := tx.Create(&ex).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// stringsLower normalizes option ids like "A".."D" to lowercase.
func stringsLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
