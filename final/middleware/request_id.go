package middleware

// Ported from Goji's middleware, source:
// https://github.com/zenazn/goji/tree/master/web/middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

// Key to use when setting the request ID.
type ctxKeyRequestID int

// RequestIDKey is the key that holds the unique request ID in a request context.
const RequestIDKey ctxKeyRequestID = 0

// RequestIDHeader is the name of the HTTP Header which contains the request id.
// Exported so that it can be changed by developers
var RequestIDHeader = "X-Request-Id"

var prefix string
var reqid atomic.Uint64

// A quick note on the statistics here: we're trying to calculate the chance that
// two randomly generated base62 prefixes will collide. We use the formula from
// http://en.wikipedia.org/wiki/Birthday_problem
//
// P[m, n] \approx 1 - e^{-m^2/2n}
//
// We ballpark an upper bound for $m$ by imagining (for whatever reason) a server
// that restarts every second over 10 years, for $m = 86400 * 365 * 10 = 315360000$
//
// For a $k$ character base-62 identifier, we have $n(k) = 62^k$
//
// Plugging this in, we find $P[m, n(10)] \approx 5.75%$, which is good enough for
// our purposes, and is surely more than anyone would ever need in practice -- a
// process that is rebooted a handful of times a day for a hundred years has less
// than a millionth of a percent chance of generating two colliding IDs.

func init() {
	hostname, err := os.Hostname()
	if hostname == "" || err != nil {
		hostname = "localhost"
	}
	var buf [12]byte
	var b64 string
	for len(b64) < 10 {
		rand.Read(buf[:])
		b64 = base64.StdEncoding.EncodeToString(buf[:])
		b64 = strings.NewReplacer("+", "", "/", "").Replace(b64)
	}

	prefix = fmt.Sprintf("%s/%s", hostname, b64[0:10])
}

// RequestID is a middleware that injects a request ID into the context of each
// request. A request ID is a string of the form "host.example.com/random-0001",
// where "random" is a base62 random string that uniquely identifies this go
// process, and where the last number is an atomically incremented request
// counter.
func RequestID(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			myid := reqid.Add(1)
			requestID = fmt.Sprintf("%s-%06d", prefix, myid)
		}
		w.Header().Set(RequestIDHeader, requestID)
		ctx = context.WithValue(ctx, RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// RequestIDGeneratorFunc is a function that generates a new request ID.
// It receives the current request as an argument, allowing for request-aware
// ID generation strategies.
type RequestIDGeneratorFunc func(r *http.Request) string

// RequestIDConfig defines the configuration options for the RequestID middleware.
type RequestIDConfig struct {
	// Header is the name of the HTTP header to read the existing request ID from.
	// If left empty, it defaults to the package-level RequestIDHeader variable ("X-Request-Id").
	Header string

	// ExtraHeaders is an optional list of additional headers to check for an existing
	// request ID, in order of priority. They are checked after the primary Header.
	// This is useful for distributed tracing scenarios where different systems use
	// different header names (e.g., "X-Trace-Id", "X-Correlation-Id").
	ExtraHeaders []string

	// Generator is an optional function for generating a new request ID when one
	// is not found in the request headers. If not set, the default generator
	// produces IDs of the form "hostname/random-000001" using an atomic counter.
	Generator RequestIDGeneratorFunc

	// ResponseHeader controls whether the request ID is written to the response
	// headers. When true, the middleware sets the request ID on the response using
	// the same header name it was read from. Default is false for backward compatibility.
	ResponseHeader bool
}

// RequestIDWithConfig returns a RequestID middleware configured with the given options.
//
// Example usage:
//
//	// Custom generator with response header
//	r.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{
//	    Generator:      func(r *http.Request) string {
//	        return uuid.New().String()
//	    },
//	    ResponseHeader: true,
//	}))
//
//	// Multiple header lookup for distributed tracing
//	r.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{
//	    ExtraHeaders: []string{"X-Trace-Id", "X-Correlation-Id"},
//	}))
func RequestIDWithConfig(config RequestIDConfig) func(next http.Handler) http.Handler {
	// Determine the primary header to use.
	header := config.Header
	if header == "" {
		header = RequestIDHeader
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Try the primary header first.
			requestID := r.Header.Get(header)

			// If not found, try the extra headers in order.
			if requestID == "" {
				for _, h := range config.ExtraHeaders {
					requestID = r.Header.Get(h)
					if requestID != "" {
						break
					}
				}
			}

			// If still no ID, generate a new one.
			if requestID == "" {
				if config.Generator != nil {
					requestID = config.Generator(r)
				} else {
					myid := reqid.Add(1)
					requestID = fmt.Sprintf("%s-%06d", prefix, myid)
				}
			}

			// Optionally write the request ID to the response header.
			if config.ResponseHeader {
				w.Header().Set(header, requestID)
			}

			ctx = context.WithValue(ctx, RequestIDKey, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

// GetReqID returns a request ID from the given context if one is present.
// Returns the empty string if a request ID cannot be found.
func GetReqID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		return reqID
	}
	return ""
}

// NextRequestID generates the next request ID in the sequence.
func NextRequestID() uint64 {
	return reqid.Add(1)
}
