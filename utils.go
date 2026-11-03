package main

import (
	"net/http"
	"encoding/json"
)

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	w.Header().Set("Content-Type", "application/json")
	errResponse := errorResponse{Error: msg}
	byteResponse, _:= json.Marshal(errResponse)
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
