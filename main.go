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

	"github.com/google/uuid"

	"github.com/moawedmohamed/chirpy-app.git/handler"

	"github.com/moawedmohamed/chirpy-app.git/internal/auth"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/moawedmohamed/chirpy-app.git/internal/database"
)

type cleanedBody struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

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

//	type paramters struct {
//		Body string `json:"body"`
//	}
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
		platform := os.Getenv("PLATFORM")
		if platform == "dev" {
			err := cfg.db.DeleteALLUsers(r.Context())
			if err != nil {
				handler.RespondWithError(w, http.StatusInternalServerError, "Couldn't delete users")
				return
			}
			w.WriteHeader(http.StatusOK)

		} else {
			w.WriteHeader(http.StatusForbidden)
			log.Fatal("you cant delete user outside of dev env")
			return
		}
	})
	mux.HandleFunc("/api/chirps", func(w http.ResponseWriter, r *http.Request) {
		chirps, err := cfg.db.GetAllChirps(r.Context())
		if err != nil {
			log.Fatal(err)
			handler.RespondWithError(w, http.StatusInternalServerError, "Couldn't fetch the chirps")
			return
		}
		res := []cleanedBody{}
		for _, chirp := range chirps {
			res = append(res, cleanedBody{
				ID:        chirp.ID,
				CreatedAt: chirp.CreatedAt,
				UpdatedAt: chirp.UpdatedAt,
				Body:      chirp.Body,
				UserID:    chirp.UserID,
			})
		}
		handler.RespondWithJSON(w, http.StatusOK, res)
	})
	mux.HandleFunc("GET /api/chirps/{chirpID}", func(w http.ResponseWriter, r *http.Request) {
		chirpID := r.PathValue("chirpID")
		id, err := uuid.Parse(chirpID)
		if err != nil {
			log.Fatal(err)
			handler.RespondWithError(w, http.StatusBadRequest, "invalid chirp id")
			return
		}

		chirp, err := cfg.db.GetChirpByID(r.Context(), id)
		if err != nil {
			// log.Fatal(err)
			handler.RespondWithError(w, http.StatusNotFound, "Couldn't get chirp")
			return
		}
		handler.RespondWithJSON(w, http.StatusOK, cleanedBody{
			ID:        chirp.ID,
			Body:      chirp.Body,
			UpdatedAt: chirp.UpdatedAt,
			CreatedAt: chirp.CreatedAt,
			UserID:    chirp.UserID,
		})
	})
	mux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		type BodyData struct {
			Body   string    `json:"body"`
			UserID uuid.UUID `json:"user_id"`
		}

		words := []string{"kerfuffle", "sharbert", "fornax"}

		parasedBody := json.NewDecoder(r.Body)
		params := BodyData{}
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
		chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
			Body:   cleaned,
			UserID: params.UserID,
		})
		if err != nil {
			log.Fatal(err)
			handler.RespondWithError(w, http.StatusInternalServerError, "Couldn't create a chirp")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(cleanedBody{
			ID:        chirp.ID,
			Body:      chirp.Body,
			UpdatedAt: chirp.UpdatedAt,
			CreatedAt: chirp.CreatedAt,
			UserID:    chirp.UserID,
		})
	})
	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		type userData struct {
			Passowrd string `json:"password"`
			Email    string `json:"email"`
		}
		parasedBody := json.NewDecoder(r.Body)
		params := userData{}
		err := parasedBody.Decode(&params)
		if err != nil {
			handler.RespondWithError(w, http.StatusBadRequest, "something went wrong in the body paring ")
			return
		}
		hasedPassowrd, err := auth.HashPassword(params.Passowrd)
		if err != nil {
			handler.RespondWithError(w, http.StatusBadRequest, err.Error())
		}

		user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
			Email:          params.Email,
			HashedPassword: hasedPassowrd,
		})
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
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		type loginData struct {
			Passowrd string `json:"password"`
			Email    string `json:"email"`
		}
		parasedBody := json.NewDecoder(r.Body)
		params := loginData{}
		err := parasedBody.Decode(&params)
		if err != nil {
			handler.RespondWithError(w, http.StatusInternalServerError, "something went weong")
			log.Fatal(err)
			return
		}
		user, err := cfg.db.GetUserByEmail(r.Context(), params.Email)
		if err != nil {
			handler.RespondWithError(w, http.StatusInternalServerError, "something went weong")
			log.Fatal(err)
			return
		}
		isValid, err := auth.CheckPasswordHash(params.Passowrd, user.HashedPassword)
		if err != nil {
			handler.RespondWithError(
				w,
				http.StatusInternalServerError,
				"Something went wrong",
			)
			return
		}
		if !isValid {
			handler.RespondWithError(
				w,
				http.StatusUnauthorized,
				"Incorrect email or password",
			)
			return
		}
		handler.RespondWithJSON(
			w,
			http.StatusOK,
			UserResponse{
				ID:        user.ID,
				CreatedAt: user.CreatedAt,
				UpdatedAt: user.UpdatedAt,
				Email:     user.Email,
			},
		)
	})
	log.Fatal(http.ListenAndServe(":8080", mux))
}
