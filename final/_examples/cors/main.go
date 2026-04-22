package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()

	// CORS middleware with full configuration.
	r.Use(middleware.CORS(middleware.CORSConfig{
		// Whitelist specific origins
		AllowedOrigins: []string{"https://example.com", "https://app.example.com"},

		// Dynamic origin function — allows any *.example.com subdomain
		// (takes precedence over AllowedOrigins when set)
		AllowOriginFunc: func(origin string, r *http.Request) bool {
			return strings.HasSuffix(origin, ".example.com")
		},

		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{"X-Request-Id", "X-Total-Count"},
		AllowCredentials: true,
		MaxAge:           3600,
		HandlePreflight:  true,
	}))

	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "abc-123")
		w.Header().Set("X-Total-Count", "42")
		w.Write([]byte("hello from CORS-enabled server"))
	})

	r.Post("/data", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	fmt.Println("CORS example server listening on :3333")
	fmt.Println()
	fmt.Println("Test with curl:")
	fmt.Println()
	fmt.Println("  # Allowed origin (preflight):")
	fmt.Println(`  curl -v -X OPTIONS http://localhost:3333/ -H "Origin: https://app.example.com" -H "Access-Control-Request-Method: POST"`)
	fmt.Println()
	fmt.Println("  # Allowed origin (actual request):")
	fmt.Println(`  curl -v http://localhost:3333/ -H "Origin: https://app.example.com"`)
	fmt.Println()
	fmt.Println("  # Disallowed origin:")
	fmt.Println(`  curl -v http://localhost:3333/ -H "Origin: https://evil.com"`)
	fmt.Println()
	fmt.Println("  # No Origin header:")
	fmt.Println(`  curl -v http://localhost:3333/`)

	http.ListenAndServe(":3333", r)
}
