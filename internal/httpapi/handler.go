package httpapi

import (
	"encoding/json"
	"net/http"
)

// ProvideMux returns the default HTTP handler for the service.
func ProvideMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	return mux
}
