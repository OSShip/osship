package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"service", "method", "route", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "route"},
	)

	HTTPRequestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "In-flight HTTP requests",
		},
		[]string{"service"},
	)

	RateLimitExceeded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_exceeded_total",
			Help: "Rate limit exceeded count",
		},
		[]string{"service", "route"},
	)
)

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

func PrometheusMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			HTTPRequestsInFlight.WithLabelValues(serviceName).Inc()
			defer HTTPRequestsInFlight.WithLabelValues(serviceName).Dec()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			route := chiRoutePattern(r)
			status := strconv.Itoa(ww.Status())
			HTTPRequestsTotal.WithLabelValues(serviceName, r.Method, route, status).Inc()
			HTTPRequestDuration.WithLabelValues(serviceName, r.Method, route).Observe(time.Since(start).Seconds())
		})
	}
}

func chiRoutePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if pattern := rc.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return r.URL.Path
}

func HealthHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"` + serviceName + `"}`))
	}
}
