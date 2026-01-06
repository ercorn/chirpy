package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ercorn/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	db             *database.Queries
	platform       string
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
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 Forbidden"))
		return
	}

	cfg.fileServerHits.Store(0)
	err := cfg.db.DeleteUsers(context.Background())
	if err != nil {
		log.Printf("Failed to delete users: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to delete users"))
		return
	}

	w.WriteHeader(200)
	w.Write([]byte("Metrics Reset"))
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	decoder := json.NewDecoder(req.Body)
	req_body := struct {
		Email string
	}{}
	err := decoder.Decode(&req_body)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong decoding the request", err)
		return
	}

	user, err := cfg.db.CreateUser(req.Context(), req_body.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create user", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, struct {
		Id        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}{
		Id:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})

}

func (cfg *apiConfig) chirpsHandler(w http.ResponseWriter, req *http.Request) {
	type request_body struct {
		Body   string    `json:"body"`
		UserId uuid.UUID `json:"user_id"`
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

	//replace profane words in body
	resp_str_arr := strings.Split(req_body.Body, " ")
	for i, word := range resp_str_arr {
		lowered_word := strings.ToLower(word)
		if lowered_word == "kerfuffle" || lowered_word == "sharbert" || lowered_word == "fornax" {
			resp_str_arr[i] = "****"
		}
	}
	cleaned_body := strings.Join(resp_str_arr, " ")

	// respondWithJSON(w, http.StatusOK, struct {
	// 	CleanedBody string `json:"cleaned_body"`
	// }{
	// 	CleanedBody: strings.Join(resp_str_arr, " "),
	// })

	chirp, err := cfg.db.CreateChirp(req.Context(), database.CreateChirpParams{
		Body:   cleaned_body,
		UserID: req_body.UserId,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create chirp", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, struct {
		Id        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserId    uuid.UUID `json:"user_id"`
	}{
		Id:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserId:    chirp.UserID,
	})
}

func main() {
	godotenv.Load()
	db_url := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", db_url)
	if err != nil {
		log.Fatalf("Failed to open a connection to the database: %v", err)
	}

	dbQueries := database.New(db)

	apiCfg := apiConfig{
		db:       dbQueries,
		platform: os.Getenv("PLATFORM"),
	}
	serve_mux := http.NewServeMux()
	serve_mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))
	serve_mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	serve_mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	serve_mux.HandleFunc("POST /admin/reset", apiCfg.metricsResetHandler)
	serve_mux.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	serve_mux.HandleFunc("POST /api/chirps", apiCfg.chirpsHandler)

	server := http.Server{
		Handler: serve_mux,
		Addr:    ":8080",
	}
	server.ListenAndServe()
}
