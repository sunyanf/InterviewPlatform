package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"ai-interview-platform/pkg/metrics"
)

func TestMetricsMiddleware_RecordsRouteAndStatus(t *testing.T) {
	r := chi.NewRouter()
	r.Use(Metrics)
	r.Get("/metrics-test/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	req := httptest.NewRequest("GET", "/metrics-test/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	out := metrics.Default.Expose()
	want := `http_requests_total{method="GET",route="/metrics-test/{id}",status="418"} 1`
	if !strings.Contains(out, want) {
		t.Fatalf("missing series %s in:\n%s", want, out)
	}
	if !strings.Contains(out, `http_request_duration_seconds_bucket{method="GET",route="/metrics-test/{id}",le="+Inf"} 1`) {
		t.Fatalf("duration histogram not recorded:\n%s", out)
	}
}

func TestMetricsMiddleware_UnmatchedRoute(t *testing.T) {
	r := chi.NewRouter()
	r.Use(Metrics)
	r.Get("/exists", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest("GET", "/does-not-exist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	out := metrics.Default.Expose()
	if !strings.Contains(out, `route="unmatched",status="404"`) {
		t.Fatalf("unmatched route label missing:\n%s", out)
	}
}
