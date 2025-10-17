package main

import "net/http"

func main() {
	serve_mux := http.NewServeMux()
	serve_mux.Handle("/", http.FileServer(http.Dir(".")))

	server := http.Server{
		Handler: serve_mux,
		Addr:    ":8080",
	}
	server.ListenAndServe()
}
