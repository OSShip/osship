//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func baseURL() string {
	if v := os.Getenv("INTEGRATION_BASE_URL"); v != "" {
		return v
	}
	return "http://localhost"
}

func TestGatewayAuthUsersFlow(t *testing.T) {
	api := baseURL() + "/api/v1"
	email := fmt.Sprintf("go-test-%d@osship.test", time.Now().UnixNano())

	regBody, _ := json.Marshal(map[string]string{
		"email":        email,
		"password":     "secret123",
		"role":         "student",
		"display_name": "Go Integration",
	})
	regResp, err := http.Post(api+"/auth/register", "application/json", bytes.NewReader(regBody))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer regResp.Body.Close()
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("register status %d", regResp.StatusCode)
	}

	var reg struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(regResp.Body).Decode(&reg); err != nil {
		t.Fatal(err)
	}
	if reg.Token == "" || reg.User.ID == "" {
		t.Fatal("missing token or user id")
	}

	meReq, _ := http.NewRequest(http.MethodGet, api+"/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+reg.Token)
	meResp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("me status %d", meResp.StatusCode)
	}

	unauth, err := http.Get(api + "/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	defer unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", unauth.StatusCode)
	}

	profileResp, err := http.Get(api + "/users/" + reg.User.ID + "/profile")
	if err != nil {
		t.Fatal(err)
	}
	defer profileResp.Body.Close()
	if profileResp.StatusCode != http.StatusOK {
		t.Fatalf("profile status %d", profileResp.StatusCode)
	}
}

func TestRateLimitAuthLogin(t *testing.T) {
	api := baseURL() + "/api/v1"
	var got429 bool
	for i := 0; i < 12; i++ {
		body, _ := json.Marshal(map[string]string{
			"email":    fmt.Sprintf("rl-go-%d-%d@osship.test", i, time.Now().UnixNano()),
			"password": "x",
		})
		resp, err := http.Post(api+"/auth/register", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}
	if !got429 {
		t.Fatal("expected 429 after exceeding auth_login rate limit")
	}
}
