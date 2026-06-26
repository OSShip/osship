package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatchRateLimitAuthLogin(t *testing.T) {
	rules := DefaultRouteLimits()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rl := MatchRateLimit(req, rules)
	if rl.Group != "auth_login" {
		t.Fatalf("expected auth_login, got %s", rl.Group)
	}
	if rl.Limit.Rate != 10 {
		t.Fatalf("expected 10 req/min, got %d", rl.Limit.Rate)
	}
}

func TestMatchRateLimitPublicRead(t *testing.T) {
	rules := DefaultRouteLimits()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/listings", nil)
	rl := MatchRateLimit(req, rules)
	if rl.Group != "public_read" {
		t.Fatalf("expected public_read, got %s", rl.Group)
	}
}

func TestMatchRateLimitDefault(t *testing.T) {
	rules := DefaultRouteLimits()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc/profile", nil)
	rl := MatchRateLimit(req, rules)
	if rl.Group != "default" {
		t.Fatalf("expected default, got %s", rl.Group)
	}
	if rl.Limit.Rate != 300 {
		t.Fatalf("expected 300 req/min, got %d", rl.Limit.Rate)
	}
}

func TestMatchRateLimitSessionsJoin(t *testing.T) {
	rules := DefaultRouteLimits()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/abc-123/join", nil)
	rl := MatchRateLimit(req, rules)
	if rl.Group != "sessions_join" {
		t.Fatalf("expected sessions_join, got %s", rl.Group)
	}
	if !rl.ByUser {
		t.Fatal("sessions_join should be keyed by user")
	}
}
