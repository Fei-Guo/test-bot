package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	serverTimeout = 30 * time.Second
	retryAttempts = 5
	retryDelay    = 100 * time.Millisecond
)

var e2eToken string

// E2E test suite for the model server
func TestE2E(t *testing.T) {
	// Build the server binary
	serverBinary := buildServer(t)
	defer os.Remove(serverBinary)

	// Start the server
	baseURL, cleanup := startServer(t, serverBinary)
	defer cleanup()

	e2eToken = fetchAPIToken(t, baseURL)

	t.Run("PrometheusFormat", func(t *testing.T) {
		testPrometheusFormat(t, baseURL)
	})

	t.Run("InitialMetrics", func(t *testing.T) {
		testInitialMetrics(t, baseURL)
	})

	t.Run("CreateModels", func(t *testing.T) {
		testCreateModels(t, baseURL)
	})

	t.Run("VerifyCreatedMetric", func(t *testing.T) {
		testVerifyCreatedMetric(t, baseURL)
	})

	t.Run("ListModels", func(t *testing.T) {
		testListModels(t, baseURL)
	})

	t.Run("GetModel", func(t *testing.T) {
		testGetModel(t, baseURL)
	})

	t.Run("VerifyReadMetric", func(t *testing.T) {
		testVerifyReadMetric(t, baseURL)
	})

	t.Run("NonExistentModel", func(t *testing.T) {
		testNonExistentModel(t, baseURL)
	})

	t.Run("UpdateModel", func(t *testing.T) {
		testUpdateModel(t, baseURL)
	})

	t.Run("DeleteModel", func(t *testing.T) {
		testDeleteModel(t, baseURL)
	})

	t.Run("CreateUsers", func(t *testing.T) {
		testCreateUsers(t, baseURL)
	})

	t.Run("ListUsers", func(t *testing.T) {
		testListUsers(t, baseURL)
	})

	t.Run("GetUser", func(t *testing.T) {
		testGetUser(t, baseURL)
	})

	t.Run("DeleteUser", func(t *testing.T) {
		testDeleteUser(t, baseURL)
	})
}

func buildServer(t *testing.T) string {
	t.Helper()
	serverBinary := "model-server-e2e"
	cmd := exec.Command("go", "build", "-o", serverBinary, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build server: %v\nOutput: %s", err, output)
	}
	return serverBinary
}

func startServer(t *testing.T, binary string) (string, func()) {
	t.Helper()

	// Create temp db file
	dbFile, err := os.CreateTemp("", "e2e-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db: %v", err)
	}
	dbFile.Close()

	// Start server on random port
	cmd := exec.Command("./" + binary)
	cmd.Env = append(os.Environ(), fmt.Sprintf("DB_PATH=%s", dbFile.Name()))
	if err := cmd.Start(); err != nil {
		os.Remove(dbFile.Name())
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give server a moment to start, then verify it's running
	baseURL := "http://localhost:8080"
	start := time.Now()
	for time.Since(start) < serverTimeout {
		if isServerReady(baseURL) {
			return baseURL, func() {
				cmd.Process.Kill()
				cmd.Wait()
				os.Remove(dbFile.Name())
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	cmd.Process.Kill()
	cmd.Wait()
	os.Remove(dbFile.Name())
	t.Fatal("Server failed to start within timeout")
	return "", nil
}

func isServerReady(baseURL string) bool {
	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func fetchAPIToken(t *testing.T, baseURL string) string {
	t.Helper()

	resp, err := http.Get(baseURL + "/getkey")
	if err != nil {
		t.Fatalf("Failed to get API token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 when getting token, got %d", resp.StatusCode)
	}

	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Failed to decode token response: %v", err)
	}

	if body.Token == "" {
		t.Fatal("Received empty API token")
	}

	return body.Token
}

func testPrometheusFormat(t *testing.T, baseURL string) {
	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	metrics := string(body)

	// Check for Prometheus HELP and TYPE annotations
	if !strings.Contains(metrics, "# HELP") || !strings.Contains(metrics, "# TYPE") {
		t.Error("Prometheus format validation failed: missing HELP or TYPE annotations")
	}
	if !strings.Contains(metrics, "models_") {
		t.Error("Prometheus format validation failed: no model metrics found")
	}
}

func testInitialMetrics(t *testing.T, baseURL string) {
	checkMetric(t, baseURL, "models_created_total", 0)
	checkMetric(t, baseURL, "models_read_total", 0)
	checkMetric(t, baseURL, "models_updated_total", 0)
	checkMetric(t, baseURL, "models_deleted_total", 0)
}

func testCreateModels(t *testing.T, baseURL string) {
	// Create gpt-4 model
	model1 := Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"}
	resp := httpRequest(t, "POST", baseURL+"/api/models", 201, model1)
	var result1 Model
	json.Unmarshal(resp, &result1)
	if result1.Name != "gpt-4" || result1.URL != model1.URL {
		t.Error("Create response validation failed for gpt-4")
	}

	// Create claude-3 model
	model2 := Model{Name: "claude-3", URL: "https://api.anthropic.com/v1/models/claude-3"}
	resp = httpRequest(t, "POST", baseURL+"/api/models", 201, model2)
	var result2 Model
	json.Unmarshal(resp, &result2)
	if result2.Name != "claude-3" || result2.URL != model2.URL {
		t.Error("Create response validation failed for claude-3")
	}
}

func testVerifyCreatedMetric(t *testing.T, baseURL string) {
	checkMetric(t, baseURL, "models_created_total", 2)
}

func testListModels(t *testing.T, baseURL string) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/models", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", e2eToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to list models: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var models []Model
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
}

func testGetModel(t *testing.T, baseURL string) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/models/gpt-4", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", e2eToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get model: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var model Model
	if err := json.NewDecoder(resp.Body).Decode(&model); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if model.Name != "gpt-4" {
		t.Errorf("Expected name 'gpt-4', got '%s'", model.Name)
	}
	if model.URL != "https://api.openai.com/v1/models/gpt-4" {
		t.Errorf("Expected URL 'https://api.openai.com/v1/models/gpt-4', got '%s'", model.URL)
	}
}

func testVerifyReadMetric(t *testing.T, baseURL string) {
	// Should be 2 reads (1 list + 1 get)
	checkMetric(t, baseURL, "models_read_total", 2)
}

func testNonExistentModel(t *testing.T, baseURL string) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/models/nonexistent", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", e2eToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}
}

