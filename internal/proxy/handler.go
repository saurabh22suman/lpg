package proxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/soloengine/lpg/internal/audit"
	"github.com/soloengine/lpg/internal/risk"
	"github.com/soloengine/lpg/internal/router"
	"github.com/soloengine/lpg/internal/sanitizer"
)

const defaultProviderTimeout = 2 * time.Second

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

type ExplainMapping struct {
	Placeholder string  `json:"placeholder"`
	EntityType  string  `json:"entity_type"`
	Confidence  float64 `json:"confidence"`
}

type ExplainResponse struct {
	RequestID      string           `json:"request_id"`
	PolicyVersion  string           `json:"policy_version"`
	Model          string           `json:"model"`
	SanitizedInput string           `json:"sanitized_input"`
	Detections     int              `json:"detections"`
	MinConfidence  float64          `json:"min_confidence"`
	RiskScore      int              `json:"risk_score"`
	RiskCategory   risk.Category    `json:"risk_category"`
	Route          router.Route     `json:"route"`
	Egress         bool             `json:"egress"`
	HardBlock      bool             `json:"hard_block"`
	Mappings       []ExplainMapping `json:"mappings"`
}

type ForwardRequest struct {
	RequestID       string
	Model           string
	SanitizedPrompt string
	RiskCategory    risk.Category
	Route           router.Route
	IdempotencyKey  string
}

type ForwardResponse struct {
	Content string
}

type UpstreamAdapter interface {
	ChatCompletions(ctx context.Context, req ForwardRequest) (ForwardResponse, error)
}

type AbstractRequest struct {
	RequestID       string
	SanitizedPrompt string
	Mappings        []sanitizer.Mapping
	Route           router.Route
}

type Abstractor interface {
	Abstract(ctx context.Context, req AbstractRequest) (string, error)
}

type Sanitizer interface {
	Sanitize(input string) (sanitizer.Result, error)
}

type AuditWriter interface {
	Append(event audit.Event) (audit.Record, error)
}

type HandlerConfig struct {
	Sanitizer       Sanitizer
	Scorer          *risk.Scorer
	Router          *router.Engine
	Upstream        UpstreamAdapter
	Abstractor      Abstractor
	Audit           AuditWriter
	ProviderTimeout time.Duration
	PolicyVersion   string
	StrictAudit     bool
}

type Handler struct {
	sanitizer       Sanitizer
	scorer          *risk.Scorer
	router          *router.Engine
	upstream        UpstreamAdapter
	abstractor      Abstractor
	audit           AuditWriter
	providerTimeout time.Duration
	policyVersion   string
	strictAudit     bool
}

func NewHandler(cfg HandlerConfig) *Handler {
	h := &Handler{
		sanitizer:       cfg.Sanitizer,
		scorer:          cfg.Scorer,
		router:          cfg.Router,
		upstream:        cfg.Upstream,
		abstractor:      cfg.Abstractor,
		audit:           cfg.Audit,
		providerTimeout: cfg.ProviderTimeout,
		policyVersion:   cfg.PolicyVersion,
		strictAudit:     cfg.StrictAudit,
	}
	if h.sanitizer == nil {
		h.sanitizer = sanitizer.NewDefault()
	}
	if h.scorer == nil {
		h.scorer = risk.NewScorer(0.70)
	}
	if h.router == nil {
		h.router = router.NewEngine(false)
	}
	if h.providerTimeout == 0 {
		h.providerTimeout = defaultProviderTimeout
	}
	if h.policyVersion == "" {
		h.policyVersion = "v2.1-phase1"
	}
	return h
}

