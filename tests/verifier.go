package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	passed := 0
	failed := 0

	// P1: Verify that in the init state, CORS middleware does not exist
	// (checked by file existence in run-tests.sh)

	// F1: AllowedOrigins whitelist - matching origin gets Access-Control-Allow-Origin
	if ok := testAllowedOriginsWhitelist(); ok {
		fmt.Println("PASS F1: AllowedOrigins whitelist sets correct Access-Control-Allow-Origin header")
		passed++
	} else {
		fmt.Println("FAIL F1: AllowedOrigins whitelist did not set correct header")
		failed++
	}

	// F2: AllowOriginFunc dynamic origin policy
	if ok := testAllowOriginFunc(); ok {
		fmt.Println("PASS F2: AllowOriginFunc dynamic origin policy works correctly")
		passed++
	} else {
		fmt.Println("FAIL F2: AllowOriginFunc dynamic origin policy failed")
		failed++
	}

	// F3: Preflight handling with HandlePreflight=true
	if ok := testPreflightHandling(); ok {
		fmt.Println("PASS F3: Preflight requests return correct headers and 204 status")
		passed++
	} else {
		fmt.Println("FAIL F3: Preflight handling incorrect")
		failed++
	}

	// F4: AllowCredentials sets correct header
	if ok := testAllowCredentials(); ok {
		fmt.Println("PASS F4: AllowCredentials sets Access-Control-Allow-Credentials: true")
		passed++
	} else {
		fmt.Println("FAIL F4: AllowCredentials header missing or incorrect")
		failed++
	}

	// F5: ExposedHeaders and MaxAge
	if ok := testExposedHeadersAndMaxAge(); ok {
		fmt.Println("PASS F5: ExposedHeaders and MaxAge configuration work correctly")
		passed++
	} else {
		fmt.Println("FAIL F5: ExposedHeaders or MaxAge not working")
		failed++
	}

	// F6: Disallowed origin gets no CORS headers
	if ok := testDisallowedOrigin(); ok {
		fmt.Println("PASS F6: Disallowed origin receives no Access-Control-Allow-Origin header")
		passed++
	} else {
		fmt.Println("FAIL F6: Disallowed origin incorrectly received CORS headers")
		failed++
	}

	fmt.Printf("\nResults: %d passed, %d failed\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func makeRequest(cfg middleware.CORSConfig, method string, origin string, isPreflight bool) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Use(middleware.CORS(cfg))
	r.Options("/", func(w http.ResponseWriter, r *http.Request) {})
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest(method, "/", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if isPreflight {
		req.Header.Set("Access-Control-Request-Method", "GET")
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func testAllowedOriginsWhitelist() bool {
	cfg := middleware.CORSConfig{
		AllowedOrigins:  []string{"https://example.com"},
		AllowedMethods:  []string{"GET", "POST"},
		HandlePreflight: true,
	}

	// Allowed origin
	rec := makeRequest(cfg, "GET", "https://example.com", false)
	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		return false
	}

	// Non-allowed origin
	rec2 := makeRequest(cfg, "GET", "https://evil.com", false)
	origin2 := rec2.Header().Get("Access-Control-Allow-Origin")
	if origin2 != "" {
		return false
	}

	return true
}

func testAllowOriginFunc() bool {
	cfg := middleware.CORSConfig{
		AllowOriginFunc: func(origin string, r *http.Request) bool {
			return strings.HasSuffix(origin, ".example.com")
		},
		AllowedMethods:  []string{"GET"},
		HandlePreflight: true,
	}

	// Matching suffix
	rec := makeRequest(cfg, "GET", "https://app.example.com", false)
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		return false
	}

	// Non-matching suffix
	rec2 := makeRequest(cfg, "GET", "https://app.other.com", false)
	if rec2.Header().Get("Access-Control-Allow-Origin") != "" {
		return false
	}

	return true
}

func testPreflightHandling() bool {
	cfg := middleware.CORSConfig{
		AllowedOrigins:  []string{"https://example.com"},
		AllowedMethods:  []string{"GET", "POST"},
		AllowedHeaders:  []string{"Content-Type"},
		HandlePreflight: true,
	}

	rec := makeRequest(cfg, "OPTIONS", "https://example.com", true)

	if rec.Code != http.StatusNoContent {
		return false
	}
	if rec.Header().Get("Access-Control-Allow-Methods") != "GET, POST" {
		return false
	}
	if rec.Header().Get("Access-Control-Allow-Headers") != "Content-Type" {
		return false
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		return false
	}
	return true
}

func testAllowCredentials() bool {
	cfg := middleware.CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowCredentials: true,
	}

	rec := makeRequest(cfg, "GET", "https://example.com", false)
	return rec.Header().Get("Access-Control-Allow-Credentials") == "true"
}

func testExposedHeadersAndMaxAge() bool {
	cfg := middleware.CORSConfig{
		AllowedOrigins:  []string{"https://example.com"},
		AllowedMethods:  []string{"GET"},
		ExposedHeaders:  []string{"X-Custom", "X-Request-Id"},
		MaxAge:          3600,
		HandlePreflight: true,
	}

	// ExposedHeaders on normal request
	rec := makeRequest(cfg, "GET", "https://example.com", false)
	if rec.Header().Get("Access-Control-Expose-Headers") != "X-Custom, X-Request-Id" {
		return false
	}

	// MaxAge on preflight
	rec2 := makeRequest(cfg, "OPTIONS", "https://example.com", true)
	if rec2.Header().Get("Access-Control-Max-Age") != "3600" {
		return false
	}

	return true
}

func testDisallowedOrigin() bool {
	cfg := middleware.CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowCredentials: true,
		ExposedHeaders:   []string{"X-Custom"},
	}

	rec := makeRequest(cfg, "GET", "https://evil.com", false)
	return rec.Header().Get("Access-Control-Allow-Origin") == ""
}
