package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-redis/redis_rate/v10"
	"github.com/osship/osship/packages/jwtutil"
	"github.com/osship/osship/packages/observability"
	"github.com/redis/go-redis/v9"
)

var routeLimits = observability.DefaultRouteLimits()

var protectedRoutes = []struct {
	method string
	prefix string
}{
	{http.MethodGet, "/api/v1/auth/me"},
	{http.MethodPatch, "/api/v1/users/me"},
	{http.MethodPost, "/api/v1/users/me/"},
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
	routeLimits = applyRateLimitOverrides(routeLimits)

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

	if requiresAuth(path, r.Method) {
		if _, err := resolveClaims(r.Context(), r, rdb, jwtSecret); err != nil {
			observability.HTTPRequestsTotal.WithLabelValues("gateway", r.Method, path, "401").Inc()
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
	}

	rl := observability.MatchRateLimit(r, routeLimits)
	if allowed, retryAfter, err := checkRateLimit(r.Context(), r, limiter, jwtSecret, rdb, rl); err == nil && !allowed {
		observability.RateLimitExceeded.WithLabelValues("gateway", rl.Group).Inc()
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
		return
	}

	target, stripPrefix := resolveBackend(path, backends)
	if target == "" {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	if r.Method == http.MethodGet && isCacheableGET(path) {
		cacheKey := cacheKeyForRequest(r)
		label := cacheLabel(cacheKey)
		if cached, err := rdb.Get(r.Context(), cacheKey).Bytes(); err == nil {
			observability.CacheHits.WithLabelValues(label).Inc()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached)
			return
		}
		observability.CacheMisses.WithLabelValues(label).Inc()
		rec := &responseRecorder{ResponseWriter: w, status: 200, body: &bytes.Buffer{}}
		proxyRequest(rec, r, target, stripPrefix, jwtSecret, rdb)
		if rec.status == http.StatusOK && rec.body.Len() > 0 {
			_ = rdb.Set(r.Context(), cacheKey, rec.body.Bytes(), cacheTTL(path)).Err()
		}
		return
	}

	if r.Method != http.MethodGet && strings.HasPrefix(path, "/api/v1/listings") {
		defer invalidateListingCache(r.Context(), rdb)
	}

	proxyRequest(w, r, target, stripPrefix, jwtSecret, rdb)
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

func proxyRequest(w http.ResponseWriter, r *http.Request, targetURL, stripPrefix, jwtSecret string, rdb *redis.Client) {
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

		if claims, err := resolveClaims(req.Context(), r, rdb, jwtSecret); err == nil {
			req.Header.Set("X-User-Id", claims.UserID)
			req.Header.Set("X-User-Role", claims.Role)
			if claims.GithubUsername != "" {
				req.Header.Set("X-Github-Username", claims.GithubUsername)
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

type cachedClaims struct {
	UserID         string `json:"sub"`
	Role           string `json:"role"`
	GithubUsername string `json:"github_username,omitempty"`
}

func resolveClaims(ctx context.Context, r *http.Request, rdb *redis.Client, jwtSecret string) (*jwtutil.Claims, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return nil, fmt.Errorf("missing authorization")
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	if tokenStr == auth {
		return nil, fmt.Errorf("invalid authorization header")
	}

	hash := sha256.Sum256([]byte(tokenStr))
	cacheKey := "auth:session:" + hex.EncodeToString(hash[:16])

	if cached, err := rdb.Get(ctx, cacheKey).Bytes(); err == nil {
		var cc cachedClaims
		if json.Unmarshal(cached, &cc) == nil {
			return &jwtutil.Claims{UserID: cc.UserID, Role: cc.Role, GithubUsername: cc.GithubUsername}, nil
		}
	}

	claims, err := jwtutil.ValidateToken(jwtSecret, tokenStr)
	if err != nil {
		return nil, err
	}

	if claims.ExpiresAt != nil {
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			payload, _ := json.Marshal(cachedClaims{
				UserID:         claims.UserID,
				Role:           claims.Role,
				GithubUsername: claims.GithubUsername,
			})
			_ = rdb.Set(ctx, cacheKey, payload, ttl).Err()
		}
	}
	return claims, nil
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

func checkRateLimit(ctx context.Context, r *http.Request, limiter *redis_rate.Limiter, jwtSecret string, rdb *redis.Client, rl observability.RouteLimit) (bool, int, error) {
	identifier := clientIP(r)
	if rl.ByUser {
		if claims, err := resolveClaims(ctx, r, rdb, jwtSecret); err == nil {
			identifier = claims.UserID
		}
	}
	key := observability.RateLimitKey(rl.Group, identifier)
	return observability.AllowRequest(ctx, limiter, key, rl.Limit)
}

func requiresAuth(path, method string) bool {
	for _, pr := range protectedRoutes {
		if method != pr.method {
			continue
		}
		if strings.HasSuffix(pr.prefix, "/") {
			if strings.HasPrefix(path, pr.prefix) {
				return true
			}
		} else if path == pr.prefix || strings.HasPrefix(path, pr.prefix+"/") {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

func cacheKeyForRequest(r *http.Request) string {
	path := r.URL.Path
	q := r.URL.Query()
	if path == "/api/v1/listings" && q.Get("status") == "active" && q.Get("oss_project") == "" {
		return "listings:active"
	}
	if path == "/api/v1/public/payout-summary" {
		return "public:ledger:summary"
	}
	if strings.HasPrefix(path, "/api/v1/listings/") && len(strings.Split(strings.TrimPrefix(path, "/api/v1/listings/"), "/")) == 1 {
		return "listings:id:" + strings.TrimPrefix(path, "/api/v1/listings/")
	}
	if path == "/api/v1/public/listings" && q.Get("status") == "active" && q.Get("oss_project") == "" {
		return "listings:active"
	}
	h := sha256.Sum256([]byte(r.URL.String()))
	return "cache:" + hex.EncodeToString(h[:8])
}

func cacheLabel(key string) string {
	if strings.HasPrefix(key, "listings:") || strings.HasPrefix(key, "public:") {
		return key
	}
	return "other"
}

func isCacheableGET(path string) bool {
	return strings.HasPrefix(path, "/api/v1/listings") || strings.HasPrefix(path, "/api/v1/public")
}

func cacheTTL(path string) time.Duration {
	if strings.Contains(path, "payout-summary") {
		return 300 * time.Second
	}
	return 60 * time.Second
}

func invalidateListingCache(ctx context.Context, rdb *redis.Client) {
	keys := []string{"listings:active", "public:ledger:summary"}
	_ = rdb.Del(ctx, keys...).Err()
	iter := rdb.Scan(ctx, 0, "listings:id:*", 100).Iterator()
	for iter.Next(ctx) {
		_ = rdb.Del(ctx, iter.Val()).Err()
	}
	iter = rdb.Scan(ctx, 0, "cache:*", 100).Iterator()
	for iter.Next(ctx) {
		_ = rdb.Del(ctx, iter.Val()).Err()
	}
}

func applyRateLimitOverrides(rules []observability.RouteLimitRule) []observability.RouteLimitRule {
	out := make([]observability.RouteLimitRule, len(rules))
	copy(out, rules)
	if v := envInt("RATE_LIMIT_AUTH_LOGIN", 0); v > 0 {
		limit := redis_rate.PerMinute(v)
		for i := range out {
			if out[i].Limit.Group == "auth_login" {
				out[i].Limit.Limit = limit
			}
		}
	}
	if v := envInt("RATE_LIMIT_PAYMENTS_CHECKOUT", 0); v > 0 {
		limit := redis_rate.PerMinute(v)
		for i := range out {
			if out[i].Limit.Group == "payments_checkout" {
				out[i].Limit.Limit = limit
			}
		}
	}
	return out
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var _ = io.Discard
