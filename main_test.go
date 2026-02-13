package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func setupTestServer() (*httptest.Server, *ModelStore) {
	store := NewModelStore()
	router := mux.NewRouter()

	router.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		var model Model
		if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if model.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		store.Create(model)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(model)
	}).Methods("POST")

	router.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		models := store.GetAll()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models)
	}).Methods("GET")

	router.HandleFunc("/api/models/{name}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		name := vars["name"]
		model, exists := store.Get(name)
		if !exists {
			http.Error(w, "model not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model)
	}).Methods("GET")

	router.HandleFunc("/api/models/{name}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		name := vars["name"]
		var model Model
		if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if model.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if !store.Update(name, model) {
			http.Error(w, "model not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model)
	}).Methods("PUT")

	router.HandleFunc("/api/models/{name}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		name := vars["name"]
		if !store.Delete(name) {
			http.Error(w, "model not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}).Methods("DELETE")

	return httptest.NewServer(router), store
}

func TestCreateModel(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	model := Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"}
	body, _ := json.Marshal(model)

	resp, err := http.Post(server.URL+"/api/models", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var result Model
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Name != model.Name || result.URL != model.URL {
		t.Errorf("Expected %v, got %v", model, result)
	}
}

func TestCreateModelWithoutName(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	model := Model{URL: "https://example.com"}
	body, _ := json.Marshal(model)

	resp, err := http.Post(server.URL+"/api/models", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestListModels(t *testing.T) {
	server, store := setupTestServer()
	defer server.Close()

	// Add some models
	store.Create(Model{Name: "model1", URL: "https://example.com/1"})
	store.Create(Model{Name: "model2", URL: "https://example.com/2"})

	resp, err := http.Get(server.URL + "/api/models")
	if err != nil {
		t.Fatalf("Failed to list models: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var models []Model
	json.NewDecoder(resp.Body).Decode(&models)
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
}

func TestGetModel(t *testing.T) {
	server, store := setupTestServer()
	defer server.Close()

	store.Create(Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"})

	resp, err := http.Get(server.URL + "/api/models/gpt-4")
	if err != nil {
		t.Fatalf("Failed to get model: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result Model
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Name != "gpt-4" {
		t.Errorf("Expected name 'gpt-4', got '%s'", result.Name)
	}
}

func TestGetNonExistentModel(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/models/nonexistent")
	if err != nil {
		t.Fatalf("Failed to get model: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestUpdateModel(t *testing.T) {
	server, store := setupTestServer()
	defer server.Close()

	store.Create(Model{Name: "gpt-4", URL: "https://old-url.com"})

	updated := Model{Name: "gpt-4", URL: "https://new-url.com"}
	body, _ := json.Marshal(updated)

	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/models/gpt-4", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to update model: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	model, _ := store.Get("gpt-4")
	if model.URL != "https://new-url.com" {
		t.Errorf("Expected URL 'https://new-url.com', got '%s'", model.URL)
	}
}

func TestDeleteModel(t *testing.T) {
	server, store := setupTestServer()
	defer server.Close()

	store.Create(Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"})

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/models/gpt-4", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete model: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	_, exists := store.Get("gpt-4")
	if exists {
		t.Error("Model should have been deleted")
	}
}
