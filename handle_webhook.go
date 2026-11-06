package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/kaylubr/chirpy/internal/auth"
	"github.com/kaylubr/chirpy/internal/database"
)

func (cfg *apiConfig) polkaWebHookHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, err := auth.GetAPIKey(r.Header)
	if err != nil || apiKey != cfg.polkaKey {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	type EventBody struct {
		Event string `json:"event"`
		Data  struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}

	var event EventBody
	err = json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if event.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	fetchedUser, err := cfg.db.GetUserById(r.Context(), uuid.MustParse(event.Data.UserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, err.Error())
			return
		}
		return
	}

	args := database.UpdateUserIsChirpyRedParams{
		IsChirpyRed: !fetchedUser.IsChirpyRed,
		ID:          fetchedUser.ID,
	}

	err = cfg.db.UpdateUserIsChirpyRed(r.Context(), args)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
