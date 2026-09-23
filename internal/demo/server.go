package demo

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/batch"
)

const maxRequestBytes = 8 << 10

//go:embed index.html
var indexHTML string

type AuditFunc func(ctx context.Context, repository string, plan bool) (batch.Report, error)

type Server struct {
	matcher     attribution.Matcher
	maxInFlight int
	timeout     time.Duration
	sem         chan struct{}
	audit       AuditFunc
	mux         *http.ServeMux
}

type auditRequest struct {
	Repository string `json:"repository"`
	Mode       string `json:"mode,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func New(matcher attribution.Matcher, maxInFlight int, timeout time.Duration) (*Server, error) {
	if matcher == nil {
		return nil, fmt.Errorf("attribution matcher is required")
	}
	if maxInFlight < 1 || maxInFlight > 32 {
		return nil, fmt.Errorf("max in-flight audits must be between 1 and 32")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("audit timeout must be positive")
	}

	s := &Server{
		matcher:     matcher,
		maxInFlight: maxInFlight,
		timeout:     timeout,
		sem:         make(chan struct{}, maxInFlight),
	}
	s.audit = s.runAudit
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("POST /v1/audits", s.handleAudit)
	return s, nil
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "byeclaude-demo",
	})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request auditRequest
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON request"})
		return
	}

	repository, err := normalizeGitHubRepository(request.Repository)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	if mode == "" {
		mode = "scan"
	}
	if mode != "scan" && mode != "plan" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "mode must be scan or plan"})
		return
	}

	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	default:
		w.Header().Set("Retry-After", "2")
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "audit capacity is currently full"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	report, err := s.audit(ctx, repository, mode == "plan")
	if err != nil {
		if ctx.Err() != nil {
			writeJSON(w, http.StatusGatewayTimeout, errorResponse{Error: "audit timed out"})
			return
		}
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "repository audit failed"})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) runAudit(ctx context.Context, repository string, plan bool) (batch.Report, error) {
	spec := batch.Spec{
		Name:       repository,
		Source:     "https://github.com/" + repository + ".git",
		Visibility: "public",
	}
	report, err := batch.Run(ctx, []batch.Spec{spec}, batch.Options{
		Jobs:               1,
		Matcher:            s.matcher,
		IncludeRemotes:     true,
		Selection:          "public-demo:" + repository,
		Plan:               plan,
		DisableCredentials: true,
	})
	if err != nil {
		return report, err
	}
	if report.FailedRepositories != 0 {
		return report, fmt.Errorf("repository audit incomplete: %d repository scan(s) failed", report.FailedRepositories)
	}
	return report, nil
}

func normalizeGitHubRepository(value string) (string, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, ".git")
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("repository must be an owner/name GitHub slug")
	}
	for _, part := range parts {
		if len(part) < 1 || len(part) > 100 || part == "." || part == ".." {
			return "", fmt.Errorf("repository must be an owner/name GitHub slug")
		}
		for _, r := range part {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
				return "", fmt.Errorf("repository contains unsupported characters")
			}
		}
	}
	return parts[0] + "/" + parts[1], nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
