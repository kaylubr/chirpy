package main

import (
	"fmt"
	"net/http"
	"os"
)

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
