package main

import (
	"context"
	"encoding/json"
	"fmt"
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

type Session struct {
	ID             string    `json:"id"`
	ListingID      string    `json:"listing_id"`
	ScheduledAt    time.Time `json:"scheduled_at"`
	JitsiRoomName  string    `json:"jitsi_room_name"`
	JitsiURL       string    `json:"jitsi_url"`
	Status         string    `json:"status"`
}

func main() {
	dbURL := env("DATABASE_URL_GENERAL", "postgres://osship:osship_secret@postgres:5432/osship?sslmode=disable&search_path=general")
	port := env("PORT", "8084")
	brokers := env("KAFKA_BROKERS", "kafka:9092")
	jitsiBase := env("JITSI_BASE_URL", "https://meet.jit.si")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	producer := kafka.NewProducer(brokers, "session.events")
	defer producer.Close()

	s := &server{pool: pool, producer: producer, jitsiBase: jitsiBase}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(observability.PrometheusMiddleware("sessions"))

	r.Get("/health", observability.HealthHandler("sessions"))
	r.Get("/metrics", observability.MetricsHandler().ServeHTTP)

	r.Post("/", s.create)
	r.Get("/listings/{listingId}", s.listByListing)
	r.Patch("/{id}", s.update)
	r.Post("/{id}/join", s.join)
	r.Post("/progress", s.addProgress)

	log.Printf("sessions listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

type server struct {
	pool      *pgxpool.Pool
	producer  *kafka.Producer
	jitsiBase string
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		ListingID   string    `json:"listing_id"`
		ScheduledAt time.Time `json:"scheduled_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	var mentorID string
	if err := s.pool.QueryRow(r.Context(), `SELECT mentor_id FROM listings WHERE id=$1`, req.ListingID).Scan(&mentorID); err != nil || mentorID != userID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	id := uuid.New().String()
	roomName := fmt.Sprintf("osship-listing-%s-session-%s", req.ListingID[:8], id[:8])
	jitsiURL := fmt.Sprintf("%s/%s", s.jitsiBase, roomName)
	_, err := s.pool.Exec(r.Context(),
		`INSERT INTO mentorship_sessions (id, listing_id, scheduled_at, jitsi_room_name, jitsi_url) VALUES ($1,$2,$3,$4,$5)`,
		id, req.ListingID, req.ScheduledAt, roomName, jitsiURL)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	_ = s.producer.Publish(r.Context(), "session.scheduled", map[string]string{"session_id": id, "listing_id": req.ListingID})
	writeJSON(w, http.StatusCreated, Session{ID: id, ListingID: req.ListingID, ScheduledAt: req.ScheduledAt, JitsiRoomName: roomName, JitsiURL: jitsiURL, Status: "scheduled"})
}

func (s *server) listByListing(w http.ResponseWriter, r *http.Request) {
	listingID := chi.URLParam(r, "listingId")
	rows, err := s.pool.Query(r.Context(),
		`SELECT id, listing_id, scheduled_at, jitsi_room_name, jitsi_url, status FROM mentorship_sessions WHERE listing_id=$1 ORDER BY scheduled_at`, listingID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var list []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.ListingID, &sess.ScheduledAt, &sess.JitsiRoomName, &sess.JitsiURL, &sess.Status); err != nil {
			continue
		}
		list = append(list, sess)
	}
	if list == nil {
		list = []Session{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		ScheduledAt *time.Time `json:"scheduled_at"`
		Status      string     `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	_, err := s.pool.Exec(r.Context(),
		`UPDATE mentorship_sessions SET scheduled_at=COALESCE($1,scheduled_at), status=COALESCE(NULLIF($2,''),status), updated_at=NOW() WHERE id=$3`,
		req.ScheduledAt, req.Status, id)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *server) join(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	var sess Session
	err := s.pool.QueryRow(r.Context(),
		`SELECT id, listing_id, scheduled_at, jitsi_room_name, jitsi_url, status FROM mentorship_sessions WHERE id=$1`, id).
		Scan(&sess.ID, &sess.ListingID, &sess.ScheduledAt, &sess.JitsiRoomName, &sess.JitsiURL, &sess.Status)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	_, _ = s.pool.Exec(r.Context(),
		`INSERT INTO session_attendance (session_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, userID)
	_, _ = s.pool.Exec(r.Context(), `UPDATE mentorship_sessions SET status='live' WHERE id=$1 AND status='scheduled'`, id)
	writeJSON(w, http.StatusOK, map[string]interface{}{"jitsi_url": sess.JitsiURL, "room": sess.JitsiRoomName})
}

func (s *server) addProgress(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		EnrollmentID string `json:"enrollment_id"`
		Note         string `json:"note"`
		PRURL        string `json:"pr_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	id := uuid.New().String()
	_, err := s.pool.Exec(r.Context(),
		`INSERT INTO progress_entries (id, enrollment_id, note, pr_url) VALUES ($1,$2,$3,$4)`,
		id, req.EnrollmentID, req.Note, nullStr(req.PRURL))
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
