package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/kaylubr/chirpy/internal/auth"
	"github.com/kaylubr/chirpy/internal/database"
)

func (cfg *apiConfig) chirpsListHandler(w http.ResponseWriter, r *http.Request) {
	authorID := r.URL.Query().Get("author_id")
	sortMethod := r.URL.Query().Get("sort")

	var dbChirps []database.Chirp
	var err error

	if authorID != "" {
		dbChirps, err = cfg.db.GetAllUserChirps(r.Context(), uuid.MustParse(authorID))
		if err != nil {
			log.Print(err)
			respondWithError(w, http.StatusInternalServerError, "Cannot get chirps for that user")
			return
		}
	} else {
		dbChirps, err = cfg.db.GetAllChirps(r.Context())
		if err != nil {
			log.Print(err)
			respondWithError(w, http.StatusInternalServerError, "Cannot get chirps")
			return
		}
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

	switch sortMethod {
	case "asc":
		sort.Slice(chirpList, func(i, j int) bool { return chirpList[i].CreatedAt.Before(chirpList[j].CreatedAt) })
	case "desc":
		sort.Slice(chirpList, func(i, j int) bool { return chirpList[i].CreatedAt.After(chirpList[j].CreatedAt) })
	}

	respondWithJSON(w, http.StatusOK, chirpList)
}

func (cfg *apiConfig) fetchChirpHandler(w http.ResponseWriter, r *http.Request) {
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
