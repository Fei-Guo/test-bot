package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

// Model represents the metadata for a model
type Model struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// DB represents the database connection
type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection and initializes the schema
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create table if not exists
	createTableSQL := `CREATE TABLE IF NOT EXISTS models (
		name TEXT PRIMARY KEY,
		url TEXT NOT NULL
	);`
	_, err = conn.Exec(createTableSQL)
	if err != nil {
		return nil, err
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Create adds a new model
func (db *DB) Create(model Model) error {
	_, err := db.conn.Exec("INSERT INTO models (name, url) VALUES (?, ?)", model.Name, model.URL)
	return err
}

// Get retrieves a model by name
func (db *DB) Get(name string) (Model, bool, error) {
	var model Model
	err := db.conn.QueryRow("SELECT name, url FROM models WHERE name = ?", name).Scan(&model.Name, &model.URL)
	if err == sql.ErrNoRows {
		return model, false, nil
	}
	if err != nil {
		return model, false, err
	}
	return model, true, nil
}

// GetAll returns all models
func (db *DB) GetAll() ([]Model, error) {
	rows, err := db.conn.Query("SELECT name, url FROM models")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []Model
	for rows.Next() {
		var model Model
		if err := rows.Scan(&model.Name, &model.URL); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

// Update modifies an existing model
func (db *DB) Update(name string, model Model) (bool, error) {
	// Check if model exists
	var exists bool
	err := db.conn.QueryRow("SELECT 1 FROM models WHERE name = ?", name).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// If name changed, we need to delete old and insert new (since name is PK)
	if name != model.Name {
		// Use transaction for atomicity
		tx, err := db.conn.Begin()
		if err != nil {
			return false, err
		}
		defer tx.Rollback()

		_, err = tx.Exec("DELETE FROM models WHERE name = ?", name)
		if err != nil {
			return false, err
		}
		_, err = tx.Exec("INSERT INTO models (name, url) VALUES (?, ?)", model.Name, model.URL)
		if err != nil {
			return false, err
		}

		if err = tx.Commit(); err != nil {
			return false, err
		}
	} else {
		_, err = db.conn.Exec("UPDATE models SET url = ? WHERE name = ?", model.URL, name)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

// Delete removes a model
func (db *DB) Delete(name string) (bool, error) {
	result, err := db.conn.Exec("DELETE FROM models WHERE name = ?", name)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

var db *DB

func main() {
	var err error
	db, err = NewDB("models.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

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

	if err := db.Create(model); err != nil {
		log.Printf("Failed to create model: %v", err)
		http.Error(w, "Failed to create model", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(model)
}

func listModels(w http.ResponseWriter, r *http.Request) {
	models, err := db.GetAll()
	if err != nil {
		log.Printf("Failed to list models: %v", err)
		http.Error(w, "Failed to list models", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func getModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	model, exists, err := db.Get(name)
	if err != nil {
		log.Printf("Failed to get model: %v", err)
		http.Error(w, "Failed to get model", http.StatusInternalServerError)
		return
	}
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

	updated, err := db.Update(name, model)
	if err != nil {
		log.Printf("Failed to update model: %v", err)
		http.Error(w, "Failed to update model", http.StatusInternalServerError)
		return
	}
	if !updated {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model)
}

func deleteModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	deleted, err := db.Delete(name)
	if err != nil {
		log.Printf("Failed to delete model: %v", err)
		http.Error(w, "Failed to delete model", http.StatusInternalServerError)
		return
	}
	if !deleted {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
