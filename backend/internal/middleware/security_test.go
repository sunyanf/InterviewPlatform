package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLimiter_BurstAndRefill(t *testing.T) {
	// 600/min = 10 token/s；突发容量 2
	l := NewLimiter(600, 2)
	if !l.Allow("k") || !l.Allow("k") {
		t.Fatal("first two requests within burst must pass")
	}
	if l.Allow("k") {
		t.Fatal("third request must be rate limited")
	}
	// 不同 key 独立计数
	if !l.Allow("other") {
		t.Fatal("other key must have its own bucket")
	}
	// 等待补充约 1 个令牌
	time.Sleep(180 * time.Millisecond)
	if !l.Allow("k") {
		t.Fatal("token must refill over time")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	l := NewLimiter(60, 1)
	h := RateLimit(l, KeyByIP)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRequest(http.MethodGet, "/", nil)
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, first)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request = %d, want 200", w1.Code)
	}

	second := httptest.NewRequest(http.MethodGet, "/", nil)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, second)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request = %d, want 429", w2.Code)
	}
	if w2.Header().Get("Retry-After") == "" {
		t.Error("429 must carry Retry-After")
	}
}

func TestMaxBody_ContentLengthRejected(t *testing.T) {
	h := MaxBody(10)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789AB"))
	req.ContentLength = 12
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %d, want 413", w.Code)
	}
}

func TestMaxBody_WithinLimit(t *testing.T) {
	h := MaxBody(100)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("small"))
	req.ContentLength = 5
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
}

func TestCORS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("whitelisted origin gets headers", func(t *testing.T) {
		h := CORS([]string{"https://app.example.com"})(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://app.example.com")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
			t.Fatalf("allow-origin = %q", got)
		}
	})

	t.Run("non-whitelisted origin gets no headers", func(t *testing.T) {
		h := CORS([]string{"https://app.example.com"})(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("unexpected allow-origin %q", got)
		}
	})

	t.Run("empty whitelist allows all in dev", func(t *testing.T) {
		h := CORS(nil)(next)
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("preflight = %d, want 204", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
			t.Fatal("dev mode must echo origin")
		}
	})

	t.Run("same-origin request without Origin passes", func(t *testing.T) {
		h := CORS([]string{"https://app.example.com"})(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("code = %d, want 200", w.Code)
		}
	})
}

func TestSecureHeaders(t *testing.T) {
	h := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, hk := range []string{"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if w.Header().Get(hk) == "" {
			t.Errorf("missing security header %s", hk)
		}
	}
}
