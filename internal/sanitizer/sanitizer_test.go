package sanitizer

import "testing"

func TestTVDET001MappingEmissionAndDeterminism(t *testing.T) {
	s := NewDefault()
	input := "Email alice@example.com and bob@example.com call 555-123-4567"

	first, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("sanitize first run failed: %v", err)
	}

	second, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("sanitize second run failed: %v", err)
	}

	expectedSanitized := "Email person1@example.net and person2@example.net call 555-010-0001"
	if first.Sanitized != expectedSanitized {
		t.Fatalf("unexpected sanitized text\nwant: %q\n got: %q", expectedSanitized, first.Sanitized)
	}

	if first.Sanitized != second.Sanitized {
		t.Fatalf("sanitization not deterministic\nfirst:  %q\nsecond: %q", first.Sanitized, second.Sanitized)
	}

	if len(first.Mappings) != 3 {
		t.Fatalf("expected 3 mappings, got %d", len(first.Mappings))
	}

	for i, m := range first.Mappings {
		if m.Placeholder == "" {
			t.Fatalf("mapping[%d] placeholder is empty", i)
		}
		if m.OriginalValue == "" {
			t.Fatalf("mapping[%d] original_value is empty", i)
		}
		if m.EntityType == "" {
			t.Fatalf("mapping[%d] entity_type is empty", i)
		}
		if m.ConfidenceScore <= 0 {
			t.Fatalf("mapping[%d] confidence_score is invalid: %f", i, m.ConfidenceScore)
		}
		if contains(first.Sanitized, m.OriginalValue) {
			t.Fatalf("sanitized payload leaked original value %q", m.OriginalValue)
		}
	}
}

func contains(s, needle string) bool {
	return len(needle) > 0 && len(s) >= len(needle) && indexOf(s, needle) >= 0
}

func indexOf(s, needle string) int {
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
