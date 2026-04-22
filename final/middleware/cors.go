package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig holds the configuration for the CORS middleware.
type CORSConfig struct {
	// AllowedOrigins is a list of origins allowed to make cross-origin requests.
	// Use "*" to allow all origins. Origins are matched exactly unless
	// the origin is "*", in which case any origin is allowed.
	AllowedOrigins []string

	// AllowOriginFunc is a function that evaluates the request origin and
	// returns true if the origin is allowed. If set, it takes precedence
	// over AllowedOrigins.
	AllowOriginFunc func(origin string, r *http.Request) bool

	// AllowedMethods is a list of methods allowed in CORS requests.
	// Defaults to ["HEAD", "GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"]
	// if empty.
	AllowedMethods []string

	// AllowedHeaders is a list of non-simple headers that are allowed
	// in CORS requests. If empty, the request's Access-Control-Request-Headers
	// header is echoed back.
	AllowedHeaders []string

	// ExposedHeaders is a list of headers that browsers are allowed to access.
	ExposedHeaders []string

	// AllowCredentials indicates whether the request can include user
	// credentials like cookies, HTTP authentication or client side SSL certificates.
	AllowCredentials bool

	// MaxAge indicates how long (in seconds) the results of a preflight
	// request can be cached. A value of 0 means no Access-Control-Max-Age
	// header is set.
	MaxAge int

	// HandlePreflight determines whether the middleware should handle
	// preflight (OPTIONS) requests automatically. If false, preflight
	// requests are passed through to the next handler.
	HandlePreflight bool
}

// corsConfigDefaults returns a copy of cfg with default values filled in.
func corsConfigDefaults(cfg CORSConfig) CORSConfig {
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{"HEAD", "GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}
	}
	return cfg
}

// CORS returns a middleware handler that adds Cross-Origin Resource Sharing
// (CORS) headers to the response based on the provided configuration.
//
// Example usage:
//
//	r.Use(middleware.CORS(middleware.CORSConfig{
//	    AllowedOrigins:   []string{"https://example.com", "https://app.example.com"},
//	    AllowedMethods:   []string{"GET", "POST", "PUT"},
//	    AllowedHeaders:   []string{"Content-Type", "Authorization"},
//	    ExposedHeaders:   []string{"X-Custom-Header"},
//	    AllowCredentials: true,
//	    MaxAge:           3600,
//	    HandlePreflight:  true,
//	}))
//
// Or with a dynamic origin function:
//
//	r.Use(middleware.CORS(middleware.CORSConfig{
//	    AllowOriginFunc: func(origin string, r *http.Request) bool {
//	        return strings.HasSuffix(origin, ".example.com")
//	    },
//	    ...
//	}))
func CORS(cfg CORSConfig) func(next http.Handler) http.Handler {
	cfg = corsConfigDefaults(cfg)

	// Pre-compute static header values for common case.
	allowedMethodsStr := strings.Join(cfg.AllowedMethods, ", ")
	allowedHeadersStr := strings.Join(cfg.AllowedHeaders, ", ")
	exposedHeadersStr := strings.Join(cfg.ExposedHeaders, ", ")

	// Build allowed origins lookup map for fast matching when AllowOriginFunc is not set.
	allowedOriginsSet := make(map[string]bool, len(cfg.AllowedOrigins))
	allowAllOrigins := false
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			allowAllOrigins = true
		}
		allowedOriginsSet[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Determine if the origin is allowed.
			allowed := false
			if cfg.AllowOriginFunc != nil {
				allowed = cfg.AllowOriginFunc(origin, r)
			} else {
				allowed = allowAllOrigins || allowedOriginsSet[origin]
			}

			if !allowed {
				// Origin not allowed — still call next handler, but don't set CORS headers.
				next.ServeHTTP(w, r)
				return
			}

			// Set Access-Control-Allow-Origin.
			if allowAllOrigins && !cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				// Vary: Origin is needed when the allowed origin varies per request.
				w.Header().Add("Vary", "Origin")
			}

			// Set Access-Control-Allow-Credentials.
			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Set Access-Control-Expose-Headers.
			if exposedHeadersStr != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposedHeadersStr)
			}

			// Handle preflight request.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if cfg.HandlePreflight {
					w.Header().Set("Access-Control-Allow-Methods", allowedMethodsStr)

					// Allowed headers: use configured list or echo the request headers.
					if allowedHeadersStr != "" {
						w.Header().Set("Access-Control-Allow-Headers", allowedHeadersStr)
					} else {
						requestHeaders := r.Header.Get("Access-Control-Request-Headers")
						if requestHeaders != "" {
							w.Header().Set("Access-Control-Allow-Headers", requestHeaders)
							w.Header().Add("Vary", "Access-Control-Request-Headers")
						}
					}

					// Set Max-Age.
					if cfg.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
					}

					w.WriteHeader(http.StatusNoContent)
					return
				}
				// If HandlePreflight is false, fall through to next handler.
			}

			next.ServeHTTP(w, r)
		})
	}
}
