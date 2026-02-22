package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestChainWriterAppendIncludesChainFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	cw, err := NewChainWriter(path)
	if err != nil {
		t.Fatalf("NewChainWriter failed: %v", err)
	}

	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	cw.now = func() time.Time { return now }

	first, err := cw.Append(Event{
		RequestID:     "req-1",
		PolicyVersion: "v2.1",
		ActionSummary: "first",
		RiskCategory:  "Medium",
		Route:         "sanitized_forward",
	})
	if err != nil {
		t.Fatalf("Append first failed: %v", err)
	}
	if first.PrevHash != "" {
		t.Fatalf("expected empty prev hash for first record, got %q", first.PrevHash)
	}
	if first.EntryHash == "" {
		t.Fatal("expected entry hash for first record")
	}

	second, err := cw.Append(Event{
		RequestID:     "req-2",
		PolicyVersion: "v2.1",
		ActionSummary: "second",
		RiskCategory:  "High",
		Route:         "high_abstraction",
	})
	if err != nil {
		t.Fatalf("Append second failed: %v", err)
	}
	if second.PrevHash != first.EntryHash {
		t.Fatalf("expected second prev hash %q, got %q", first.EntryHash, second.PrevHash)
	}
	if second.EntryHash == "" {
		t.Fatal("expected entry hash for second record")
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}

	var parsed Record
	if err := json.Unmarshal([]byte(lines[1]), &parsed); err != nil {
		t.Fatalf("unmarshal second record failed: %v", err)
	}
	if parsed.PrevHash != first.EntryHash {
		t.Fatalf("parsed second prev hash mismatch: %q", parsed.PrevHash)
	}
}

func TestVerifyChainPassesForUntamperedLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	cw, err := NewChainWriter(path)
	if err != nil {
		t.Fatalf("NewChainWriter failed: %v", err)
	}

	if _, err := cw.Append(Event{
		RequestID:     "req-1",
		PolicyVersion: "v2.1",
		ActionSummary: "first",
		RiskCategory:  "Medium",
		Route:         "sanitized_forward",
	}); err != nil {
		t.Fatalf("Append first failed: %v", err)
	}

	if _, err := cw.Append(Event{
		RequestID:     "req-2",
		PolicyVersion: "v2.1",
		ActionSummary: "second",
		RiskCategory:  "High",
		Route:         "high_abstraction",
	}); err != nil {
		t.Fatalf("Append second failed: %v", err)
	}

	if err := VerifyChain(path); err != nil {
		t.Fatalf("VerifyChain failed on untampered log: %v", err)
	}
}

func TestVerifyChainDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	cw, err := NewChainWriter(path)
	if err != nil {
		t.Fatalf("NewChainWriter failed: %v", err)
	}

	if _, err := cw.Append(Event{
		RequestID:     "req-1",
		PolicyVersion: "v2.1",
		ActionSummary: "first",
		RiskCategory:  "Medium",
		Route:         "sanitized_forward",
	}); err != nil {
		t.Fatalf("Append first failed: %v", err)
	}

	if _, err := cw.Append(Event{
		RequestID:     "req-2",
		PolicyVersion: "v2.1",
		ActionSummary: "second",
		RiskCategory:  "High",
		Route:         "high_abstraction",
	}); err != nil {
		t.Fatalf("Append second failed: %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log failed: %v", err)
	}
	modified := strings.Replace(string(contents), `"action_summary":"second"`, `"action_summary":"tampered"`, 1)
	if modified == string(contents) {
		t.Fatal("failed to tamper with audit content")
	}
	if err := os.WriteFile(path, []byte(modified), 0o600); err != nil {
		t.Fatalf("write tampered audit log failed: %v", err)
	}

	if err := VerifyChain(path); err == nil {
		t.Fatal("expected VerifyChain to fail for tampered log")
	}
}
