package analysis

import "testing"

func TestCalculateScore_NoFlags(t *testing.T) {
	score := CalculateScore(nil)
	if score.Score != 100 {
		t.Errorf("Score = %d, want 100", score.Score)
	}
	if score.Grade != "A" {
		t.Errorf("Grade = %q, want A", score.Grade)
	}
}

func TestCalculateScore_OneHighFlag(t *testing.T) {
	flags := []RiskFlagDetail{
		{Flag: FlagHighDilution, Severity: "HIGH", Points: 25},
	}
	score := CalculateScore(flags)
	if score.Score != 75 {
		t.Errorf("Score = %d, want 75", score.Score)
	}
	if score.Grade != "B" {
		t.Errorf("Grade = %q, want B", score.Grade)
	}
}

func TestCalculateScore_MultipleFlags(t *testing.T) {
	flags := []RiskFlagDetail{
		{Flag: FlagHighDilution, Severity: "HIGH", Points: 25},
		{Flag: FlagRecentATM, Severity: "HIGH", Points: 20},
		{Flag: FlagShelfReg, Severity: "MEDIUM", Points: 10},
	}
	score := CalculateScore(flags)
	if score.Score != 45 {
		t.Errorf("Score = %d, want 45", score.Score)
	}
	if score.Grade != "D" {
		t.Errorf("Grade = %q, want D", score.Grade)
	}
}

func TestCalculateScore_FloorAtZero(t *testing.T) {
	flags := []RiskFlagDetail{
		{Points: 50},
		{Points: 40},
		{Points: 30},
	}
	score := CalculateScore(flags)
	if score.Score != 0 {
		t.Errorf("Score = %d, want 0 (floored)", score.Score)
	}
	if score.Grade != "F" {
		t.Errorf("Grade = %q, want F", score.Grade)
	}
}

func TestCalculateScore_Grades(t *testing.T) {
	tests := []struct {
		points int
		grade  string
	}{
		{0, "A"},   // 100
		{5, "A"},   // 95
		{10, "A"},  // 90
		{15, "B"},  // 85
		{25, "B"},  // 75
		{30, "C"},  // 70
		{40, "C"},  // 60
		{45, "D"},  // 55
		{60, "D"},  // 40
		{65, "F"},  // 35
		{100, "F"}, // 0
	}

	for _, tt := range tests {
		flags := []RiskFlagDetail{{Points: tt.points}}
		score := CalculateScore(flags)
		if score.Grade != tt.grade {
			t.Errorf("points=%d: Grade = %q, want %q (score=%d)", tt.points, score.Grade, tt.grade, score.Score)
		}
	}
}
