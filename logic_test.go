package main

import "testing"

func TestIsCorrectAllOrNothing(t *testing.T) {
    tests := []struct {
        name     string
        selected []string
        correct  []string
        want     bool
    }{
        {
            name:     "exact match",
            selected: []string{"a", "b"},
            correct:  []string{"a", "b"},
            want:     true,
        },
        {
            name:     "missing option",
            selected: []string{"a"},
            correct:  []string{"a", "b"},
            want:     false,
        },
        {
            name:     "extra option",
            selected: []string{"a", "b", "c"},
            correct:  []string{"a", "b"},
            want:     false,
        },
        {
            name:     "duplicate option",
            selected: []string{"a", "a"},
            correct:  []string{"a", "b"},
            want:     false,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := isCorrectAllOrNothing(tt.selected, tt.correct); got != tt.want {
                t.Errorf("isCorrectAllOrNothing() = %v, want %v", got, tt.want)
            }
        })
    }
}

