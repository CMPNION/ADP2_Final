package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_HealthAndReady(t *testing.T) {
	s := NewServer(Config{
		HTTPAddr:           ":0",
		JWTSecret:          "test-secret",
		RateLimitPerMinute: 1000,
	}, nil, nil, nil, nil)
	h := s.Router()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected /healthz 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected /readyz 200, got %d", rr.Code)
	}
}

func TestServer_AuthRegisterLoginFlow(t *testing.T) {
	s := NewServer(Config{
		HTTPAddr:           ":0",
		JWTSecret:          "test-secret",
		RateLimitPerMinute: 1000,
	}, nil, nil, nil, nil)
	h := s.Router()

	registerBody, _ := json.Marshal(map[string]string{
		"username": "demo-test",
		"password": "pass-test",
		"role":     "admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected register 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	loginBody, _ := json.Marshal(map[string]string{
		"username": "demo-test",
		"password": "pass-test",
	})
	req = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var out map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}
	if out["token"] == "" {
		t.Fatalf("expected jwt token in login response, got %v", out)
	}
}
