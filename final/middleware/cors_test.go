package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCORS_AllowedOrigins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		config         CORSConfig
		origin         string
		wantOrigin     string
		wantStatus     int
		wantVaryOrigin bool
	}{
		{
			name: "origin in whitelist",
			config: CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
			},
			origin:         "https://example.com",
			wantOrigin:     "https://example.com",
			wantStatus:     http.StatusOK,
			wantVaryOrigin: true,
		},
		{
			name: "origin not in whitelist",
			config: CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
			},
			origin:     "https://evil.com",
			wantOrigin: "",
			wantStatus: http.StatusOK,
		},
		{
			name: "wildcard origin without credentials",
			config: CORSConfig{
				AllowedOrigins: []string{"*"},
			},
			origin:         "https://any.com",
			wantOrigin:     "*",
			wantStatus:     http.StatusOK,
			wantVaryOrigin: false,
		},
		{
			name: "wildcard origin with credentials uses specific origin",
			config: CORSConfig{
				AllowedOrigins:   []string{"*"},
				AllowCredentials: true,
			},
			origin:         "https://any.com",
			wantOrigin:     "https://any.com",
			wantStatus:     http.StatusOK,
			wantVaryOrigin: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := chi.NewRouter()
			r.Use(CORS(tt.config))
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			got := rec.Header().Get("Access-Control-Allow-Origin")
			if got != tt.wantOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantOrigin)
			}

			if tt.wantVaryOrigin {
				varyValues := rec.Header().Values("Vary")
				found := false
				for _, v := range varyValues {
					if strings.Contains(v, "Origin") {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Vary header should contain Origin, got %v", varyValues)
				}
			}
		})
	}
}

func TestCORS_AllowOriginFunc(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowOriginFunc: func(origin string, r *http.Request) bool {
			return strings.HasSuffix(origin, ".example.com")
		},
	}

	tests := []struct {
		name       string
		origin     string
		wantOrigin string
	}{
		{
			name:       "matching suffix",
			origin:     "https://app.example.com",
			wantOrigin: "https://app.example.com",
		},
		{
			name:       "non-matching suffix",
			origin:     "https://app.other.com",
			wantOrigin: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := chi.NewRouter()
			r.Use(CORS(cfg))
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			got := rec.Header().Get("Access-Control-Allow-Origin")
			if got != tt.wantOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantOrigin)
			}
		})
	}
}

func TestCORS_AllowOriginFuncTakesPrecedence(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://blocked.com"},
		AllowOriginFunc: func(origin string, r *http.Request) bool {
			return origin == "https://allowed.com"
		},
	}

	r := chi.NewRouter()
	r.Use(CORS(cfg))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://allowed.com")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.com" {
		t.Errorf("AllowOriginFunc should take precedence, got %q", got)
	}

	// The AllowedOrigins entry should be ignored.
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Origin", "https://blocked.com")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if got := rec2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("AllowOriginFunc should override AllowedOrigins, got %q", got)
	}
}

func TestCORS_Preflight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		config            CORSConfig
		wantMethods       string
		wantHeaders       string
		wantMaxAge        string
		wantStatus        int
		wantCredentials   string
		wantExposeHeaders string
	}{
		{
			name: "basic preflight",
			config: CORSConfig{
				AllowedOrigins:   []string{"https://example.com"},
				AllowedMethods:   []string{"GET", "POST", "PUT"},
				AllowedHeaders:   []string{"Content-Type", "Authorization"},
				HandlePreflight:  true,
			},
			wantMethods:     "GET, POST, PUT",
			wantHeaders:     "Content-Type, Authorization",
			wantStatus:      http.StatusNoContent,
		},
		{
			name: "preflight with credentials",
			config: CORSConfig{
				AllowedOrigins:   []string{"https://example.com"},
				AllowedMethods:   []string{"GET", "POST"},
				AllowCredentials: true,
				HandlePreflight:  true,
			},
			wantMethods:     "GET, POST",
			wantStatus:      http.StatusNoContent,
			wantCredentials: "true",
		},
		{
			name: "preflight with max-age",
			config: CORSConfig{
				AllowedOrigins:  []string{"https://example.com"},
				AllowedMethods:  []string{"GET"},
				MaxAge:          3600,
				HandlePreflight: true,
			},
			wantMethods: "GET",
			wantMaxAge:  "3600",
			wantStatus:  http.StatusNoContent,
		},
		{
			name: "preflight with exposed headers",
			config: CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET"},
				ExposedHeaders: []string{"X-Custom-Header", "X-Another"},
				HandlePreflight: true,
			},
			wantMethods:       "GET",
			wantStatus:        http.StatusNoContent,
			wantExposeHeaders: "X-Custom-Header, X-Another",
		},
		{
			name: "preflight not handled when HandlePreflight false",
			config: CORSConfig{
				AllowedOrigins:  []string{"https://example.com"},
				AllowedMethods:  []string{"GET"},
				HandlePreflight: false,
			},
			wantStatus: http.StatusOK, // passes through to the actual handler
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := chi.NewRouter()
			r.Use(CORS(tt.config))
			r.Options("/", func(w http.ResponseWriter, r *http.Request) {})
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

			req := httptest.NewRequest("OPTIONS", "/", nil)
			req.Header.Set("Origin", "https://example.com")
			req.Header.Set("Access-Control-Request-Method", "GET")
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if got := rec.Header().Get("Access-Control-Allow-Methods"); got != tt.wantMethods {
				t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, tt.wantMethods)
			}
			if got := rec.Header().Get("Access-Control-Allow-Headers"); got != tt.wantHeaders {
				t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, tt.wantHeaders)
			}
			if got := rec.Header().Get("Access-Control-Max-Age"); got != tt.wantMaxAge {
				t.Errorf("Access-Control-Max-Age = %q, want %q", got, tt.wantMaxAge)
			}
			if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != tt.wantCredentials {
				t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, tt.wantCredentials)
			}
			if got := rec.Header().Get("Access-Control-Expose-Headers"); got != tt.wantExposeHeaders {
				t.Errorf("Access-Control-Expose-Headers = %q, want %q", got, tt.wantExposeHeaders)
			}
		})
	}
}

