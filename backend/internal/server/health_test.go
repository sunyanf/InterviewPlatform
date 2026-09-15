package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivezHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()

	livezHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("want status ok, got %q", body.Status)
	}
}

func TestEvaluateReadiness_AllHealthy(t *testing.T) {
	status, resp := evaluateReadiness(context.Background(), []namedCheck{
		{"postgres", func(context.Context) error { return nil }},
		{"minio", func(context.Context) error { return nil }},
	})
	if status != http.StatusOK {
		t.Fatalf("want 200, got %d", status)
	}
	if resp.Status != "ok" {
		t.Fatalf("want ok, got %q", resp.Status)
	}
	if resp.Checks["postgres"] != "ok" || resp.Checks["minio"] != "ok" {
		t.Fatalf("want both checks ok, got %+v", resp.Checks)
	}
}

func TestEvaluateReadiness_OneFailed(t *testing.T) {
	dbErr := errors.New("connection refused")
	status, resp := evaluateReadiness(context.Background(), []namedCheck{
		{"postgres", func(context.Context) error { return dbErr }},
		{"minio", func(context.Context) error { return nil }},
	})
	if status != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", status)
	}
	if resp.Status != "unavailable" {
		t.Fatalf("want unavailable, got %q", resp.Status)
	}
	if resp.Checks["postgres"] != "error: connection refused" {
		t.Fatalf("want postgres error surfaced, got %q", resp.Checks["postgres"])
	}
	if resp.Checks["minio"] != "ok" {
		t.Fatalf("want minio still ok, got %q", resp.Checks["minio"])
	}
}

func TestEvaluateReadiness_AllFailed(t *testing.T) {
	status, resp := evaluateReadiness(context.Background(), []namedCheck{
		{"postgres", func(context.Context) error { return errors.New("down") }},
		{"minio", func(context.Context) error { return errors.New("down") }},
	})
	if status != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", status)
	}
	if len(resp.Checks) != 2 {
		t.Fatalf("want 2 checks reported, got %d", len(resp.Checks))
	}
}

// TestEvaluateReadiness_Empty 无检查项时视为就绪
func TestEvaluateReadiness_Empty(t *testing.T) {
	status, resp := evaluateReadiness(context.Background(), nil)
	if status != http.StatusOK || resp.Status != "ok" {
		t.Fatalf("want 200/ok for no checks, got %d/%q", status, resp.Status)
	}
}
