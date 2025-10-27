package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileServerHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	cfg.fileServerHits.Add(1)
	return next
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	response := fmt.Sprint("Hits: ", cfg.fileServerHits.Load())
	w.Write([]byte(response))
}

func main() {
	apiCfg := apiConfig{}
	serve_mux := http.NewServeMux()
	serve_mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))
	serve_mux.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	serve_mux.HandleFunc("/metrics", apiCfg.metricsHandler)

	server := http.Server{
		Handler: serve_mux,
		Addr:    ":8080",
	}
	server.ListenAndServe()
}
