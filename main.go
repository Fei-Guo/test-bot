package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics
var (
	modelsCreatedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "models_created_total",
		Help: "Total number of models created",
	})
	modelsReadTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "models_read_total",
		Help: "Total number of models read",
	})
	modelsUpdatedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "models_updated_total",
		Help: "Total number of models updated",
	})
	modelsDeletedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "models_deleted_total",
		Help: "Total number of models deleted",
	})
)

func init() {
	prometheus.MustRegister(modelsCreatedTotal, modelsReadTotal, modelsUpdatedTotal, modelsDeletedTotal)
}

type Model struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type DB struct {
	conn *sql.DB
}

func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	createModelsTableSQL := `CREATE TABLE IF NOT EXISTS models (name TEXT PRIMARY KEY, url TEXT NOT NULL);`
	if _, err = conn.Exec(createModelsTableSQL); err != nil {
		return nil, err
	}

	createUsersTableSQL := `CREATE TABLE IF NOT EXISTS users (name TEXT PRIMARY KEY, email TEXT NOT NULL);`
	if _, err = conn.Exec(createUsersTableSQL); err != nil {
		return nil, err
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Create(model Model) error {
	_, err := db.conn.Exec("INSERT INTO models (name, url) VALUES (?, ?)", model.Name, model.URL)
	return err
}

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

func (db *DB) Update(name string, model Model) (bool, error) {
	var exists bool
	err := db.conn.QueryRow("SELECT 1 FROM models WHERE name = ?", name).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Transactional update when name changes
	if name != model.Name {
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

func (db *DB) CreateUser(user User) error {
	_, err := db.conn.Exec("INSERT INTO users (name, email) VALUES (?, ?)", user.Name, user.Email)
	return err
}

func (db *DB) GetUser(name string) (User, bool, error) {
	var user User
	err := db.conn.QueryRow("SELECT name, email FROM users WHERE name = ?", name).Scan(&user.Name, &user.Email)
	if err == sql.ErrNoRows {
		return user, false, nil
	}
	if err != nil {
		return user, false, err
	}
	return user, true, nil
}

func (db *DB) GetAllUsers() ([]User, error) {
	rows, err := db.conn.Query("SELECT name, email FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Name, &user.Email); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (db *DB) UpdateUser(name string, user User) (bool, error) {
	var exists bool
	err := db.conn.QueryRow("SELECT 1 FROM users WHERE name = ?", name).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if name != user.Name {
		tx, err := db.conn.Begin()
		if err != nil {
			return false, err
		}
		defer tx.Rollback()

		if _, err = tx.Exec("DELETE FROM users WHERE name = ?", name); err != nil {
			return false, err
		}
		if _, err = tx.Exec("INSERT INTO users (name, email) VALUES (?, ?)", user.Name, user.Email); err != nil {
			return false, err
		}

		if err = tx.Commit(); err != nil {
			return false, err
		}
	} else {
		if _, err = db.conn.Exec("UPDATE users SET email = ? WHERE name = ?", user.Email, name); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (db *DB) DeleteUser(name string) (bool, error) {
	result, err := db.conn.Exec("DELETE FROM users WHERE name = ?", name)
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
var apiToken string

func main() {
	var err error
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "models.db"
	}
	db, err = NewDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	apiToken = os.Getenv("API_TOKEN")
	if apiToken == "" {
		apiToken, err = generateToken()
		if err != nil {
			log.Fatalf("Failed to generate API token: %v", err)
		}
		log.Printf("Generated API token for server")
	}

	router := newRouter()

	port := ":8080"
	log.Printf("Starting server on %s", port)
	log.Fatal(http.ListenAndServe(port, router))
}

func newRouter() *mux.Router {
	router := mux.NewRouter()

	// Public endpoints
	router.HandleFunc("/getkey", getKeyHandler).Methods("GET")
	router.Handle("/metrics", promhttp.Handler()).Methods("GET")

	// Protected API endpoints
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(authMiddleware)

	apiRouter.HandleFunc("/models", createModel).Methods("POST")
	apiRouter.HandleFunc("/models", listModels).Methods("GET")
	apiRouter.HandleFunc("/models/{name}", getModel).Methods("GET")
	apiRouter.HandleFunc("/models/{name}", updateModel).Methods("PUT")
	apiRouter.HandleFunc("/models/{name}", deleteModel).Methods("DELETE")

	apiRouter.HandleFunc("/users", createUser).Methods("POST")
	apiRouter.HandleFunc("/users", listUsers).Methods("GET")
	apiRouter.HandleFunc("/users/{name}", getUser).Methods("GET")
	apiRouter.HandleFunc("/users/{name}", updateUser).Methods("PUT")
	apiRouter.HandleFunc("/users/{name}", deleteUser).Methods("DELETE")

	return router
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func getKeyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": apiToken,
	})
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-API-Key")
		if token == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token == "" || token != apiToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func createUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if user.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if user.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}
	if err := db.CreateUser(user); err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := db.GetAllUsers()
	if err != nil {
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func getUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	user, exists, err := db.GetUser(vars["name"])
	if err != nil {
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if user.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if user.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}
	updated, err := db.UpdateUser(name, user)
	if err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}
	if !updated {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deleted, err := db.DeleteUser(vars["name"])
	if err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}
	if !deleted {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		http.Error(w, "Failed to create model", http.StatusInternalServerError)
		return
	}
	modelsCreatedTotal.Inc()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(model)
}

func listModels(w http.ResponseWriter, r *http.Request) {
	models, err := db.GetAll()
	if err != nil {
		http.Error(w, "Failed to list models", http.StatusInternalServerError)
		return
	}
	modelsReadTotal.Inc()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func getModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	model, exists, err := db.Get(vars["name"])
	if err != nil {
		http.Error(w, "Failed to get model", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}
	modelsReadTotal.Inc()
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
		http.Error(w, "Failed to update model", http.StatusInternalServerError)
		return
	}
	if !updated {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}
	modelsUpdatedTotal.Inc()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model)
}

func deleteModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deleted, err := db.Delete(vars["name"])
	if err != nil {
		http.Error(w, "Failed to delete model", http.StatusInternalServerError)
		return
	}
	if !deleted {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}
	modelsDeletedTotal.Inc()
	w.WriteHeader(http.StatusNoContent)
}
