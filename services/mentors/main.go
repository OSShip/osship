package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osship/osship/packages/kafka"
	"github.com/osship/osship/packages/observability"
)

type Application struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	Status     string          `json:"status"`
	GithubData json.RawMessage `json:"github_data,omitempty"`
	CreatedAt  string          `json:"created_at"`
}

func main() {
	dbURL := env("DATABASE_URL_GENERAL", "postgres://osship:osship_secret@postgres:5432/osship?sslmode=disable&search_path=general")
	port := env("PORT", "8085")
	brokers := env("KAFKA_BROKERS", "kafka:9092")
	githubToken := env("GITHUB_TOKEN", "")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	producer := kafka.NewProducer(brokers, "mentor.events")
	defer producer.Close()

	s := &server{pool: pool, producer: producer, githubToken: githubToken}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(observability.PrometheusMiddleware("mentors"))

	r.Get("/health", observability.HealthHandler("mentors"))
	r.Get("/metrics", observability.MetricsHandler().ServeHTTP)

	r.Post("/apply", s.apply)
	r.Get("/admin/applications", s.listApplications)
	r.Patch("/admin/applications/{id}", s.reviewApplication)

	log.Printf("mentors listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

type server struct {
	pool        *pgxpool.Pool
	producer    *kafka.Producer
	githubToken string
}

func (s *server) apply(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	githubUser := r.Header.Get("X-Github-Username")
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if githubUser == "" {
		var req struct {
			GithubUsername string `json:"github_username"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		githubUser = req.GithubUsername
	}
	if githubUser == "" {
		http.Error(w, `{"error":"github_username required"}`, http.StatusBadRequest)
		return
	}

	githubData := fetchGitHubProfile(s.githubToken, githubUser)
	id := uuid.New().String()
	_, err := s.pool.Exec(r.Context(),
		`INSERT INTO mentor_applications (id, user_id, github_data) VALUES ($1,$2,$3)`,
		id, userID, githubData)
	if err != nil {
		http.Error(w, `{"error":"application already exists"}`, http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusCreated, Application{ID: id, UserID: userID, Status: "pending", GithubData: githubData})
}

func (s *server) listApplications(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-User-Role") != "admin" {
		http.Error(w, `{"error":"admin required"}`, http.StatusForbidden)
		return
	}
	status := r.URL.Query().Get("status")
	q := `SELECT id, user_id, status, github_data, created_at FROM mentor_applications`
	args := []interface{}{}
	if status != "" {
		q += ` WHERE status=$1`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.pool.Query(r.Context(), q, args...)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var list []Application
	for rows.Next() {
		var a Application
		var createdAt interface{}
		if err := rows.Scan(&a.ID, &a.UserID, &a.Status, &a.GithubData, &createdAt); err != nil {
			continue
		}
		list = append(list, a)
	}
	if list == nil {
		list = []Application{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) reviewApplication(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-User-Role") != "admin" {
		http.Error(w, `{"error":"admin required"}`, http.StatusForbidden)
		return
	}
	id := chi.URLParam(r, "id")
	adminID := r.Header.Get("X-User-Id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Status != "approved" && req.Status != "rejected") {
		http.Error(w, `{"error":"status must be approved or rejected"}`, http.StatusBadRequest)
		return
	}
	var userID string
	err := s.pool.QueryRow(r.Context(),
		`UPDATE mentor_applications SET status=$1, reviewed_by=$2, reviewed_at=NOW() WHERE id=$3 RETURNING user_id`,
		req.Status, adminID, id).Scan(&userID)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if req.Status == "approved" {
		_, _ = s.pool.Exec(r.Context(), `UPDATE users SET role='mentor' WHERE id=$1`, userID)
		_ = s.producer.Publish(r.Context(), "mentor.approved", map[string]string{"user_id": userID})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": req.Status})
}

func fetchGitHubProfile(token, username string) json.RawMessage {
	url := "https://api.github.com/users/" + username
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return json.RawMessage(`{"error":"fetch failed"}`)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	reposURL := "https://api.github.com/users/" + username + "/repos?sort=updated&per_page=5"
	req2, _ := http.NewRequest(http.MethodGet, reposURL, nil)
	if token != "" {
		req2.Header.Set("Authorization", "Bearer "+token)
	}
	resp2, _ := http.DefaultClient.Do(req2)
	var repos json.RawMessage = json.RawMessage(`[]`)
	if resp2 != nil && resp2.StatusCode == http.StatusOK {
		reposBody, _ := io.ReadAll(resp2.Body)
		repos = reposBody
		resp2.Body.Close()
	}

	result := map[string]json.RawMessage{
		"profile": body,
		"repos":   repos,
	}
	data, _ := json.Marshal(result)
	return data
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
