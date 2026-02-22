package risk

import "fmt"

type Category string

const (
	CategoryLow      Category = "Low"
	CategoryMedium   Category = "Medium"
	CategoryHigh     Category = "High"
	CategoryCritical Category = "Critical"
)

type Result struct {
	Score      int
	Category   Category
	Confidence float64
}

type Scorer struct {
	confidenceThreshold float64
}

func NewScorer(confidenceThreshold float64) *Scorer {
	if confidenceThreshold <= 0 {
		confidenceThreshold = 0.70
	}
	return &Scorer{confidenceThreshold: confidenceThreshold}
}

func (s *Scorer) Evaluate(detections int, confidence float64) (Result, error) {
	score := detections * 25
	if score > 100 {
		score = 100
	}

	baseCategory, err := categoryForScore(score)
	if err != nil {
		return Result{}, err
	}

	finalCategory := baseCategory
	if confidence < s.confidenceThreshold {
		finalCategory = escalate(baseCategory)
	}

	return Result{
		Score:      score,
		Category:   finalCategory,
		Confidence: confidence,
	}, nil
}

func categoryForScore(score int) (Category, error) {
	switch {
	case score >= 0 && score <= 24:
		return CategoryLow, nil
	case score >= 25 && score <= 49:
		return CategoryMedium, nil
	case score >= 50 && score <= 74:
		return CategoryHigh, nil
	case score >= 75 && score <= 100:
		return CategoryCritical, nil
	default:
		return "", fmt.Errorf("invalid score: %d", score)
	}
}

func escalate(category Category) Category {
	switch category {
	case CategoryLow:
		return CategoryMedium
	case CategoryMedium:
		return CategoryHigh
	case CategoryHigh:
		return CategoryCritical
	default:
		return CategoryCritical
	}
}
