package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
)

// Model represents the metadata for a model
type Model struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ModelStore manages models in memory
type ModelStore struct {
	models map[string]Model
	mu     sync.RWMutex
}

// NewModelStore creates a new ModelStore
func NewModelStore() *ModelStore {
	return &ModelStore{
		models: make(map[string]Model),
	}
}

// Create adds a new model
func (s *ModelStore) Create(model Model) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.models[model.Name] = model
}

// Get retrieves a model by name
func (s *ModelStore) Get(name string) (Model, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	model, exists := s.models[name]
	return model, exists
}

// GetAll returns all models
func (s *ModelStore) GetAll() []Model {
	s.mu.RLock()
	defer s.mu.RUnlock()
	models := make([]Model, 0, len(s.models))
	for _, model := range s.models {
		models = append(models, model)
	}
	return models
}

// Update modifies an existing model
func (s *ModelStore) Update(name string, model Model) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.models[name]; !exists {
		return false
	}
	// If name changed, delete old entry
	if name != model.Name {
		delete(s.models, name)
	}
	s.models[model.Name] = model
	return true
}

// Delete removes a model
func (s *ModelStore) Delete(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.models[name]; !exists {
		return false
	}
	delete(s.models, name)
	return true
}

var store *ModelStore

func main() {
	store = NewModelStore()

	router := mux.NewRouter()

	// Routes
	router.HandleFunc("/api/models", createModel).Methods("POST")
	router.HandleFunc("/api/models", listModels).Methods("GET")
	router.HandleFunc("/api/models/{name}", getModel).Methods("GET")
	router.HandleFunc("/api/models/{name}", updateModel).Methods("PUT")
	router.HandleFunc("/api/models/{name}", deleteModel).Methods("DELETE")

	port := ":8080"
	log.Printf("Starting server on %s", port)
	log.Fatal(http.ListenAndServe(port, router))
}

func createModel(w http.ResponseWriter, r *http.Request) {
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
}

func listModels(w http.ResponseWriter, r *http.Request) {
	models := store.GetAll()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func getModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	model, exists := store.Get(name)
	if !exists {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model)
}

func updateModel(w http.ResponseWriter, r *http.Request) {
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
}

func deleteModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	if !store.Delete(name) {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
