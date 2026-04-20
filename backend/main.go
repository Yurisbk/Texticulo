package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"texticulo/backend/db"
	"texticulo/backend/handlers"
	"texticulo/backend/middleware"
)

func main() {
	database, err := db.Open()
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	if err := db.Migrate(database); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	auth := &handlers.AuthHandler{DB: database}
	links := &handlers.LinksHandler{DB: database}
	metricsHandler := &handlers.MetricsHandler{DB: database}

	frontendURL := "http://localhost:5173"
	if f := os.Getenv("FRONTEND_URL"); f != "" {
		parts := strings.Split(f, ",")
		if len(parts) > 0 {
			frontendURL = strings.TrimSpace(parts[0])
		}
	}
	oauth := &handlers.OAuthHandler{
		DB:          database,
		Google:      handlers.NewGoogleOAuthConfig(),
		FrontendURL: frontendURL,
	}

	allowedOrigins := []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	if f := os.Getenv("FRONTEND_URL"); f != "" {
		for _, p := range strings.Split(f, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				allowedOrigins = append(allowedOrigins, p)
			}
		}
	}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", handlers.Health)

	r.Route("/api", func(r chi.Router) {
		r.Get("/metrics", metricsHandler.Metrics)
		r.Get("/auth/google", oauth.GoogleStart)
		r.Get("/auth/google/callback", oauth.GoogleCallback)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthRequired)
			r.Get("/auth/me", auth.Me)
			r.Get("/links", links.List)
			r.Get("/links/{code}/stats", links.Stats)
			r.Delete("/links/{code}", links.Delete)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthOptional)
			r.With(middleware.ShortenRateLimit(30, time.Minute)).Post("/shorten", links.Shorten)
		})
	})

	r.Get("/{code}", links.Redirect)

	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
