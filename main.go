package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileServerHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	response := fmt.Sprintf(
		`<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
		</html>`, cfg.fileServerHits.Load())
	w.Write([]byte(response))
}

func (cfg *apiConfig) metricsResetHandler(w http.ResponseWriter, req *http.Request) {
	cfg.fileServerHits.Store(0)

	w.WriteHeader(200)
	w.Write([]byte("Metrics Reset"))
}

func (cfg *apiConfig) chirpValidationHandler(w http.ResponseWriter, req *http.Request) {
	type request_body struct {
		Body string `json:"body"`
	}

	type response_body struct {
		Valid bool `json:"valid"`
	}

	type error_body struct {
		Error string `json:"error"`
	}

	w.Header().Set("Content-Type", "application/json")

	decoder := json.NewDecoder(req.Body)
	req_body := request_body{}
	err := decoder.Decode(&req_body)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong decoding the request, check server logs.", err)
		return
	}

	if len(req_body.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long", nil)
		return
	}

	resp_body := response_body{
		Valid: true,
	}

	dat, err := json.Marshal(resp_body)
	if err != nil {
		log.Printf("Error marshalling valid response JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(200)
	w.Write(dat)
}

func main() {
	apiCfg := apiConfig{}
	serve_mux := http.NewServeMux()
	serve_mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))
	serve_mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	serve_mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	serve_mux.HandleFunc("POST /admin/reset", apiCfg.metricsResetHandler)
	serve_mux.HandleFunc("POST /api/validate_chirp", apiCfg.chirpValidationHandler)

	server := http.Server{
		Handler: serve_mux,
		Addr:    ":8080",
	}
	server.ListenAndServe()
}
