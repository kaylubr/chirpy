package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	w.Header().Set("Content-Type", "application/json")
	errResponse := errorResponse{Error: msg}
	byteResponse, _ := json.Marshal(errResponse)
	w.WriteHeader(code)
	w.Write(byteResponse)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	bytesResponse, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Cannot encode the payload")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(bytesResponse)
}

func cleanSentence(sentence string) string {
	words := strings.Fields(sentence)
	cleanedWords := make([]string, 0)
	for _, word := range words {
		if strings.ToLower(word) == "kerfuffle" || strings.ToLower(word) == "sharbert" || strings.ToLower(word) == "fornax" {
			cleanedWords = append(cleanedWords, "****")
			continue
		}
		cleanedWords = append(cleanedWords, word)
	}
	return strings.Join(cleanedWords, " ")
}

func validateChirp(w http.ResponseWriter, chirp string) (string, error) {
	if len(chirp) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return "", errors.New("Invalid chirp")
	}
	return cleanSentence(chirp), nil
}
