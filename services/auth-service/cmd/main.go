package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/bcrypt"
)

type user struct {
	Username     string `json:"username"`
	PasswordHash []byte
	Role         string `json:"role"`
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func main() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("warning: JWT_SECRET not set, using default dev secret")
		jwtSecret = "dev-jwt-secret"
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:pass@localhost:5435/identity_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("auth db open: %v", err)
	}
	defer db.Close()
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		log.Fatalf("auth db ping: %v", err)
	}

	mux := http.NewServeMux()
	httpRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_http_requests_total",
			Help: "Total HTTP requests handled by auth service.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "auth_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds for auth service.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)

	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	withCORS := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
			w.Header().Set("Access-Control-Max-Age", "600")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			h(w, r)
		}
	}

	mux.HandleFunc("/auth/register", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ Username, Password, Role string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.Username == "" || req.Password == "" {
			http.Error(w, "username/password required", http.StatusBadRequest)
			return
		}
		if req.Role == "" {
			req.Role = "user"
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		_, err = db.ExecContext(r.Context(), `
			INSERT INTO users (username, password_hash, role)
			VALUES ($1, $2, $3)
		`, req.Username, hash, req.Role)
		if err != nil {
			http.Error(w, "user exists", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"username": req.Username})
	}))

	mux.HandleFunc("/auth/login", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ Username, Password string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		var u user
		err := db.QueryRowContext(r.Context(), `
			SELECT username, password_hash, role
			FROM users
			WHERE username = $1
		`, req.Username).Scan(&u.Username, &u.PasswordHash, &u.Role)
		if err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(req.Password)); err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		claims := jwt.MapClaims{
			"user_id": u.Username,
			"role":    u.Role,
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := tok.SignedString([]byte(jwtSecret))
		if err != nil {
			http.Error(w, "sign failed", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": signed})
	}))

	mux.HandleFunc("/auth/me", withCORS(func(w http.ResponseWriter, r *http.Request) {
		tokStr := r.Header.Get("Authorization")
		if tokStr == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		if len(tokStr) > 7 && tokStr[:7] == "Bearer " {
			tokStr = tokStr[7:]
		}
		tok, err := jwt.Parse(tokStr, func(*jwt.Token) (any, error) { return []byte(jwtSecret), nil })
		if err != nil || !tok.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		if claims, ok := tok.Claims.(jwt.MapClaims); ok {
			_ = json.NewEncoder(w).Encode(map[string]any{"claims": claims})
			return
		}
		http.Error(w, "invalid token", http.StatusUnauthorized)
	}))

	top := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		w.Header().Set("Access-Control-Max-Age", "600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(http.StatusOK)).Inc()
			httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(time.Since(start).Seconds())
			return
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		mux.ServeHTTP(rec, r)
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(rec.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(time.Since(start).Seconds())
	})

	log.Printf("auth-service listening on %s", addr)
	if err := http.ListenAndServe(addr, top); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
