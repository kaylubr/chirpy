package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/kaylubr/chirpy/internal/auth"
	"github.com/kaylubr/chirpy/internal/database"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	db             database.Queries
	jwtSecret      string
}

type User struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func main() {
	godotenv.Load()

	jwtSecret := os.Getenv("SECRET")
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

	apiCfg := apiConfig{
		fileServerHits: atomic.Int32{},
		db:             *dbQueries,
		jwtSecret:      jwtSecret,
	}

	// File server
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))

	// Health check
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Users
	mux.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	mux.HandleFunc("PUT /api/users", apiCfg.createUpdateUserHandler)
	mux.HandleFunc("POST /api/login", apiCfg.createLoginHandler)

	// Chirps
	mux.HandleFunc("GET /api/chirps", apiCfg.handlerChirpsRetrieve)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handlerChirpGet)
	mux.HandleFunc("POST /api/chirps", apiCfg.createChirpsHandler)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.deleteChirpsHandler)

	// Refresh tokens
	mux.HandleFunc("POST /api/refresh", apiCfg.handleRefreshToken)
	mux.HandleFunc("POST /api/revoke", apiCfg.handleRevokeToken)

	// Admin
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

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	type UserRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var userRequest UserRequest
	err := json.NewDecoder(r.Body).Decode(&userRequest)
	if err != nil {
		log.Printf("decode create user request: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	hashedPassword, err := auth.HashPassword(userRequest.Password)
	if err != nil {
		log.Print(err)
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	dbUser := database.CreateUserParams{Email: userRequest.Email, HashedPassword: hashedPassword}
	user, err := cfg.db.CreateUser(r.Context(), dbUser)
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

func (cfg *apiConfig) createChirpsHandler(w http.ResponseWriter, r *http.Request) {
	type chirpRequest struct {
		Body string `json:"body"`
	}

	type chirpResponse struct {
		chirpRequest
		ID        uuid.UUID `json:"id"`
		UserID    uuid.UUID `json:"user_id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	requestToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var chirpBody chirpRequest
	err = json.NewDecoder(r.Body).Decode(&chirpBody)
	if err != nil {
		log.Printf("decode create user request: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	cleanedChirp, err := validateChirp(w, chirpBody.Body)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	chirpBody.Body = cleanedChirp

	jwtID, err := auth.ValidateJWT(requestToken, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chirpQuery := database.CreateChripParams{
		UserID: jwtID,
		Body:   chirpBody.Body,
	}

	c, err := cfg.db.CreateChrip(r.Context(), chirpQuery)
	if err != nil {
		log.Print(err)
		respondWithError(w, http.StatusBadRequest, "Could not create chirp")
		return
	}

	response := chirpResponse{
		ID:        c.ID,
		Body:      c.Body,
		UserID:    c.UserID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}

	respondWithJSON(w, http.StatusCreated, response)
}

func (cfg *apiConfig) handlerChirpsRetrieve(w http.ResponseWriter, r *http.Request) {
	dbChirps, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		log.Print(err)
		respondWithError(w, http.StatusInternalServerError, "Cannot get chirps")
		return
	}
	chirpList := make([]Chirp, 0, len(dbChirps))
	for _, c := range dbChirps {
		chirps := Chirp{
			ID:        c.ID,
			Body:      c.Body,
			UserID:    c.UserID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		}
		chirpList = append(chirpList, chirps)
	}
	respondWithJSON(w, http.StatusOK, chirpList)
}

func (cfg *apiConfig) handlerChirpGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(id)
	if err != nil {
		log.Print(err)
		respondWithError(w, http.StatusBadRequest, "Cannot parse the ID")
		return
	}

	dbChirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		log.Println(err)
		respondWithError(w, http.StatusNotFound, "No chirp with that user id")
		return
	}

	response := Chirp{
		ID:        dbChirp.ID,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
	}
	respondWithJSON(w, http.StatusOK, response)
}

func (cfg *apiConfig) createLoginHandler(w http.ResponseWriter, r *http.Request) {
	type UserRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var userRequest UserRequest
	err := json.NewDecoder(r.Body).Decode(&userRequest)
	if err != nil {
		log.Printf("decode create user request: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	dbUser, err := cfg.db.GetUser(r.Context(), userRequest.Email)
	if err != nil {
		log.Printf("decode create user request: %v", err)
		respondWithError(w, http.StatusUnauthorized, "Incorrect email")
		return
	}

	match, err := auth.CheckPasswordHash(userRequest.Password, dbUser.HashedPassword)
	if err != nil || !match {
		log.Printf("decode create user request: %v", err)
		respondWithError(w, http.StatusUnauthorized, "Incorrect password")
		return
	}

	token, err := auth.MakeJWT(dbUser.ID, cfg.jwtSecret, 1*time.Hour)
	if err != nil {
		respondWithJSON(w, http.StatusUnauthorized, err.Error())
		return
	}

	refreshToken := auth.MakeRefreshToken()

	dbRefreshToken, err := cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    dbUser.ID,
		ExpiresAt: time.Now().Add(60 * 24 * time.Hour),
		RevokedAt: sql.NullTime{},
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	userResponse := User{
		ID:           dbUser.ID,
		CreatedAt:    dbUser.CreatedAt,
		UpdatedAt:    dbUser.UpdatedAt,
		Email:        dbUser.Email,
		Token:        token,
		RefreshToken: dbRefreshToken.Token,
	}
	respondWithJSON(w, http.StatusOK, userResponse)
}

func (cfg *apiConfig) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	dbRefreshToken, err := cfg.db.GetRefreshToken(r.Context(), token)
	if err != nil || dbRefreshToken.ExpiresAt.Before(time.Now()) || dbRefreshToken.RevokedAt.Valid {
		respondWithError(w, http.StatusUnauthorized, "Token is not found or expired")
		return
	}

	newToken, err := auth.MakeJWT(dbRefreshToken.UserID, cfg.jwtSecret, 1*time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, struct {
		Token string `json:"token"`
	}{Token: newToken})
}

func (cfg *apiConfig) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	arg := database.UpdateRevokeRefreshTokenParams{
		RevokedAt: sql.NullTime{Time: time.Now(), Valid: true},
		UpdatedAt: time.Now(),
		Token:     token,
	}

	err = cfg.db.UpdateRevokeRefreshToken(r.Context(), arg)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) createUpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	tokenUserID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	type UserBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var decodedBody UserBody
	err = json.NewDecoder(r.Body).Decode(&decodedBody)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	newPassword, err := auth.HashPassword(decodedBody.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updatedUser := database.UpdateUserParams{
		Email:          decodedBody.Email,
		HashedPassword: newPassword,
		ID:             tokenUserID,
	}
	err = cfg.db.UpdateUser(r.Context(), updatedUser)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	userDb, err := cfg.db.GetUser(r.Context(), updatedUser.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	userResponse := User{
		ID:        userDb.ID,
		CreatedAt: userDb.CreatedAt,
		UpdatedAt: userDb.UpdatedAt,
		Email:     userDb.Email,
	}

	respondWithJSON(w, http.StatusOK, userResponse)
}

func (cfg *apiConfig) deleteChirpsHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	tokenUserID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	id := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(id)
	if err != nil {
		log.Print(err)
		respondWithError(w, http.StatusBadRequest, "Cannot parse the ID")
		return
	}

	chirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithJSON(w, http.StatusNotFound, "No chirp found with that ID")
			return
		}
	}

	if tokenUserID != chirp.UserID {
		respondWithError(w, http.StatusForbidden, "User does not own this chirp")
		return
	}

	err = cfg.db.DeleteChirp(r.Context(), database.DeleteChirpParams{ID: chirp.ID, UserID: tokenUserID})
	if err != nil {
		respondWithError(w, http.StatusForbidden, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
