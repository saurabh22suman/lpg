package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type Event struct {
	RequestID     string
	PolicyVersion string
	ActionSummary string
	RiskCategory  string
	Route         string
}

type Record struct {
	Timestamp     time.Time `json:"timestamp"`
	RequestID     string    `json:"request_id"`
	PolicyVersion string    `json:"policy_version"`
	ActionSummary string    `json:"action_summary"`
	RiskCategory  string    `json:"risk_category"`
	Route         string    `json:"route"`
	PrevHash      string    `json:"prev_hash"`
	EntryHash     string    `json:"entry_hash"`
}

type ChainWriter struct {
	mu       sync.Mutex
	path     string
	prevHash string
	now      func() time.Time
}

func NewChainWriter(path string) (*ChainWriter, error) {
	cw := &ChainWriter{
		path: path,
		now:  time.Now,
	}

	if err := cw.loadPrevHash(); err != nil {
		return nil, err
	}

	return cw, nil
}

func (cw *ChainWriter) Append(event Event) (Record, error) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	record := Record{
		Timestamp:     cw.now().UTC(),
		RequestID:     event.RequestID,
		PolicyVersion: event.PolicyVersion,
		ActionSummary: event.ActionSummary,
		RiskCategory:  event.RiskCategory,
		Route:         event.Route,
		PrevHash:      cw.prevHash,
	}

	hash, err := entryHash(record)
	if err != nil {
		return Record{}, err
	}
	record.EntryHash = hash

	line, err := json.Marshal(record)
	if err != nil {
		return Record{}, err
	}

	f, err := os.OpenFile(cw.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Record{}, err
	}

	if _, err := f.Write(append(line, '\n')); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return Record{}, closeErr
		}
		return Record{}, err
	}
	if err := f.Close(); err != nil {
		return Record{}, err
	}

	cw.prevHash = record.EntryHash
	return record, nil
}

func (cw *ChainWriter) loadPrevHash() error {
	contents, err := os.ReadFile(cw.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	trimmed := strings.TrimSpace(string(contents))
	if trimmed == "" {
		return nil
	}

	lines := strings.Split(trimmed, "\n")
	lastLine := lines[len(lines)-1]
	var record Record
	if err := json.Unmarshal([]byte(lastLine), &record); err != nil {
		return err
	}
	cw.prevHash = record.EntryHash
	return nil
}

func entryHash(record Record) (string, error) {
	payload, err := json.Marshal(struct {
		Timestamp     time.Time `json:"timestamp"`
		RequestID     string    `json:"request_id"`
		PolicyVersion string    `json:"policy_version"`
		ActionSummary string    `json:"action_summary"`
		RiskCategory  string    `json:"risk_category"`
		Route         string    `json:"route"`
	}{
		Timestamp:     record.Timestamp,
		RequestID:     record.RequestID,
		PolicyVersion: record.PolicyVersion,
		ActionSummary: record.ActionSummary,
		RiskCategory:  record.RiskCategory,
		Route:         record.Route,
	})
	if err != nil {
		return "", err
	}

	h := sha256.New()
	if _, err := h.Write(payload); err != nil {
		return "", err
	}
	if _, err := h.Write([]byte(record.PrevHash)); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func VerifyChain(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	prev := ""
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return fmt.Errorf("invalid audit record at line %d: %w", lineNum, err)
		}

		if record.PrevHash != prev {
			return fmt.Errorf("invalid prev_hash at line %d", lineNum)
		}

		expected, err := entryHash(record)
		if err != nil {
			return fmt.Errorf("failed to compute entry hash at line %d: %w", lineNum, err)
		}
		if record.EntryHash != expected {
			return fmt.Errorf("invalid entry_hash at line %d", lineNum)
		}

		prev = record.EntryHash
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