func testUpdateModel(t *testing.T, baseURL string) {
	// Update gpt-4 model
	model := Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4-turbo"}
	body, _ := json.Marshal(model)
	req, _ := http.NewRequest(http.MethodPut, baseURL+"/api/models/gpt-4", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", e2eToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to update model: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	// Verify update persisted
	req2, err := http.NewRequest(http.MethodGet, baseURL+"/api/models/gpt-4", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req2.Header.Set("X-API-Key", e2eToken)

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Failed to get model after update: %v", err)
	}
	defer resp2.Body.Close()

	var result Model
	json.NewDecoder(resp2.Body).Decode(&result)
	if result.URL != "https://api.openai.com/v1/models/gpt-4-turbo" {
		t.Errorf("Update persistence failed: expected URL 'https://api.openai.com/v1/models/gpt-4-turbo', got '%s'", result.URL)
	}

	// Check updated metric
	checkMetric(t, baseURL, "models_updated_total", 1)
}

func testDeleteModel(t *testing.T, baseURL string) {
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/api/models/claude-3", nil)
	req.Header.Set("X-API-Key", e2eToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete model: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("Expected 204, got %d", resp.StatusCode)
	}

	// Check deleted metric
	checkMetric(t, baseURL, "models_deleted_total", 1)

	// Verify deletion
	req2, err := http.NewRequest(http.MethodGet, baseURL+"/api/models/claude-3", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req2.Header.Set("X-API-Key", e2eToken)

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 after delete, got %d", resp2.StatusCode)
	}
}

func testCreateUsers(t *testing.T, baseURL string) {
	user1 := User{Name: "alice", Email: "alice@example.com"}
	resp := httpRequest(t, http.MethodPost, baseURL+"/api/users", http.StatusCreated, user1)
	var result1 User
	if err := json.Unmarshal(resp, &result1); err != nil {
		t.Fatalf("Failed to decode create user response: %v", err)
	}
	if result1.Name != user1.Name || result1.Email != user1.Email {
		t.Error("Create response validation failed for alice")
	}

	user2 := User{Name: "bob", Email: "bob@example.com"}
	resp = httpRequest(t, http.MethodPost, baseURL+"/api/users", http.StatusCreated, user2)
	var result2 User
	if err := json.Unmarshal(resp, &result2); err != nil {
		t.Fatalf("Failed to decode create user response: %v", err)
	}
	if result2.Name != user2.Name || result2.Email != user2.Email {
		t.Error("Create response validation failed for bob")
	}
}

func testListUsers(t *testing.T, baseURL string) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/users", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", e2eToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to list users: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func testGetUser(t *testing.T, baseURL string) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/users/alice", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", e2eToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if user.Name != "alice" {
		t.Errorf("Expected name 'alice', got '%s'", user.Name)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Expected email 'alice@example.com', got '%s'", user.Email)
	}
}

func testDeleteUser(t *testing.T, baseURL string) {
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/users/bob", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", e2eToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("Expected 204, got %d", resp.StatusCode)
	}

	_ = httpRequest(t, http.MethodGet, baseURL+"/api/users/bob", http.StatusNotFound, nil)
}

func httpRequest(t *testing.T, method, url string, expectedCode int, data interface{}) []byte {
	t.Helper()

	var body io.Reader
	if data != nil {
		jsonData, _ := json.Marshal(data)
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if e2eToken != "" {
		req.Header.Set("X-API-Key", e2eToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedCode {
		t.Fatalf("Expected HTTP %d, got %d", expectedCode, resp.StatusCode)
	}

	returnBody, _ := io.ReadAll(resp.Body)
	return returnBody
}

func checkMetric(t *testing.T, baseURL, metricName string, expected float64) {
	t.Helper()

	for i := 0; i < retryAttempts; i++ {
		value, err := getMetric(baseURL, metricName)
		if err == nil && value == expected {
			return
		}
		if i < retryAttempts-1 {
			time.Sleep(retryDelay)
		}
	}

	value, err := getMetric(baseURL, metricName)
	if err != nil {
		t.Errorf("Failed to get metric %s: %v", metricName, err)
	} else if value != expected {
		t.Errorf("Metric %s = %v, expected %v", metricName, value, expected)
	}
}

func getMetric(baseURL, name string) (float64, error) {
	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(string(body), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, name+" ") {
			var value float64
			fmt.Sscanf(line, name+" %f", &value)
			return value, nil
		}
	}

	return 0, fmt.Errorf("metric %s not found", name)
}
