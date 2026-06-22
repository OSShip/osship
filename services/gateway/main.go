package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-redis/redis_rate/v10"
	"github.com/osship/osship/packages/jwtutil"
	"github.com/osship/osship/packages/observability"
	"github.com/redis/go-redis/v9"
)

type routeLimit struct {
	group    string
	limit    redis_rate.Limit
	byUser   bool
}

var routeLimits = []struct {
	method  string
	pattern string
	limit   routeLimit
}{
	{"POST", "/api/v1/auth/login", routeLimit{"auth_login", redis_rate.PerMinute(10), false}},
	{"POST", "/api/v1/auth/register", routeLimit{"auth_login", redis_rate.PerMinute(10), false}},
	{"POST", "/api/v1/auth/refresh", routeLimit{"auth_refresh", redis_rate.PerMinute(30), true}},
	{"POST", "/api/v1/payments/checkout", routeLimit{"payments_checkout", redis_rate.PerMinute(5), true}},
	{"POST", "/api/v1/payments/webhooks/stripe", routeLimit{"payments_webhook", redis_rate.PerMinute(100), false}},
	{"POST", "/api/v1/mentors/apply", routeLimit{"mentors_apply", redis_rate.PerHour(3), true}},
	{"POST", "/api/v1/sessions/*/join", routeLimit{"sessions_join", redis_rate.PerMinute(20), true}},
}

