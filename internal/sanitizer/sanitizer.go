package sanitizer

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
)

type Mapping struct {
	Placeholder     string  `json:"placeholder"`
	OriginalValue   string  `json:"original_value"`
	EntityType      string  `json:"entity_type"`
	ConfidenceScore float64 `json:"confidence_score"`
}

type Result struct {
	Sanitized string
	Mappings  []Mapping
}

type Rule struct {
	EntityType string
	Regex      *regexp.Regexp
	Confidence float64
}

type match struct {
	start int
	end   int
	value string
	rule  Rule
}

type Sanitizer struct {
	rules []Rule
}

func NewDefault() *Sanitizer {
	return &Sanitizer{
		rules: []Rule{
			{EntityType: "EMAIL", Regex: regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`), Confidence: 0.99},
			{EntityType: "PHONE", Regex: regexp.MustCompile(`\b\d{3}-\d{3}-\d{4}\b`), Confidence: 0.99},
			{EntityType: "SSN", Regex: regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`), Confidence: 0.99},
		},
	}
}

func surrogateForEntity(entityType string, index int) string {
	suffix := strconv.Itoa(index)
	switch entityType {
	case "EMAIL":
		return "person" + suffix + "@example.net"
	case "PHONE":
		return "555-010-" + fmt.Sprintf("%04d", index)
	case "SSN":
		return "900-00-" + fmt.Sprintf("%04d", index)
	default:
		return "redacted-" + suffix
	}
}

func (s *Sanitizer) Sanitize(input string) (Result, error) {
	matches := make([]match, 0)

	for _, rule := range s.rules {
		indexes := rule.Regex.FindAllStringIndex(input, -1)
		for _, idx := range indexes {
			matches = append(matches, match{
				start: idx[0],
				end:   idx[1],
				value: input[idx[0]:idx[1]],
				rule:  rule,
			})
		}
	}

	if len(matches) == 0 {
		return Result{Sanitized: input}, nil
	}

	sort.SliceStable(matches, func(i, j int) bool {
		lenI := matches[i].end - matches[i].start
		lenJ := matches[j].end - matches[j].start
		if lenI != lenJ {
			return lenI > lenJ
		}
		return matches[i].start < matches[j].start
	})

	accepted := make([]match, 0, len(matches))
	occupied := make([]bool, len(input))
	for _, m := range matches {
		overlap := false
		for i := m.start; i < m.end; i++ {
			if occupied[i] {
				overlap = true
				break
			}
		}
		if overlap {
			continue
		}
		for i := m.start; i < m.end; i++ {
			occupied[i] = true
		}
		accepted = append(accepted, m)
	}

	sort.SliceStable(accepted, func(i, j int) bool {
		return accepted[i].start < accepted[j].start
	})

	counts := map[string]int{}
	surrogateByEntityAndValue := map[string]map[string]string{}
	replacements := make([]struct {
		start       int
		end         int
		placeholder string
		mapping     Mapping
	}, 0, len(accepted))

	for _, m := range accepted {
		byValue, ok := surrogateByEntityAndValue[m.rule.EntityType]
		if !ok {
			byValue = map[string]string{}
			surrogateByEntityAndValue[m.rule.EntityType] = byValue
		}
		surrogate := byValue[m.value]
		if surrogate == "" {
			counts[m.rule.EntityType]++
			surrogate = surrogateForEntity(m.rule.EntityType, counts[m.rule.EntityType])
			byValue[m.value] = surrogate
		}

		replacements = append(replacements, struct {
			start       int
			end         int
			placeholder string
			mapping     Mapping
		}{
			start:       m.start,
			end:         m.end,
			placeholder: surrogate,
			mapping: Mapping{
				Placeholder:     surrogate,
				OriginalValue:   m.value,
				EntityType:      m.rule.EntityType,
				ConfidenceScore: m.rule.Confidence,
			},
		})
	}

	output := make([]byte, 0, len(input)+len(replacements)*8)
	cursor := 0
	mappings := make([]Mapping, 0, len(replacements))
	for _, r := range replacements {
		if cursor > r.start {
			return Result{}, fmt.Errorf("overlap resolution failure")
		}
		output = append(output, input[cursor:r.start]...)
		output = append(output, r.placeholder...)
		cursor = r.end
		mappings = append(mappings, r.mapping)
	}
	output = append(output, input[cursor:]...)

	return Result{Sanitized: string(output), Mappings: mappings}, nil
}
