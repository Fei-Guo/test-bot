package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func setupTestServer(t *testing.T) (*httptest.Server, *DB, string) {
	dbFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db: %v", err)
	}
	dbFile.Close()

	testDB, err := NewDB(dbFile.Name())
	if err != nil {
		os.Remove(dbFile.Name())
		t.Fatalf("Failed to create database: %v", err)
	}

	// Use the same global DB and router setup as the main server.
	apiToken = "test-secret"
	db = testDB

	router := newRouter()

	return httptest.NewServer(router), testDB, dbFile.Name()
}

func newTestJWT(t *testing.T) string {
	t.Helper()

	token, err := generateJWT("test-user", "")
	if err != nil {
		t.Fatalf("Failed to generate test JWT: %v", err)
	}
	return token
}

func newTestJWTForName(t *testing.T, name string) string {
	t.Helper()

	token, err := generateJWT(name, "")
	if err != nil {
		t.Fatalf("Failed to generate test JWT: %v", err)
	}
	return token
}

func ensureTestUser(t *testing.T, db *DB, name string) {
	t.Helper()

	if err := db.CreateUser(User{Name: name, Email: name + "@example.com"}); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
}

func TestCreateModel(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	defer os.Remove(dbPath)

	ensureTestUser(t, db, "test-user")

	model := Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"}
	body, _ := json.Marshal(model)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/models", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", newTestJWT(t))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}
}

func TestCreateWithoutName(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	defer os.Remove(dbPath)

	ensureTestUser(t, db, "test-user")

	model := Model{URL: "https://example.com"}
	body, _ := json.Marshal(model)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/models", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", newTestJWT(t))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", resp.StatusCode)
	}
}

func TestGetModel(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer os.Remove(dbPath)
	defer db.Close()

	ensureTestUser(t, db, "test-user")
	db.Create(Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"})

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/models/gpt-4", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", newTestJWT(t))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestDeleteModel(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer os.Remove(dbPath)
	defer db.Close()

	ensureTestUser(t, db, "test-user")
	db.Create(Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"})

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/models/gpt-4", nil)
	req.Header.Set("X-API-Key", newTestJWT(t))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", resp.StatusCode)
	}
}

func TestGetKeyReturnsToken(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer os.Remove(dbPath)
	defer db.Close()

	resp, err := http.Get(server.URL + "/getkey?name=alice")
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if body.Token == "" {
		t.Fatal("Expected non-empty token")
	}
	if body.Name != "alice" {
		t.Fatalf("Expected name 'alice', got '%s'", body.Name)
	}

	claims, err := parseAndValidateJWT(body.Token)
	if err != nil {
		t.Fatalf("Token validation failed: %v", err)
	}
	if claims.Name != "alice" {
		t.Errorf("Expected name 'alice', got '%s'", claims.Name)
	}
}

func TestUnauthorizedWithoutToken(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer os.Remove(dbPath)
	defer db.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/models", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}
}

func TestUnauthorizedWithWrongToken(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer os.Remove(dbPath)
	defer db.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/models", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", "invalid.token.value")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}
}

func TestModelRequiresRegisteredUser(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer os.Remove(dbPath)
	defer db.Close()

	model := Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"}
	body, _ := json.Marshal(model)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/models", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", newTestJWTForName(t, "unregistered-user"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateUser(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	defer os.Remove(dbPath)

	user := User{Name: "alice", Email: "alice@example.com"}
	body, _ := json.Marshal(user)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/users", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", newTestJWT(t))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}
}

func TestGetUser(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer os.Remove(dbPath)
	defer db.Close()

	if err := db.CreateUser(User{Name: "alice", Email: "alice@example.com"}); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/users/alice", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", newTestJWT(t))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestDeleteUser(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer os.Remove(dbPath)
	defer db.Close()

	if err := db.CreateUser(User{Name: "alice", Email: "alice@example.com"}); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/users/alice", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", newTestJWT(t))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", resp.StatusCode)
	}
}
