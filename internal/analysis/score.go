package analysis

import "fmt"

func CalculateScore(flags []RiskFlagDetail) DDScore {
	score := 100
	for _, f := range flags {
		score -= f.Points
	}
	if score < 0 {
		score = 0
	}

	grade := "A"
	switch {
	case score >= 90:
		grade = "A"
	case score >= 75:
		grade = "B"
	case score >= 60:
		grade = "C"
	case score >= 40:
		grade = "D"
	default:
		grade = "F"
	}

	summary := summarize(grade, flags)
	return DDScore{Score: score, Grade: grade, Summary: summary}
}

func summarize(grade string, flags []RiskFlagDetail) string {
	highCount := 0
	for _, f := range flags {
		if f.Severity == "HIGH" {
			highCount++
		}
	}

	switch {
	case len(flags) == 0:
		return "No significant risk flags detected"
	case highCount >= 3:
		return fmt.Sprintf("Extreme risk: %d critical flags — heavy dilution likely", highCount)
	case highCount >= 2:
		return fmt.Sprintf("High risk: %d critical flags — exercise caution", highCount)
	case highCount == 1:
		return fmt.Sprintf("Moderate risk: 1 critical flag and %d warnings", len(flags)-1)
	default:
		return fmt.Sprintf("Low-moderate risk: %d warning flags", len(flags))
	}
}
