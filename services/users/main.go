package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osship/osship/packages/kafka"
	"github.com/osship/osship/packages/observability"
)

type profile struct {
	ID             string `json:"id"`
	Email          string `json:"email,omitempty"`
	Role           string `json:"role"`
	GithubUsername string `json:"github_username,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	Bio            string `json:"bio,omitempty"`
}

type contribution struct {
	ID             string     `json:"id"`
	PRURL          string     `json:"pr_url"`
	GithubVerified bool       `json:"github_verified"`
	MergedAt       *time.Time `json:"merged_at,omitempty"`
}

type enrollment struct {
	ID        string `json:"id"`
	ListingID string `json:"listing_id"`
	Status    string `json:"status"`
}

func main() {
	dbURL := env("DATABASE_URL_GENERAL", "postgres://osship:osship_secret@postgres:5432/osship?sslmode=disable&search_path=general")
	port := env("PORT", "8083")
	brokers := env("KAFKA_BROKERS", "kafka:9092")
	githubToken := env("GITHUB_TOKEN", "")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	producer := kafka.NewProducer(brokers, "enrollment.events")
	defer producer.Close()

	s := &server{pool: pool, producer: producer, githubToken: githubToken}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(observability.PrometheusMiddleware("users"))

	r.Get("/health", observability.HealthHandler("users"))
	r.Get("/metrics", observability.MetricsHandler().ServeHTTP)

	r.Get("/{id}/profile", s.getProfile)
	r.Patch("/me", s.updateProfile)
	r.Post("/me/contributions", s.addContribution)
	r.Get("/{id}/enrollments", s.getEnrollments)
	r.Post("/enrollments", s.createEnrollment)
	r.Route("/enrollments/{id}/activate", func(r chi.Router) {
		r.Post("/", s.activateEnrollment)
		r.Patch("/", s.activateEnrollment)
	})

	log.Printf("users listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

type server struct {
	pool        *pgxpool.Pool
	producer    *kafka.Producer
	githubToken string
}

func (s *server) getProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var p profile
	err := s.pool.QueryRow(r.Context(),
		`SELECT id, role, COALESCE(github_username,''), COALESCE(display_name,''), COALESCE(bio,'') FROM users WHERE id=$1`, id).
		Scan(&p.ID, &p.Role, &p.GithubUsername, &p.DisplayName, &p.Bio)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if r.Header.Get("X-User-Id") == id {
		_ = s.pool.QueryRow(r.Context(), `SELECT email FROM users WHERE id=$1`, id).Scan(&p.Email)
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *server) updateProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		DisplayName    string `json:"display_name"`
		Bio            string `json:"bio"`
		GithubUsername string `json:"github_username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	_, err := s.pool.Exec(r.Context(),
		`UPDATE users SET
			display_name = CASE WHEN $1 <> '' THEN $1 ELSE display_name END,
			bio = CASE WHEN $2 <> '' THEN $2 ELSE bio END,
			github_username = CASE WHEN $3 <> '' THEN $3 ELSE github_username END,
			updated_at = NOW()
		WHERE id = $4`,
		req.DisplayName, req.Bio, req.GithubUsername, userID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	var p profile
	err = s.pool.QueryRow(r.Context(),
		`SELECT id, role, COALESCE(github_username,''), COALESCE(display_name,''), COALESCE(bio,'') FROM users WHERE id=$1`, userID).
		Scan(&p.ID, &p.Role, &p.GithubUsername, &p.DisplayName, &p.Bio)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	_ = s.pool.QueryRow(r.Context(), `SELECT email FROM users WHERE id=$1`, userID).Scan(&p.Email)
	writeJSON(w, http.StatusOK, p)
}

func (s *server) addContribution(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		PRURL string `json:"pr_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PRURL == "" {
		http.Error(w, `{"error":"pr_url required"}`, http.StatusBadRequest)
		return
	}

	verified, mergedAt := verifyGitHubPR(s.githubToken, req.PRURL)
	id := uuid.New().String()
	_, err := s.pool.Exec(r.Context(),
		`INSERT INTO contributions (id, user_id, pr_url, github_verified, merged_at) VALUES ($1,$2,$3,$4,$5)`,
		id, userID, req.PRURL, verified, mergedAt)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, contribution{ID: id, PRURL: req.PRURL, GithubVerified: verified, MergedAt: mergedAt})
}

func (s *server) getEnrollments(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rows, err := s.pool.Query(r.Context(),
		`SELECT id, listing_id, status FROM enrollments WHERE student_id=$1 ORDER BY created_at DESC`, id)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var list []enrollment
	for rows.Next() {
		var e enrollment
		if err := rows.Scan(&e.ID, &e.ListingID, &e.Status); err != nil {
			continue
		}
		list = append(list, e)
	}
	if list == nil {
		list = []enrollment{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) createEnrollment(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		ListingID string `json:"listing_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	id := uuid.New().String()
	_, err := s.pool.Exec(r.Context(),
		`INSERT INTO enrollments (id, listing_id, student_id, status) VALUES ($1,$2,$3,'pending_payment')`,
		id, req.ListingID, userID)
	if err != nil {
		http.Error(w, `{"error":"already enrolled or listing full"}`, http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusCreated, enrollment{ID: id, ListingID: req.ListingID, Status: "pending_payment"})
}

func (s *server) activateEnrollment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		CheckoutSessionID string `json:"checkout_session_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	var listingID string
	err = tx.QueryRow(r.Context(),
		`UPDATE enrollments SET status='active', stripe_checkout_session_id=$1, updated_at=NOW()
		 WHERE id=$2 AND status='pending_payment' RETURNING listing_id`,
		req.CheckoutSessionID, id).Scan(&listingID)
	if err != nil {
		http.Error(w, `{"error":"not found or already active"}`, http.StatusNotFound)
		return
	}

	_, err = tx.Exec(r.Context(),
		`UPDATE listings SET filled_slots = filled_slots + 1, updated_at = NOW(),
		 status = CASE WHEN filled_slots + 1 >= total_slots THEN 'full'::listing_status ELSE status END
		 WHERE id = $1`, listingID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	_ = s.producer.Publish(r.Context(), "enrollment.confirmed", map[string]string{"enrollment_id": id})
	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

var prURLPattern = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/pull/(\d+)`)

func verifyGitHubPR(token, prURL string) (bool, *time.Time) {
	matches := prURLPattern.FindStringSubmatch(prURL)
	if len(matches) != 4 {
		return false, nil
	}
	if token == "" {
		return false, nil
	}
	apiURL := "https://api.github.com/repos/" + matches[1] + "/" + matches[2] + "/pulls/" + matches[3]
	req, _ := http.NewRequest(http.MethodGet, apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var pr struct {
		MergedAt *time.Time `json:"merged_at"`
		State    string     `json:"state"`
	}
	if json.Unmarshal(body, &pr) != nil {
		return false, nil
	}
	return pr.State == "closed" && pr.MergedAt != nil, pr.MergedAt
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var _ = strings.TrimSpace