func main() {
	port := env("PORT", "8080")
	jwtSecret := env("JWT_SECRET", "dev-secret")
	redisURL := env("REDIS_URL", "redis://redis:6379")

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	limiter := redis_rate.NewLimiter(rdb)

	backends := map[string]string{
		"auth":     env("AUTH_SERVICE_URL", "http://auth:8081"),
		"listings": env("LISTINGS_SERVICE_URL", "http://listings:8082"),
		"users":    env("USERS_SERVICE_URL", "http://users:8083"),
		"sessions": env("SESSIONS_SERVICE_URL", "http://sessions:8084"),
		"mentors":  env("MENTORS_SERVICE_URL", "http://mentors:8085"),
		"payments": env("PAYMENTS_SERVICE_URL", "http://payments:8087"),
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(observability.PrometheusMiddleware("gateway"))

	r.Get("/health", observability.HealthHandler("gateway"))
	r.Get("/metrics", observability.MetricsHandler().ServeHTTP)

	r.Get("/api/v1/health", observability.HealthHandler("gateway"))

	r.Route("/api/v1", func(api chi.Router) {
		api.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
			handleProxy(w, req, backends, rdb, limiter, jwtSecret)
		})
	})

	log.Printf("gateway listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func handleProxy(w http.ResponseWriter, r *http.Request, backends map[string]string, rdb *redis.Client, limiter *redis_rate.Limiter, jwtSecret string) {
	path := r.URL.Path

	if err := applyRateLimit(r.Context(), r, limiter, jwtSecret); err != nil {
		observability.RateLimitExceeded.WithLabelValues("gateway", rateLimitGroup(r)).Inc()
		w.Header().Set("Retry-After", "60")
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
		return
	}

	target, stripPrefix := resolveBackend(path, backends)
	if target == "" {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	// Cache public listings
	if r.Method == http.MethodGet && (strings.HasPrefix(path, "/api/v1/listings") || strings.HasPrefix(path, "/api/v1/public")) {
		cacheKey := cacheKeyForRequest(r)
		if cached, err := rdb.Get(r.Context(), cacheKey).Bytes(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached)
			return
		}
		rec := &responseRecorder{ResponseWriter: w, status: 200, body: &bytes.Buffer{}}
		proxyRequest(rec, r, target, stripPrefix, jwtSecret)
		if rec.status == http.StatusOK && rec.body.Len() > 0 {
			ttl := 60 * time.Second
			if strings.Contains(path, "payout-summary") {
				ttl = 300 * time.Second
			}
			_ = rdb.Set(r.Context(), cacheKey, rec.body.Bytes(), ttl).Err()
		}
		return
	}

	// Invalidate cache on listing writes
	if r.Method != http.MethodGet && strings.HasPrefix(path, "/api/v1/listings") {
		defer invalidateListingCache(r.Context(), rdb)
	}

	proxyRequest(w, r, target, stripPrefix, jwtSecret)
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func proxyRequest(w http.ResponseWriter, r *http.Request, targetURL, stripPrefix, jwtSecret string) {
	target, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		stripped := strings.TrimPrefix(req.URL.Path, stripPrefix)
		req.URL.Path = rewritePath(stripped, stripPrefix)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		req.Host = target.Host

		if auth := r.Header.Get("Authorization"); auth != "" {
			if claims, err := jwtutil.ValidateToken(jwtSecret, strings.TrimPrefix(auth, "Bearer ")); err == nil {
				req.Header.Set("X-User-Id", claims.UserID)
				req.Header.Set("X-User-Role", claims.Role)
				if claims.GithubUsername != "" {
					req.Header.Set("X-Github-Username", claims.GithubUsername)
				}
			}
		}
		if reqID := middleware.GetReqID(r.Context()); reqID != "" {
			req.Header.Set("X-Request-Id", reqID)
		}
	}

	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(rw, `{"error":"service unavailable"}`, http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

func rewritePath(stripped, stripPrefix string) string {
	if stripPrefix == "/api/v1/public/payout-summary" {
		return "/payout-summary"
	}
	if stripPrefix == "/api/v1/public/listings" {
		if stripped == "" {
			return "/"
		}
		return stripped
	}
	return stripped
}

func resolveBackend(path string, backends map[string]string) (string, string) {
	routes := []struct {
		prefix  string
		service string
		strip   string
	}{
		{"/api/v1/auth", "auth", "/api/v1/auth"},
		{"/api/v1/listings", "listings", "/api/v1/listings"},
		{"/api/v1/users", "users", "/api/v1/users"},
		{"/api/v1/sessions", "sessions", "/api/v1/sessions"},
		{"/api/v1/mentors", "mentors", "/api/v1/mentors"},
		{"/api/v1/payments", "payments", "/api/v1/payments"},
		{"/api/v1/public/listings", "listings", "/api/v1/public/listings"},
		{"/api/v1/public/payout-summary", "payments", "/api/v1/public/payout-summary"},
	}
	for _, rt := range routes {
		if strings.HasPrefix(path, rt.prefix) {
			return backends[rt.service], rt.strip
		}
	}
	return "", ""
}

func applyRateLimit(ctx context.Context, r *http.Request, limiter *redis_rate.Limiter, jwtSecret string) error {
	rl := matchRateLimit(r)
	identifier := clientIP(r)
	if rl.byUser {
		if auth := r.Header.Get("Authorization"); auth != "" {
			if claims, err := jwtutil.ValidateToken(jwtSecret, strings.TrimPrefix(auth, "Bearer ")); err == nil {
				identifier = claims.UserID
			}
		}
	}
	key := fmt.Sprintf("rl:%s:%s", rl.group, identifier)
	res, err := limiter.Allow(ctx, key, rl.limit)
	if err != nil {
		return nil // fail open on redis errors
	}
	if res.Allowed == 0 {
		return fmt.Errorf("rate limited")
	}
	return nil
}

func matchRateLimit(r *http.Request) routeLimit {
	defaultLimit := routeLimit{"default", redis_rate.PerMinute(300), false}
	path := r.URL.Path
	for _, rl := range routeLimits {
		if r.Method != rl.method {
			continue
		}
		if strings.Contains(rl.pattern, "*") {
			prefix := strings.Split(rl.pattern, "*")[0]
			if strings.HasPrefix(path, prefix) {
				return rl.limit
			}
		} else if path == rl.pattern {
			return rl.limit
		}
	}
	if r.Method == http.MethodGet && (strings.HasPrefix(path, "/api/v1/public") || path == "/api/v1/listings") {
		return routeLimit{"public_read", redis_rate.PerMinute(120), false}
	}
	return defaultLimit
}

func rateLimitGroup(r *http.Request) string {
	return matchRateLimit(r).group
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

func cacheKeyForRequest(r *http.Request) string {
	h := sha256.Sum256([]byte(r.URL.String()))
	return "cache:" + hex.EncodeToString(h[:8])
}

func invalidateListingCache(ctx context.Context, rdb *redis.Client) {
	iter := rdb.Scan(ctx, 0, "cache:*", 100).Iterator()
	for iter.Next(ctx) {
		_ = rdb.Del(ctx, iter.Val()).Err()
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// suppress unused import
var _ = io.Discard
