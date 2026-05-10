package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type user struct{
	Username string `json:"username"`
	PasswordHash []byte
	Role string `json:"role"`
}

var (
	mu sync.RWMutex
	users = map[string]user{}
)

func main(){
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("warning: JWT_SECRET not set, using default dev secret")
		jwtSecret = "dev-jwt-secret"
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" { addr = ":8090" }

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.WriteHeader(200); w.Write([]byte("ok")) })

	// simple CORS wrapper (kept for per-handler compatibility)
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

	mux.HandleFunc("/auth/register", withCORS(func(w http.ResponseWriter, r *http.Request){
		if r.Method != http.MethodPost { http.Error(w,"method not allowed", http.StatusMethodNotAllowed); return }
		var req struct{ Username, Password, Role string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w,"bad json", http.StatusBadRequest); return }
		if req.Username=="" || req.Password=="" { http.Error(w,"username/password required", http.StatusBadRequest); return }
		mu.Lock()
		defer mu.Unlock()
		if _,ok := users[req.Username]; ok { http.Error(w,"user exists", http.StatusConflict); return }
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil { http.Error(w,"internal", http.StatusInternalServerError); return }
		users[req.Username] = user{Username: req.Username, PasswordHash: hash, Role: req.Role}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"username": req.Username})
	}))

	mux.HandleFunc("/auth/login", withCORS(func(w http.ResponseWriter, r *http.Request){
		if r.Method != http.MethodPost { http.Error(w,"method not allowed", http.StatusMethodNotAllowed); return }
		var req struct{ Username, Password string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w,"bad json", http.StatusBadRequest); return }
		mu.RLock()
		u, ok := users[req.Username]
		mu.RUnlock()
		if !ok { http.Error(w,"invalid credentials", http.StatusUnauthorized); return }
		if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(req.Password)); err != nil { http.Error(w,"invalid credentials", http.StatusUnauthorized); return }
		claims := jwt.MapClaims{"user_id": u.Username, "role": u.Role, "exp": time.Now().Add(24*time.Hour).Unix()}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := tok.SignedString([]byte(jwtSecret))
		if err != nil { http.Error(w,"sign failed", http.StatusInternalServerError); return }
		json.NewEncoder(w).Encode(map[string]string{"token": signed})
	}))

	mux.HandleFunc("/auth/me", withCORS(func(w http.ResponseWriter, r *http.Request){
		tokStr := r.Header.Get("Authorization")
		if tokStr=="" { http.Error(w,"missing auth", http.StatusUnauthorized); return }
		if len(tokStr) > 7 && tokStr[:7]=="Bearer " { tokStr = tokStr[7:] }
		tok, err := jwt.Parse(tokStr, func(t *jwt.Token)(any, error){ return []byte(jwtSecret), nil })
		if err!=nil || !tok.Valid { http.Error(w,"invalid token", http.StatusUnauthorized); return }
		if claims, ok := tok.Claims.(jwt.MapClaims); ok {
			json.NewEncoder(w).Encode(map[string]any{"claims": claims})
			return
		}
		http.Error(w,"invalid token", http.StatusUnauthorized)
	}))

	// top-level handler to ensure CORS headers are present for all routes and handle preflight
	top := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		// always set permissive CORS for demo
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		w.Header().Set("Access-Control-Max-Age", "600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
	})

	log.Printf("auth-service listening on %s", addr)
	if err := http.ListenAndServe(addr, top); err!=nil { log.Fatalf("serve: %v", err) }
}
