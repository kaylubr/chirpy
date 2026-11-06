package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/kaylubr/chirpy/internal/auth"
	"github.com/kaylubr/chirpy/internal/database"
)

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
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
		IsChirpyRed:  dbUser.IsChirpyRed,
	}
	respondWithJSON(w, http.StatusOK, userResponse)
}
