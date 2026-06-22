package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osship/osship/packages/kafka"
	"github.com/osship/osship/packages/observability"
)

type Listing struct {
	ID             string    `json:"id"`
	MentorID       string    `json:"mentor_id"`
	OSSProjectName string    `json:"oss_project_name"`
	OSSRepoURL     string    `json:"oss_repo_url"`
	Description    string    `json:"description"`
	PriceCents     int       `json:"price_cents"`
	DurationWeeks  int       `json:"duration_weeks"`
	TotalSlots     int       `json:"total_slots"`
	FilledSlots    int       `json:"filled_slots"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

func main() {
	dbURL := env("DATABASE_URL_GENERAL", "postgres://osship:osship_secret@postgres:5432/osship?sslmode=disable&search_path=general")
	port := env("PORT", "8082")
	brokers := env("KAFKA_BROKERS", "kafka:9092")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	producer := kafka.NewProducer(brokers, "listing.events")
	defer producer.Close()

	s := &server{pool: pool, producer: producer}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(observability.PrometheusMiddleware("listings"))

	r.Get("/health", observability.HealthHandler("listings"))
	r.Get("/metrics", observability.MetricsHandler().ServeHTTP)

	r.Get("/", s.list)
	r.Get("/{id}", s.get)
	r.Post("/", s.create)
	r.Patch("/{id}", s.update)

	log.Printf("listings listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

type server struct {
	pool     *pgxpool.Pool
	producer *kafka.Producer
}

func (s *server) list(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}
	rows, err := s.pool.Query(r.Context(),
		`SELECT id, mentor_id, oss_project_name, oss_repo_url, description, price_cents, duration_weeks, total_slots, filled_slots, status, created_at
		 FROM listings WHERE status=$1 ORDER BY created_at DESC`, status)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	list := scanListings(rows)
	if list == nil {
		list = []Listing{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var l Listing
	err := s.pool.QueryRow(r.Context(),
		`SELECT id, mentor_id, oss_project_name, oss_repo_url, description, price_cents, duration_weeks, total_slots, filled_slots, status, created_at
		 FROM listings WHERE id=$1`, id).
		Scan(&l.ID, &l.MentorID, &l.OSSProjectName, &l.OSSRepoURL, &l.Description, &l.PriceCents, &l.DurationWeeks, &l.TotalSlots, &l.FilledSlots, &l.Status, &l.CreatedAt)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	role := r.Header.Get("X-User-Role")
	if userID == "" || role != "mentor" {
		http.Error(w, `{"error":"mentor role required"}`, http.StatusForbidden)
		return
	}

	var approved bool
	_ = s.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM mentor_applications WHERE user_id=$1 AND status='approved')`, userID).Scan(&approved)
	if !approved {
		http.Error(w, `{"error":"mentor not approved"}`, http.StatusForbidden)
		return
	}

	var req Listing
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	id := uuid.New().String()
	status := req.Status
	if status == "" {
		status = "active"
	}
	_, err := s.pool.Exec(r.Context(),
		`INSERT INTO listings (id, mentor_id, oss_project_name, oss_repo_url, description, price_cents, duration_weeks, total_slots, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, userID, req.OSSProjectName, req.OSSRepoURL, req.Description, req.PriceCents, req.DurationWeeks, req.TotalSlots, status)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	_ = s.producer.Publish(r.Context(), "listing.created", map[string]string{"listing_id": id})
	req.ID = id
	req.MentorID = userID
	req.Status = status
	writeJSON(w, http.StatusCreated, req)
}

func (s *server) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")
	var mentorID string
	if err := s.pool.QueryRow(r.Context(), `SELECT mentor_id FROM listings WHERE id=$1`, id).Scan(&mentorID); err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if mentorID != userID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Description   string `json:"description"`
		Status        string `json:"status"`
		PriceCents    int    `json:"price_cents"`
		DurationWeeks int    `json:"duration_weeks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	_, err := s.pool.Exec(r.Context(),
		`UPDATE listings SET description=COALESCE(NULLIF($1,''),description), status=COALESCE(NULLIF($2,''),status),
		 price_cents=CASE WHEN $3>0 THEN $3 ELSE price_cents END,
		 duration_weeks=CASE WHEN $4>0 THEN $4 ELSE duration_weeks END, updated_at=NOW() WHERE id=$5`,
		req.Description, req.Status, req.PriceCents, req.DurationWeeks, id)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	_ = s.producer.Publish(r.Context(), "listing.updated", map[string]string{"listing_id": id})
	s.get(w, r)
}

func scanListings(rows interface {
	Next() bool
	Scan(...interface{}) error
}) []Listing {
	var list []Listing
	for rows.Next() {
		var l Listing
		if err := rows.Scan(&l.ID, &l.MentorID, &l.OSSProjectName, &l.OSSRepoURL, &l.Description, &l.PriceCents, &l.DurationWeeks, &l.TotalSlots, &l.FilledSlots, &l.Status, &l.CreatedAt); err != nil {
			continue
		}
		list = append(list, l)
	}
	return list
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
