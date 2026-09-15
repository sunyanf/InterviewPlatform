package realtime

import (
	"net/http"
	"testing"
)

func reqWithOrigin(origin string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/ws", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	return r
}

// 空白名单：开发模式全部放行（保持向后兼容）
func TestOriginChecker_EmptyWhitelistAllowsAll(t *testing.T) {
	check := newOriginChecker(nil)
	if !check(reqWithOrigin("http://evil.example.com")) {
		t.Fatal("empty whitelist must allow all origins in dev mode")
	}
	if !check(reqWithOrigin("")) {
		t.Fatal("empty whitelist must allow missing origin")
	}
}

// 配置白名单后：仅命中放行
func TestOriginChecker_Whitelist(t *testing.T) {
	check := newOriginChecker([]string{
		"http://localhost:5173",
		"https://interview.example.com",
	})

	cases := []struct {
		name   string
		origin string
		want   bool
	}{
		{"dev frontend", "http://localhost:5173", true},
		{"prod https", "https://interview.example.com", true},
		{"case insensitive", "HTTP://Localhost:5173", true},
		{"trailing slash normalized", "http://localhost:5173/", true},
		{"missing origin (non-browser)", "", true},
		{"different port", "http://localhost:8080", false},
		{"http instead of https", "http://interview.example.com", false},
		{"evil subdomain", "http://evil.localhost:5173", false},
		{"evil host", "https://interview.example.com.evil.com", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := check(reqWithOrigin(c.origin)); got != c.want {
				t.Fatalf("origin %q: want %v, got %v", c.origin, c.want, got)
			}
		})
	}
}

// 配置项含空白条目时被忽略；当显式配置了白名单，未命中即 fail-closed
func TestOriginChecker_IgnoreBlankEntries(t *testing.T) {
	check := newOriginChecker([]string{"  ", "http://localhost:5173", ""})
	if !check(reqWithOrigin("http://localhost:5173")) {
		t.Fatal("valid origin should be allowed")
	}
	if check(reqWithOrigin("http://evil.example.com")) {
		t.Fatal("evil origin must be rejected when whitelist configured")
	}
}
