package risk

import "testing"

func TestTVROUTE001ScoreBandBoundaries(t *testing.T) {
	tests := []struct {
		score    int
		expected Category
	}{
		{score: 0, expected: CategoryLow},
		{score: 24, expected: CategoryLow},
		{score: 25, expected: CategoryMedium},
		{score: 49, expected: CategoryMedium},
		{score: 50, expected: CategoryHigh},
		{score: 74, expected: CategoryHigh},
		{score: 75, expected: CategoryCritical},
		{score: 100, expected: CategoryCritical},
	}

	for _, tc := range tests {
		got, err := categoryForScore(tc.score)
		if err != nil {
			t.Fatalf("categoryForScore(%d) returned error: %v", tc.score, err)
		}
		if got != tc.expected {
			t.Fatalf("categoryForScore(%d)\nwant: %s\n got: %s", tc.score, tc.expected, got)
		}
	}

	for _, invalid := range []int{-1, 101} {
		if _, err := categoryForScore(invalid); err == nil {
			t.Fatalf("categoryForScore(%d) expected error, got nil", invalid)
		}
	}
}

func TestTVROUTE002ConfidenceEscalationBehavior(t *testing.T) {
	s := NewScorer(0.70)

	tests := []struct {
		name       string
		detections int
		confidence float64
		expected   Category
	}{
		{name: "low escalates to medium", detections: 0, confidence: 0.69, expected: CategoryMedium},
		{name: "medium escalates to high", detections: 1, confidence: 0.69, expected: CategoryHigh},
		{name: "high escalates to critical", detections: 2, confidence: 0.69, expected: CategoryCritical},
		{name: "critical remains critical", detections: 4, confidence: 0.10, expected: CategoryCritical},
		{name: "equal threshold does not escalate", detections: 1, confidence: 0.70, expected: CategoryMedium},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := s.Evaluate(tc.detections, tc.confidence)
			if err != nil {
				t.Fatalf("Evaluate returned error: %v", err)
			}
			if result.Category != tc.expected {
				t.Fatalf("Evaluate(%d, %.2f)\nwant: %s\n got: %s", tc.detections, tc.confidence, tc.expected, result.Category)
			}
		})
	}
}
