package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/moawedmohamed/chirpy-app.git/handler"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/moawedmohamed/chirpy-app.git/internal/database"
)

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

type paramters struct {
	Body string `json:"body"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	mux := http.NewServeMux()
	cfg := &apiConfig{
		db: dbQueries,
	}
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
		err := cfg.db.DeleteALLUsers(r.Context())
		if err != nil {
			handler.RespondWithError(w, http.StatusInternalServerError, "Couldn't delete users")
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		words := []string{"kerfuffle", "sharbert", "fornax"}

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
	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		type emailData struct {
			Email string `json:"email"`
		}
		parasedBody := json.NewDecoder(r.Body)
		params := emailData{}
		err := parasedBody.Decode(&params)
		if err != nil {
			handler.RespondWithError(w, http.StatusBadRequest, "something went wrong in the body paring ")
			return
		}

		user, err := cfg.db.CreateUser(r.Context(), params.Email)
		if err != nil {
			handler.RespondWithError(w, http.StatusInternalServerError, "something went wrong")
			return
		}
		res := UserResponse{
			ID:        uuid.UUID(user.ID),
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		}
		handler.RespondWithJSON(w, http.StatusCreated, res)
	})
	log.Fatal(http.ListenAndServe(":8080", mux))
}
