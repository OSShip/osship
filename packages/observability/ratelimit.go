package observability

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-redis/redis_rate/v10"
)

// RouteLimit describes a per-route rate limit group.
type RouteLimit struct {
	Group  string
	Limit  redis_rate.Limit
	ByUser bool
}

// RouteLimitRule binds an HTTP method and path pattern to a limit.
type RouteLimitRule struct {
	Method  string
	Pattern string
	Limit   RouteLimit
}

// DefaultRouteLimits returns the standard OSShip gateway rate limit rules.
func DefaultRouteLimits() []RouteLimitRule {
	return []RouteLimitRule{
		{"POST", "/api/v1/auth/login", RouteLimit{"auth_login", redis_rate.PerMinute(10), false}},
		{"POST", "/api/v1/auth/register", RouteLimit{"auth_login", redis_rate.PerMinute(10), false}},
		{"POST", "/api/v1/auth/refresh", RouteLimit{"auth_refresh", redis_rate.PerMinute(30), true}},
		{"POST", "/api/v1/payments/checkout", RouteLimit{"payments_checkout", redis_rate.PerMinute(5), true}},
		{"POST", "/api/v1/payments/webhooks/stripe", RouteLimit{"payments_webhook", redis_rate.PerMinute(100), false}},
		{"POST", "/api/v1/mentors/apply", RouteLimit{"mentors_apply", redis_rate.PerHour(3), true}},
		{"POST", "/api/v1/sessions/*/join", RouteLimit{"sessions_join", redis_rate.PerMinute(20), true}},
	}
}

// MatchRateLimit selects the rate limit rule for a request.
func MatchRateLimit(r *http.Request, rules []RouteLimitRule) RouteLimit {
	defaultLimit := RouteLimit{"default", redis_rate.PerMinute(300), false}
	path := r.URL.Path
	for _, rl := range rules {
		if r.Method != rl.Method {
			continue
		}
		if strings.Contains(rl.Pattern, "*") {
			prefix := strings.Split(rl.Pattern, "*")[0]
			if strings.HasPrefix(path, prefix) {
				return rl.Limit
			}
		} else if path == rl.Pattern {
			return rl.Limit
		}
	}
	if r.Method == http.MethodGet && (strings.HasPrefix(path, "/api/v1/public") || path == "/api/v1/listings") {
		return RouteLimit{"public_read", redis_rate.PerMinute(120), false}
	}
	return defaultLimit
}

// AllowRequest checks Redis rate limiter; returns retry-after seconds when denied.
func AllowRequest(ctx context.Context, limiter *redis_rate.Limiter, key string, limit redis_rate.Limit) (allowed bool, retryAfter int, err error) {
	res, err := limiter.Allow(ctx, key, limit)
	if err != nil {
		return true, 0, err
	}
	if res.Allowed == 0 {
		retry := int(res.RetryAfter.Seconds())
		if retry < 1 {
			retry = 60
		}
		return false, retry, nil
	}
	return true, 0, nil
}

// RateLimitKey builds the Redis key for a rate limit bucket.
func RateLimitKey(group, identifier string) string {
	return fmt.Sprintf("rl:%s:%s", group, identifier)
}
