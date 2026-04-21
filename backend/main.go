package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"

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

	// Global middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	// Limit all request bodies to 64 KB (C5/M10 — DoS prevention)
	r.Use(middleware.MaxBodySize(64 * 1024))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// SEO
	publicURL := strings.TrimRight(os.Getenv("PUBLIC_API_URL"), "/")
	if publicURL == "" {
		publicURL = "http://localhost:8080"
	}
	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", publicURL)
	})
	r.Get("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>%s</loc>
    <changefreq>weekly</changefreq>
    <priority>1.0</priority>
  </url>
</urlset>`, strings.TrimRight(os.Getenv("FRONTEND_URL"), ","))
	})

	r.Get("/health", handlers.Health)

	r.Route("/api", func(r chi.Router) {
		// All /api routes require application/json on mutations (A4 — CSRF via text/plain)
		r.Use(middleware.RequireJSON)

		// Metrics — requires auth to prevent leaking aggregated data (C1/C7)
		r.With(middleware.AuthRequired).Get("/metrics", metricsHandler.Metrics)

		// Google OAuth (GET — exempt from RequireJSON naturally)
		r.Get("/auth/google", oauth.GoogleStart)
		r.Get("/auth/google/callback", oauth.GoogleCallback)

		// Email/password auth — rate-limited per IP (A8)
		registerLimiter := middleware.NewIPRateLimiter(10, time.Minute)
		loginLimiter := middleware.NewIPRateLimiter(20, time.Minute)
		r.With(registerLimiter).Post("/auth/register", auth.Register)
		r.With(loginLimiter).Post("/auth/login", auth.Login)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthRequired)
			r.Get("/auth/me", auth.Me)
			r.Get("/links", links.List)
			r.Get("/links/{code}/stats", links.Stats)
			r.Delete("/links/{code}", links.Delete)
		})

		// Shorten — auth optional, rate-limited to 15 req/min per IP
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthOptional)
			r.With(middleware.ShortenRateLimit(15, time.Minute)).Post("/shorten", links.Shorten)
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
