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
	"github.com/osship/osship/packages/jwtutil"
	"github.com/osship/osship/packages/observability"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID             string `json:"id"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	GithubUsername string `json:"github_username,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
}

type registerReq struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	Role           string `json:"role"`
	GithubUsername string `json:"github_username"`
	DisplayName    string `json:"display_name"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResp struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

func main() {
	dbURL := env("DATABASE_URL_GENERAL", "postgres://osship:osship_secret@postgres:5432/osship?sslmode=disable&search_path=general")
	jwtSecret := env("JWT_SECRET", "dev-secret")
	expiryHours := 24
	port := env("PORT", "8081")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	s := &server{pool: pool, jwtSecret: jwtSecret, expiryHours: expiryHours}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(observability.PrometheusMiddleware("auth"))

	r.Get("/health", observability.HealthHandler("auth"))
	r.Get("/metrics", observability.MetricsHandler().ServeHTTP)

	r.Post("/register", s.register)
	r.Post("/login", s.login)
	r.Post("/refresh", s.refresh)
	r.Get("/me", s.me)

	log.Printf("auth listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

type server struct {
	pool        *pgxpool.Pool
	jwtSecret   string
	expiryHours int
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, http.StatusBadRequest)
		return
	}
	role := req.Role
	if role == "" {
		role = "student"
	}
	if role != "student" && role != "mentor" && role != "admin" {
		http.Error(w, `{"error":"invalid role"}`, http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	id := uuid.New().String()
	_, err = s.pool.Exec(r.Context(),
		`INSERT INTO users (id, email, password_hash, role, github_username, display_name) VALUES ($1,$2,$3,$4,$5,$6)`,
		id, req.Email, string(hash), role, nullStr(req.GithubUsername), nullStr(req.DisplayName))
	if err != nil {
		http.Error(w, `{"error":"email already exists"}`, http.StatusConflict)
		return
	}

	token, err := jwtutil.GenerateToken(s.jwtSecret, id, role, req.GithubUsername, s.expiryHours)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, tokenResp{
		Token: token,
		User: User{ID: id, Email: req.Email, Role: role, GithubUsername: req.GithubUsername, DisplayName: req.DisplayName},
	})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	var id, email, role, hash, github, display string
	err := s.pool.QueryRow(r.Context(),
		`SELECT id, email, role, password_hash, COALESCE(github_username,''), COALESCE(display_name,'') FROM users WHERE email=$1`,
		req.Email).Scan(&id, &email, &role, &hash, &github, &display)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	token, err := jwtutil.GenerateToken(s.jwtSecret, id, role, github, s.expiryHours)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, tokenResp{
		Token: token,
		User:  User{ID: id, Email: email, Role: role, GithubUsername: github, DisplayName: display},
	})
}

func (s *server) refresh(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	tokenStr := auth
	if len(auth) > 7 && auth[:7] == "Bearer " {
		tokenStr = auth[7:]
	}
	claims, err := jwtutil.ValidateToken(s.jwtSecret, tokenStr)
	if err != nil {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}
	token, err := jwtutil.GenerateToken(s.jwtSecret, claims.UserID, claims.Role, claims.GithubUsername, s.expiryHours)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var u User
	err := s.pool.QueryRow(r.Context(),
		`SELECT id, email, role, COALESCE(github_username,''), COALESCE(display_name,'') FROM users WHERE id=$1`, userID).
		Scan(&u.ID, &u.Email, &u.Role, &u.GithubUsername, &u.DisplayName)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var _ = time.Now