func TestCORS_PreflightEchoRequestHeaders(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins:  []string{"https://example.com"},
		AllowedMethods:  []string{"GET", "POST"},
		HandlePreflight: true,
		// AllowedHeaders is empty, so request headers should be echoed.
	}

	r := chi.NewRouter()
	r.Use(CORS(cfg))
	r.Options("/", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-Custom, X-Another")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "X-Custom, X-Another" {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, "X-Custom, X-Another")
	}

	varyValues := rec.Header().Values("Vary")
	found := false
	for _, v := range varyValues {
		if strings.Contains(v, "Access-Control-Request-Headers") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Vary should contain Access-Control-Request-Headers, got %v", varyValues)
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	}

	r := chi.NewRouter()
	r.Use(CORS(cfg))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/", nil)
	// No Origin header set.
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("no CORS headers should be set without Origin, got %q", got)
	}
}

func TestCORS_DefaultMethods(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins:  []string{"https://example.com"},
		HandlePreflight: true,
	}

	r := chi.NewRouter()
	r.Use(CORS(cfg))
	r.Options("/", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	methods := rec.Header().Get("Access-Control-Allow-Methods")
	expected := "HEAD, GET, POST, PUT, DELETE, PATCH, OPTIONS"
	if methods != expected {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", methods, expected)
	}
}

func TestCORS_ExposeHeadersOnNormalRequest(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		ExposedHeaders: []string{"X-Total-Count", "X-Request-Id"},
	}

	r := chi.NewRouter()
	r.Use(CORS(cfg))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Total-Count, X-Request-Id" {
		t.Errorf("Access-Control-Expose-Headers = %q, want %q", got, "X-Total-Count, X-Request-Id")
	}
}

func TestCORS_DisallowedOriginNoCORSHeaders(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowCredentials: true,
		ExposedHeaders:   []string{"X-Custom"},
		MaxAge:           3600,
		HandlePreflight:  true,
	}

	r := chi.NewRouter()
	r.Use(CORS(cfg))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	corsHeaders := []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Credentials",
		"Access-Control-Expose-Headers",
	}
	for _, h := range corsHeaders {
		if got := rec.Header().Get(h); got != "" {
			t.Errorf("disallowed origin should not have %q header, got %q", h, got)
		}
	}
}

func TestCORS_MaxAgeNotSetWhenZero(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins:  []string{"https://example.com"},
		AllowedMethods:  []string{"GET"},
		MaxAge:          0,
		HandlePreflight: true,
	}

	r := chi.NewRouter()
	r.Use(CORS(cfg))
	r.Options("/", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Max-Age"); got != "" {
		t.Errorf("Access-Control-Max-Age should not be set when MaxAge=0, got %q", got)
	}
}

func TestCORS_AllowOriginFuncWithRequest(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowOriginFunc: func(origin string, r *http.Request) bool {
			// Only allow origins from the same host as the request's Host header.
			return strings.Contains(origin, r.Host)
		},
	}

	r := chi.NewRouter()
	r.Use(CORS(cfg))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Host = "example.com"
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://example.com")
	}
}

func TestCORS_PreflightNotOptions(t *testing.T) {
	t.Parallel()

	// A non-OPTIONS request should not get Access-Control-Allow-Methods etc.
	cfg := CORSConfig{
		AllowedOrigins:  []string{"https://example.com"},
		AllowedMethods:  []string{"GET", "POST"},
		AllowedHeaders:  []string{"Content-Type"},
		MaxAge:          3600,
		HandlePreflight: true,
	}

	r := chi.NewRouter()
	r.Use(CORS(cfg))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://example.com")
	}
	// Preflight-specific headers should NOT be set on normal requests.
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("Access-Control-Allow-Methods should not be set on normal request, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "" {
		t.Errorf("Access-Control-Max-Age should not be set on normal request, got %q", got)
	}
}

func TestCORS_MultipleOrigins(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://app.example.com", "https://admin.example.com"},
	}

	r := chi.NewRouter()
	r.Use(CORS(cfg))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

	// Test first allowed origin.
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("Origin", "https://app.example.com")
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	if got := rec1.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://app.example.com")
	}

	// Test second allowed origin.
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Origin", "https://admin.example.com")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if got := rec2.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://admin.example.com")
	}
}
