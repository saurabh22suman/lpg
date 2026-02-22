package router

import (
	"testing"

	"github.com/soloengine/lpg/internal/risk"
)

func TestEngineDecide(t *testing.T) {
	tests := []struct {
		name           string
		allowRaw       bool
		category       risk.Category
		hasHardBlock   bool
		expectedRoute  Route
		expectedEgress bool
		expectedCat    risk.Category
	}{
		{
			name:           "low with allow raw and no hard block",
			allowRaw:       true,
			category:       risk.CategoryLow,
			hasHardBlock:   false,
			expectedRoute:  RouteRawForward,
			expectedEgress: true,
			expectedCat:    risk.CategoryLow,
		},
		{
			name:           "low with hard block becomes sanitized",
			allowRaw:       true,
			category:       risk.CategoryLow,
			hasHardBlock:   true,
			expectedRoute:  RouteSanitizedForward,
			expectedEgress: true,
			expectedCat:    risk.CategoryLow,
		},
		{
			name:           "medium sanitized",
			allowRaw:       false,
			category:       risk.CategoryMedium,
			hasHardBlock:   false,
			expectedRoute:  RouteSanitizedForward,
			expectedEgress: true,
			expectedCat:    risk.CategoryMedium,
		},
		{
			name:           "high abstraction allows remote egress after local abstraction",
			allowRaw:       false,
			category:       risk.CategoryHigh,
			hasHardBlock:   false,
			expectedRoute:  RouteHighAbstraction,
			expectedEgress: true,
			expectedCat:    risk.CategoryHigh,
		},
		{
			name:           "critical blocked by default",
			allowRaw:       false,
			category:       risk.CategoryCritical,
			hasHardBlock:   false,
			expectedRoute:  RouteCriticalBlocked,
			expectedEgress: false,
			expectedCat:    risk.CategoryCritical,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEngine(tc.allowRaw)
			decision := e.Decide(tc.category, tc.hasHardBlock)
			if decision.Route != tc.expectedRoute {
				t.Fatalf("expected route %s, got %s", tc.expectedRoute, decision.Route)
			}
			if decision.Egress != tc.expectedEgress {
				t.Fatalf("expected egress %v, got %v", tc.expectedEgress, decision.Egress)
			}
			if decision.Category != tc.expectedCat {
				t.Fatalf("expected category %s, got %s", tc.expectedCat, decision.Category)
			}
		})
	}
}

func TestEngineWithCriticalLocalOnly(t *testing.T) {
	e := NewEngineWithCriticalLocalOnly(false, true)
	decision := e.Decide(risk.CategoryCritical, false)

	if decision.Route != RouteCriticalLocalOnly {
		t.Fatalf("expected route %s, got %s", RouteCriticalLocalOnly, decision.Route)
	}
	if decision.Egress {
		t.Fatal("expected no egress for critical local-only mode")
	}
	if decision.Category != risk.CategoryCritical {
		t.Fatalf("expected category %s, got %s", risk.CategoryCritical, decision.Category)
	}
}
