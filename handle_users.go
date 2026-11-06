package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/kaylubr/chirpy/internal/auth"
	"github.com/kaylubr/chirpy/internal/database"
)

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
		ID:          uuid.UUID(user.ID),
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}

	respondWithJSON(w, http.StatusCreated, userResponse)
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
		ID:          userDb.ID,
		CreatedAt:   userDb.CreatedAt,
		UpdatedAt:   userDb.UpdatedAt,
		Email:       userDb.Email,
		IsChirpyRed: userDb.IsChirpyRed,
	}

	respondWithJSON(w, http.StatusOK, userResponse)
}