func (h *Handler) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	requestID := newRequestID()
	w.Header().Set("x-lpg-request-id", requestID)
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "ERR_METHOD_NOT_ALLOWED", "method not allowed", requestID)
		return
	}

	req, rawPrompt, sanitized, _, _, decision, err := h.analyzeChatRequest(w, r, requestID, true)
	if err != nil {
		return
	}

	summary := fmt.Sprintf("route=%s category=%s", decision.Route, decision.Category)
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))

	switch decision.Route {
	case router.RouteRawForward, router.RouteSanitizedForward:
		if h.upstream == nil {
			h.writeError(w, http.StatusBadGateway, "ERR_PROVIDER_FAILURE", "upstream adapter not configured", requestID)
			_ = h.appendAudit(requestID, decision.Category, decision.Route, summary+" upstream-missing")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), h.providerTimeout)
		defer cancel()

		promptForRoute := sanitized.Sanitized
		if decision.Route == router.RouteRawForward {
			promptForRoute = rawPrompt
		}

		forwardReq := ForwardRequest{
			RequestID:       requestID,
			Model:           req.Model,
			SanitizedPrompt: promptForRoute,
			RiskCategory:    decision.Category,
			Route:           decision.Route,
			IdempotencyKey:  idempotencyKey,
		}

		resp, err := h.upstream.ChatCompletions(ctx, forwardReq)
		if err != nil && idempotencyKey != "" && (decision.Category == risk.CategoryLow || decision.Category == risk.CategoryMedium) {
			resp, err = h.upstream.ChatCompletions(ctx, forwardReq)
		}
		if err != nil {
			if isTimeout(err) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				h.writeError(w, http.StatusServiceUnavailable, "ERR_PROVIDER_TIMEOUT", "provider timeout", requestID)
				_ = h.appendAudit(requestID, decision.Category, decision.Route, summary+" provider-timeout")
				return
			}
			h.writeError(w, http.StatusBadGateway, "ERR_PROVIDER_FAILURE", "provider request failed", requestID)
			auditSummary := summary + " provider-failure"
			if diagnostic := safeProviderDiagnostic(err); diagnostic != "" {
				auditSummary += " " + diagnostic
			}
			_ = h.appendAudit(requestID, decision.Category, decision.Route, auditSummary)
			return
		}

		if err := h.appendAudit(requestID, decision.Category, decision.Route, summary+" success"); err != nil {
			h.writeError(w, http.StatusInternalServerError, "ERR_AUDIT_FAILURE", "audit append failed", requestID)
			return
		}

		h.writeSuccess(w, requestID, req.Model, resp.Content)
	case router.RouteHighAbstraction:
		abstraction, err := h.requireAbstraction(r.Context(), w, requestID, sanitized, decision, summary)
		if err != nil {
			return
		}

		if h.upstream == nil {
			h.writeError(w, http.StatusBadGateway, "ERR_PROVIDER_FAILURE", "upstream adapter not configured", requestID)
			_ = h.appendAudit(requestID, decision.Category, decision.Route, summary+" upstream-missing")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), h.providerTimeout)
		defer cancel()

		resp, err := h.upstream.ChatCompletions(ctx, ForwardRequest{
			RequestID:       requestID,
			Model:           req.Model,
			SanitizedPrompt: abstraction,
			RiskCategory:    decision.Category,
			Route:           decision.Route,
			IdempotencyKey:  idempotencyKey,
		})
		if err != nil {
			if isTimeout(err) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				h.writeError(w, http.StatusServiceUnavailable, "ERR_PROVIDER_TIMEOUT", "provider timeout", requestID)
				_ = h.appendAudit(requestID, decision.Category, decision.Route, summary+" provider-timeout")
				return
			}
			h.writeError(w, http.StatusBadGateway, "ERR_PROVIDER_FAILURE", "provider request failed", requestID)
			auditSummary := summary + " provider-failure"
			if diagnostic := safeProviderDiagnostic(err); diagnostic != "" {
				auditSummary += " " + diagnostic
			}
			_ = h.appendAudit(requestID, decision.Category, decision.Route, auditSummary)
			return
		}

		if err := h.appendAudit(requestID, decision.Category, decision.Route, summary+" success"); err != nil {
			h.writeError(w, http.StatusInternalServerError, "ERR_AUDIT_FAILURE", "audit append failed", requestID)
			return
		}

		h.writeSuccess(w, requestID, req.Model, resp.Content)
	case router.RouteCriticalLocalOnly:
		abstraction, err := h.requireAbstraction(r.Context(), w, requestID, sanitized, decision, summary)
		if err != nil {
			return
		}

		if err := h.appendAudit(requestID, decision.Category, decision.Route, summary+" local-only-success"); err != nil {
			h.writeError(w, http.StatusInternalServerError, "ERR_AUDIT_FAILURE", "audit append failed", requestID)
			return
		}

		h.writeSuccess(w, requestID, req.Model, abstraction)
	default:
		h.writeError(w, http.StatusForbidden, "ERR_POLICY_BLOCK", "request blocked by policy", requestID)
		_ = h.appendAudit(requestID, risk.CategoryCritical, router.RouteCriticalBlocked, summary+" blocked")
	}
}

func (h *Handler) HandleDebugExplain(w http.ResponseWriter, r *http.Request) {
	requestID := newRequestID()
	w.Header().Set("x-lpg-request-id", requestID)
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "ERR_METHOD_NOT_ALLOWED", "method not allowed", requestID)
		return
	}

	req, _, sanitized, result, hasHardBlock, decision, err := h.analyzeChatRequest(w, r, requestID, false)
	if err != nil {
		return
	}

	mappings := make([]ExplainMapping, 0, len(sanitized.Mappings))
	for _, mapping := range sanitized.Mappings {
		mappings = append(mappings, ExplainMapping{
			Placeholder: mapping.Placeholder,
			EntityType:  mapping.EntityType,
			Confidence:  mapping.ConfidenceScore,
		})
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ExplainResponse{
		RequestID:      requestID,
		PolicyVersion:  h.policyVersion,
		Model:          req.Model,
		SanitizedInput: sanitized.Sanitized,
		Detections:     len(sanitized.Mappings),
		MinConfidence:  minMappingConfidence(sanitized.Mappings),
		RiskScore:      result.Score,
		RiskCategory:   result.Category,
		Route:          decision.Route,
		Egress:         decision.Egress,
		HardBlock:      hasHardBlock,
		Mappings:       mappings,
	})
}

