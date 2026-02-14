package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
)

func setupTestServer(t *testing.T) (*httptest.Server, *DB, string) {
	dbFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db: %v", err)
	}
	dbFile.Close()

	db, err := NewDB(dbFile.Name())
	if err != nil {
		os.Remove(dbFile.Name())
		t.Fatalf("Failed to create database: %v", err)
	}

	router := mux.NewRouter()

	router.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		var model Model
		if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if model.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if err := db.Create(model); err != nil {
			http.Error(w, "create failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(model)
	}).Methods("POST")

	router.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		models, err := db.GetAll()
		if err != nil {
			http.Error(w, "list failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models)
	}).Methods("GET")

	router.HandleFunc("/api/models/{name}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		model, exists, err := db.Get(vars["name"])
		if err != nil {
			http.Error(w, "get failed", http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model)
	}).Methods("GET")

	router.HandleFunc("/api/models/{name}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		var model Model
		if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if model.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		updated, err := db.Update(vars["name"], model)
		if err != nil {
			http.Error(w, "update failed", http.StatusInternalServerError)
			return
		}
		if !updated {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model)
	}).Methods("PUT")

	router.HandleFunc("/api/models/{name}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		deleted, err := db.Delete(vars["name"])
		if err != nil {
			http.Error(w, "delete failed", http.StatusInternalServerError)
			return
		}
		if !deleted {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}).Methods("DELETE")

	return httptest.NewServer(router), db, dbFile.Name()
}

func TestCreateModel(t *testing.T) {
	server, db, dbPath := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	defer os.Remove(dbPath)

	model := Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"}
	body, _ := json.Marshal(model)

	resp, err := http.Post(server.URL+"/api/models", "application/json", bytes.NewBuffer(body))
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

	model := Model{URL: "https://example.com"}
	body, _ := json.Marshal(model)

	resp, err := http.Post(server.URL+"/api/models", "application/json", bytes.NewBuffer(body))
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

	db.Create(Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"})

	resp, err := http.Get(server.URL + "/api/models/gpt-4")
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

	db.Create(Model{Name: "gpt-4", URL: "https://api.openai.com/v1/models/gpt-4"})

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/models/gpt-4", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", resp.StatusCode)
	}
}
