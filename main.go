package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/joho/godotenv"
	"github.com/kaylubr/chirpy/internal/database"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	db             database.Queries
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}

	dbQueries := database.New(db)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	apiCfg := apiConfig{fileServerHits: atomic.Int32{}, db: *dbQueries}

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("POST /api/validate_chirp", apiCfg.validateChirpConfig)
	mux.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetMetricsHandler)

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	hits := cfg.fileServerHits.Load()
	page := `
		<html>
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
		</html>
	`
	w.Write(fmt.Appendf(nil, page, hits))
}

func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	platform := os.Getenv("PLATFORM")
	if platform != "dev" {
		respondWithError(w, http.StatusForbidden, "You don't have permission for this operation")
		return
	}
	cfg.fileServerHits.Store(0)
	cfg.db.DeleteAllUsers(r.Context())
}

func (cfg *apiConfig) validateChirpConfig(w http.ResponseWriter, r *http.Request) {
	type response struct {
		CleanedBody string `json:"cleaned_body"`
	}

	type request struct {
		Body string `json:"body"`
	}

	var chirpConfigRequest request
	err := json.NewDecoder(r.Body).Decode(&chirpConfigRequest)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	if len(chirpConfigRequest.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	cleanedBody := cleanSentence(chirpConfigRequest.Body)
	validResponse := response{CleanedBody: cleanedBody}
	respondWithJSON(w, http.StatusOK, validResponse)
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email string `json:"email"`
	}

	var userRequest request
	err := json.NewDecoder(r.Body).Decode(&userRequest)
	if err != nil {
		log.Printf("decode create user request: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := cfg.db.CreateUser(r.Context(), userRequest.Email)
	if err != nil {
		log.Print(err)
		respondWithError(w, http.StatusInternalServerError, "Could not create user")
		return
	}

	userResponse := User{
		ID:        uuid.UUID(user.ID),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}

	respondWithJSON(w, http.StatusCreated, userResponse)
}
