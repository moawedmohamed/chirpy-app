package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func main() {
	mux := http.NewServeMux()
	cfg := &apiConfig{}
	mux.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	mux.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("assets"))))

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html;charset=utf-8")
		w.WriteHeader(http.StatusOK)
		hits := cfg.fileserverHits.Load()
		message := fmt.Sprintf(`
			<html>
	  <body>
	    <h1>Welcome, Chirpy Admin</h1>
	    <p>Chirpy has been visited %d times!</p>
	  </body>
	</html>
			`, hits)
		w.Write([]byte(message))
	})
	mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		cfg.fileserverHits.Store(0)
		hits := cfg.fileserverHits.Load()
		message := fmt.Sprintf("Hits: %d", hits)
		w.Write([]byte(message))
	})

	mux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		words := []string{"kerfuffle", "sharbert", "fornax"}
		type paramters struct {
			Body string `json:"body"`
		}

		type errorResponse struct {
			Error string `json:"error"`
		}

		type cleanedBody struct {
			CleanedBody string `json:"cleaned_body"`
		}

		parasedBody := json.NewDecoder(r.Body)
		params := paramters{}
		err := parasedBody.Decode(&params)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errorResponse{
				Error: "Something went wrong",
			})
			return
		}
		if len(params.Body) > 140 {

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errorResponse{
				Error: "Chirp is too long",
			})
			return
		}
		wordsInBody := strings.Fields(params.Body)
		for i, v := range wordsInBody {
			for _, c := range words {
				if strings.EqualFold(strings.ToLower(v), strings.ToLower(c)) {
					wordsInBody[i] = "****"
				}
			}
		}
		cleaned := strings.Join(wordsInBody, " ")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(cleanedBody{
			CleanedBody: cleaned,
		})
	})

	log.Fatal(http.ListenAndServe(":8080", mux))
}
