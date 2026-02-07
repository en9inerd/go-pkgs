package middleware

import (
	"encoding/json"
	"net/http"
)

type HealthResponse struct {
	Status string `json:"status"`
}

func Health(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
