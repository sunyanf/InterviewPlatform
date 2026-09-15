package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCounterVec_IncAndExpose(t *testing.T) {
	r := New()
	c := r.Counter("test_requests_total", "test counter", "method", "status")
	c.Inc("GET", "200")
	c.Inc("GET", "200")
	c.Add(2.5, "POST", "500")

	out := r.Expose()
	if !strings.Contains(out, "# HELP test_requests_total test counter") ||
		!strings.Contains(out, "# TYPE test_requests_total counter") {
		t.Fatalf("missing help/type:\n%s", out)
	}
	if !strings.Contains(out, `test_requests_total{method="GET",status="200"} 2`) {
		t.Errorf("GET 200 series wrong:\n%s", out)
	}
	if !strings.Contains(out, `test_requests_total{method="POST",status="500"} 2.5`) {
		t.Errorf("POST 500 series wrong:\n%s", out)
	}
}

func TestCounterVec_RedefinePanics(t *testing.T) {
	r := New()
	r.Counter("dup_total", "x", "a")
	defer func() {
		if recover() == nil {
			t.Fatal("redefining labels should panic")
		}
	}()
	r.Counter("dup_total", "x", "a", "b")
}

func TestHistogram_CumulativeBuckets(t *testing.T) {
	r := New()
	h := r.Histogram("test_duration_seconds", "test histogram",
		[]float64{0.1, 0.5, 1}, "route")

	h.Observe(0.05, "r1") // le=0.1
	h.Observe(0.4, "r1")  // le=0.1,0.5
	h.Observe(2, "r1")    // 仅 +Inf

	out := r.Expose()
	expect := []string{
		`test_duration_seconds_bucket{route="r1",le="0.1"} 1`,
		`test_duration_seconds_bucket{route="r1",le="0.5"} 2`,
		`test_duration_seconds_bucket{route="r1",le="1"} 2`,
		`test_duration_seconds_bucket{route="r1",le="+Inf"} 3`,
		`test_duration_seconds_count{route="r1"} 3`,
	}
	for _, want := range expect {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "test_duration_seconds_sum{route=\"r1\"} 2.45") {
		t.Errorf("sum wrong:\n%s", out)
	}
}

func TestHistogram_NoLabels(t *testing.T) {
	r := New()
	h := r.Histogram("plain_seconds", "no labels", []float64{1})
	h.Observe(0.5)
	out := r.Expose()
	if !strings.Contains(out, `plain_seconds_bucket{le="1"} 1`) {
		t.Errorf("unlabeled bucket wrong:\n%s", out)
	}
}

func TestRegistry_HandlerContentType(t *testing.T) {
	r := New()
	r.Counter("h_total", "x").Inc()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, req)
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type = %q", ct)
	}
	if !strings.Contains(w.Body.String(), "h_total 1") {
		t.Errorf("body = %q", w.Body.String())
	}
}
