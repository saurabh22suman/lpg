package router

import "github.com/soloengine/lpg/internal/risk"

type Route string

const (
	RouteRawForward        Route = "raw_forward"
	RouteSanitizedForward  Route = "sanitized_forward"
	RouteHighAbstraction   Route = "high_abstraction"
	RouteCriticalLocalOnly Route = "critical_local_only"
	RouteCriticalBlocked   Route = "critical_blocked"
)

type Decision struct {
	Category risk.Category
	Route    Route
	Egress   bool
}

type Engine struct {
	allowRawForwarding bool
	criticalLocalOnly  bool
}

func NewEngine(allowRawForwarding bool) *Engine {
	return &Engine{allowRawForwarding: allowRawForwarding}
}

func NewEngineWithCriticalLocalOnly(allowRawForwarding, criticalLocalOnly bool) *Engine {
	return &Engine{
		allowRawForwarding: allowRawForwarding,
		criticalLocalOnly:  criticalLocalOnly,
	}
}

func (e *Engine) Decide(category risk.Category, hasHardBlock bool) Decision {
	switch category {
	case risk.CategoryLow:
		if e.allowRawForwarding && !hasHardBlock {
			return Decision{Category: category, Route: RouteRawForward, Egress: true}
		}
		return Decision{Category: category, Route: RouteSanitizedForward, Egress: true}
	case risk.CategoryMedium:
		return Decision{Category: category, Route: RouteSanitizedForward, Egress: true}
	case risk.CategoryHigh:
		return Decision{Category: category, Route: RouteHighAbstraction, Egress: true}
	default:
		if e.criticalLocalOnly {
			return Decision{Category: risk.CategoryCritical, Route: RouteCriticalLocalOnly, Egress: false}
		}
		return Decision{Category: risk.CategoryCritical, Route: RouteCriticalBlocked, Egress: false}
	}
}
