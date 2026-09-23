package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/batch"
	"github.com/IamAngusU/ByeClaude/internal/preset"
)

func newTestServer(t *testing.T, audit AuditFunc) *Server {
	t.Helper()
	server, err := New(preset.Claude(), 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	server.audit = audit
	return server
}

func TestHealth(t *testing.T) {
	server := newTestServer(t, func(context.Context, string, bool) (batch.Report, error) {
		t.Fatal("audit should not run")
		return batch.Report{}, nil
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d", res.Code)
	}
}

func TestAuditAcceptsOnlyGitHubSlug(t *testing.T) {
	var calls atomic.Int32
	server := newTestServer(t, func(_ context.Context, repo string, plan bool) (batch.Report, error) {
		calls.Add(1)
		if repo != "owner/repo" || plan {
			t.Fatalf("repo=%q plan=%v", repo, plan)
		}
		return batch.Report{Selection: repo, Scanned: 1}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/audits", strings.NewReader(`{"repository":"owner/repo","mode":"scan"}`))
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("status=%d calls=%d body=%s", res.Code, calls.Load(), res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/audits", strings.NewReader(`{"repository":"https://127.0.0.1/private"}`))
	res = httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest || calls.Load() != 1 {
		t.Fatalf("invalid target status=%d calls=%d", res.Code, calls.Load())
	}
}

func TestAuditPlanMode(t *testing.T) {
	server := newTestServer(t, func(_ context.Context, repo string, plan bool) (batch.Report, error) {
		if repo != "owner/repo" || !plan {
			t.Fatalf("repo=%q plan=%v", repo, plan)
		}
		return batch.Report{Operation: "plan", ObjectWritesEstimate: 42}, nil
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/audits", strings.NewReader(`{"repository":"owner/repo.git","mode":"plan"}`))
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	var report batch.Report
	if err := json.Unmarshal(res.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Operation != "plan" || report.ObjectWritesEstimate != 42 {
		t.Fatalf("report=%#v", report)
	}
}

func TestAuditCapacityReturns429(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := newTestServer(t, func(ctx context.Context, _ string, _ bool) (batch.Report, error) {
		close(started)
		select {
		case <-release:
			return batch.Report{}, nil
		case <-ctx.Done():
			return batch.Report{}, ctx.Err()
		}
	})

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		req := httptest.NewRequest(http.MethodPost, "/v1/audits", strings.NewReader(`{"repository":"owner/one"}`))
		res := httptest.NewRecorder()
		server.Handler().ServeHTTP(res, req)
	}()

	<-started
	req := httptest.NewRequest(http.MethodPost, "/v1/audits", strings.NewReader(`{"repository":"owner/two"}`))
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	close(release)
	<-firstDone
}


func TestAuditRepositoryFailureReturns502(t *testing.T) {
	server := newTestServer(t, func(context.Context, string, bool) (batch.Report, error) {
		return batch.Report{}, fmt.Errorf("clone failed")
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/audits", strings.NewReader(`{"repository":"owner/repo"}`))
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}


func TestAuditTimeoutReturns504(t *testing.T) {
	server, err := New(preset.Claude(), 1, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	server.audit = func(ctx context.Context, _ string, _ bool) (batch.Report, error) {
		<-ctx.Done()
		return batch.Report{}, ctx.Err()
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audits", strings.NewReader(`{"repository":"owner/repo"}`))
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusGatewayTimeout {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}
