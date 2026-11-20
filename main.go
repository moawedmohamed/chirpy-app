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
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Token        string    `json:"token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
}
type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	jwtSecret      string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
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
		db:        dbQueries,
		jwtSecret: os.Getenv("TOKEN_SECRET"),
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
			Body string `json:"body"`
		}

		words := []string{"kerfuffle", "sharbert", "fornax"}

		parasedBody := json.NewDecoder(r.Body)
		params := BodyData{}
		err := parasedBody.Decode(&params)
		if err != nil {
			handler.RespondWithError(w, http.StatusBadRequest, "something went wrong in the body paring ")
			return
		}
		if len(params.Body) > 140 {
			handler.RespondWithError(w, http.StatusBadRequest, "chirp body is too long")
			return
		}
		tokenString, err := auth.GetBearerToken(r.Header)
		if err != nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "token is invalid")
			return
		}
		log.Println("GetBearerToken: SUCCESS")

		userID, err := auth.ValidateJWT(tokenString, cfg.jwtSecret)
		log.Println(err)
		if err != nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "token is invalid")
			return
		}
		if userID == uuid.Nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "token is invalid")
			return
		}
		log.Println("ValidateJWT: SUCCESS")

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
			UserID: userID,
		})
		if err != nil {
			log.Printf("Couldn't create chirp: %v", err)
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
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", func(w http.ResponseWriter, r *http.Request) {
		chirpID := r.PathValue("chirpID")
		id, err := uuid.Parse(chirpID)
		if err != nil {
			log.Fatal(err)
			handler.RespondWithError(w, http.StatusBadRequest, "invalid chirp id")
			return
		}
		tokenString, err := auth.GetBearerToken(r.Header)
		if err != nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "token is invalid")
			return
		}
		userID, err := auth.ValidateJWT(tokenString, cfg.jwtSecret)
		if err != nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "token is invalid")
			return
		}
		if userID == uuid.Nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "token is invalid")
			return
		}
		chirp, err := cfg.db.GetChirpByID(r.Context(), id)
		if err != nil {
			handler.RespondWithError(w, http.StatusNotFound, "Couldn't get chirp")
			return
		}
		if chirp.UserID != userID {
			handler.RespondWithError(w, http.StatusForbidden, "You are not authorized to delete this chirp")
			return
		}
		err = cfg.db.DeleteChirpByID(r.Context(), id)
		if err != nil {
			handler.RespondWithError(w, http.StatusInternalServerError, "Couldn't delete chirp")
			return
		}

		handler.RespondWithJSON(w, http.StatusNoContent, nil)
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
		refreshToken := auth.MakeRefreshToken()
		cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
			UserID:    user.ID,
			Token:     refreshToken,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		})

		expiresIn := time.Hour
		token, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Duration(expiresIn))
		if err != nil {
			handler.RespondWithError(w, http.StatusInternalServerError, "Failed to generate JWT")
		}
		handler.RespondWithJSON(
			w,
			http.StatusOK,
			UserResponse{
				ID:           user.ID,
				CreatedAt:    user.CreatedAt,
				UpdatedAt:    user.UpdatedAt,
				Email:        user.Email,
				Token:        token,
				RefreshToken: refreshToken,
			},
		)
	})

	mux.HandleFunc("PUT /api/users", func(w http.ResponseWriter, r *http.Request) {
		type updateData struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		parasedBody := json.NewDecoder(r.Body)
		params := updateData{}
		err := parasedBody.Decode(&params)
		if err != nil {
			handler.RespondWithError(w, http.StatusBadRequest, "something went wrong in the body paring ")
			return
		}
		tokenString, err := auth.GetBearerToken(r.Header)
		if err != nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "token is invalid")
			return
		}
		userID, err := auth.ValidateJWT(tokenString, cfg.jwtSecret)
		if err != nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "token is invalid")
			return
		}
		if userID == uuid.Nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "token is invalid")
			return
		}
		hasedPassowrd, err := auth.HashPassword(params.Password)
		if err != nil {
			handler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		user, err := cfg.db.UpdateUser(r.Context(), database.UpdateUserParams{
			ID:             userID,
			Email:          params.Email,
			HashedPassword: hasedPassowrd,
		})
		if err != nil {
			handler.RespondWithError(w, http.StatusInternalServerError, "something went wrong")
			return
		}
		res := UserResponse{
			ID:    uuid.UUID(user.ID),
			Email: user.Email,
		}
		handler.RespondWithJSON(w, http.StatusOK, res)
	})
	mux.HandleFunc("POST /api/refresh", func(w http.ResponseWriter, r *http.Request) {
		type res struct {
			Token string `json:"token"`
		}

		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "refresh token is invalid")
			return
		}
		if token == "" {
			handler.RespondWithError(w, http.StatusUnauthorized, "refresh token is missing")
			return
		}
		refreshToken, err := cfg.db.GetRefreshToken(r.Context(), token)
		if err != nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "refresh token is invalid")
			return
		}
		if refreshToken.ExpiresAt.Before(time.Now()) {
			handler.RespondWithError(w, http.StatusUnauthorized, "refresh token is expired")
			return
		}
		if refreshToken.RevokedAt.Valid {
			handler.RespondWithError(w, http.StatusUnauthorized, "refresh token is revoked")
			return
		}
		newToken, err := auth.MakeJWT(refreshToken.UserID, cfg.jwtSecret, time.Hour)
		if err != nil {
			handler.RespondWithError(w, http.StatusInternalServerError, "failed to generate new token")
			return
		}
		handler.RespondWithJSON(w, http.StatusOK, res{Token: newToken})
	})

	mux.HandleFunc("POST /api/revoke", func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "refresh token is invalid")
			return
		}
		if token == "" {
			handler.RespondWithError(w, http.StatusUnauthorized, "refresh token is missing")
			return
		}
		refreshToken, err := cfg.db.GetRefreshToken(r.Context(), token)
		if err != nil {
			handler.RespondWithError(w, http.StatusUnauthorized, "refresh token is invalid")
			return
		}
		if refreshToken.ExpiresAt.Before(time.Now()) {
			handler.RespondWithError(w, http.StatusUnauthorized, "refresh token is expired")
		}
		if refreshToken.RevokedAt.Valid {
			handler.RespondWithError(w, http.StatusUnauthorized, "refresh token is revoked")
		}
		errRevoke := cfg.db.RevokeRefreshToken(r.Context(), token)
		if errRevoke != nil {
			handler.RespondWithError(w, http.StatusInternalServerError, "failed to revoke refresh token")
			return
		}
		handler.RespondWithJSON(w, http.StatusNoContent, nil)
	})

	log.Fatal(http.ListenAndServe(":8080", mux))
}