func (h *Handler) analyzeChatRequest(w http.ResponseWriter, r *http.Request, requestID string, auditFailures bool) (ChatCompletionRequest, string, sanitizer.Result, risk.Result, bool, router.Decision, error) {
	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "invalid JSON payload", requestID)
		return ChatCompletionRequest{}, "", sanitizer.Result{}, risk.Result{}, false, router.Decision{}, err
	}

	if err := validateRequest(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "ERR_VALIDATION", err.Error(), requestID)
		return ChatCompletionRequest{}, "", sanitizer.Result{}, risk.Result{}, false, router.Decision{}, err
	}

	rawPrompt := joinPrompt(req.Messages)
	sanitized, err := h.sanitizer.Sanitize(rawPrompt)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "ERR_SANITIZATION_FAILURE", "sanitization failed", requestID)
		return ChatCompletionRequest{}, "", sanitizer.Result{}, risk.Result{}, false, router.Decision{}, err
	}

	result, err := h.scorer.Evaluate(len(sanitized.Mappings), minMappingConfidence(sanitized.Mappings))
	if err != nil {
		h.writeError(w, http.StatusForbidden, "ERR_POLICY_BLOCK", "risk evaluation failed", requestID)
		if auditFailures {
			_ = h.appendAudit(requestID, risk.CategoryCritical, router.RouteCriticalBlocked, "risk evaluation failed")
		}
		return ChatCompletionRequest{}, "", sanitizer.Result{}, risk.Result{}, false, router.Decision{}, err
	}

	hasHardBlock := false
	for _, m := range sanitized.Mappings {
		if m.EntityType == "SSN" {
			hasHardBlock = true
			break
		}
	}

	decision := h.router.Decide(result.Category, hasHardBlock)
	return req, rawPrompt, sanitized, result, hasHardBlock, decision, nil
}

func (h *Handler) requireAbstraction(ctx context.Context, w http.ResponseWriter, requestID string, sanitized sanitizer.Result, decision router.Decision, summary string) (string, error) {
	if h.abstractor == nil {
		h.writeError(w, http.StatusServiceUnavailable, "ERR_ABSTRACTION_UNAVAILABLE", "local abstraction is not enabled", requestID)
		_ = h.appendAudit(requestID, decision.Category, decision.Route, summary+" abstraction-unavailable")
		return "", errors.New("abstraction unavailable")
	}

	abstraction, err := h.abstractor.Abstract(ctx, AbstractRequest{
		RequestID:       requestID,
		SanitizedPrompt: sanitized.Sanitized,
		Mappings:        sanitized.Mappings,
		Route:           decision.Route,
	})
	if err != nil {
		h.writeError(w, http.StatusServiceUnavailable, "ERR_ABSTRACTION_UNAVAILABLE", "local abstraction failed", requestID)
		_ = h.appendAudit(requestID, decision.Category, decision.Route, summary+" abstraction-failed")
		return "", err
	}
	return abstraction, nil
}

func (h *Handler) appendAudit(requestID string, category risk.Category, route router.Route, actionSummary string) error {
	if h.audit == nil {
		return nil
	}
	_, err := h.audit.Append(audit.Event{
		RequestID:     requestID,
		PolicyVersion: h.policyVersion,
		ActionSummary: actionSummary,
		RiskCategory:  string(category),
		Route:         string(route),
	})
	if err != nil && !h.strictAudit {
		return nil
	}
	return err
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message, requestID string) {
	w.WriteHeader(status)
	resp := errorResponse{RequestID: requestID}
	resp.Error.Code = code
	resp.Error.Message = message
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) writeSuccess(w http.ResponseWriter, requestID, model, content string) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ChatCompletionResponse{
		ID:     requestID,
		Object: "chat.completion",
		Model:  model,
		Choices: []chatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
	})
}

func validateRequest(req ChatCompletionRequest) error {
	if strings.TrimSpace(req.Model) == "" {
		return errors.New("model is required")
	}
	if len(req.Messages) == 0 {
		return errors.New("at least one message is required")
	}
	for i, m := range req.Messages {
		if strings.TrimSpace(m.Role) == "" {
			return fmt.Errorf("messages[%d].role is required", i)
		}
		if strings.TrimSpace(m.Content) == "" {
			return fmt.Errorf("messages[%d].content is required", i)
		}
	}
	return nil
}

func joinPrompt(messages []ChatMessage) string {
	parts := make([]string, 0, len(messages))
	for _, m := range messages {
		parts = append(parts, m.Content)
	}
	return strings.Join(parts, "\n")
}

func minMappingConfidence(mappings []sanitizer.Mapping) float64 {
	if len(mappings) == 0 {
		return 0.99
	}
	min := mappings[0].ConfidenceScore
	for _, m := range mappings[1:] {
		if m.ConfidenceScore < min {
			min = m.ConfidenceScore
		}
	}
	return min
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return "req-" + hex.EncodeToString(b)
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
