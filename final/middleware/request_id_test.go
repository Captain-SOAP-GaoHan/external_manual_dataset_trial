package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func maintainDefaultRequestID() func() {
	original := RequestIDHeader

	return func() {
		RequestIDHeader = original
	}
}

func TestRequestID(t *testing.T) {
	tests := map[string]struct {
		requestIDHeader  string
		request          func() *http.Request
		expectedResponse string
	}{
		"Retrieves Request Id from default header": {
			"X-Request-Id",
			func() *http.Request {
				req, _ := http.NewRequest("GET", "/", nil)
				req.Header.Add("X-Request-Id", "req-123456")

				return req
			},
			"RequestID: req-123456",
		},
		"Retrieves Request Id from custom header": {
			"X-Trace-Id",
			func() *http.Request {
				req, _ := http.NewRequest("GET", "/", nil)
				req.Header.Add("X-Trace-Id", "trace:abc123")

				return req
			},
			"RequestID: trace:abc123",
		},
	}

	defer maintainDefaultRequestID()()

	for _, test := range tests {
		w := httptest.NewRecorder()

		r := chi.NewRouter()

		RequestIDHeader = test.requestIDHeader

		r.Use(RequestID)

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			requestID := GetReqID(r.Context())
			response := fmt.Sprintf("RequestID: %s", requestID)

			w.Write([]byte(response))
		})
		r.ServeHTTP(w, test.request())

		if w.Body.String() != test.expectedResponse {
			t.Fatalf("RequestID was not the expected value")
		}
	}
}

func TestRequestIDWithConfig_CustomGenerator(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequestIDWithConfig(RequestIDConfig{
		Generator: func(r *http.Request) string {
			return "custom-id-12345"
		},
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		requestID := GetReqID(r.Context())
		w.Write([]byte(requestID))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	if w.Body.String() != "custom-id-12345" {
		t.Fatalf("expected custom-id-12345, got %s", w.Body.String())
	}
}

func TestRequestIDWithConfig_ResponseHeader(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequestIDWithConfig(RequestIDConfig{
		ResponseHeader: true,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	responseID := w.Header().Get("X-Request-Id")
	if responseID == "" {
		t.Fatal("expected X-Request-Id response header to be set")
	}

	contextID := ""
	r.Get("/check", func(w http.ResponseWriter, r *http.Request) {
		contextID = GetReqID(r.Context())
		w.Write([]byte("ok"))
	})

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/check", nil)
	r.ServeHTTP(w2, req2)
	responseID2 := w2.Header().Get("X-Request-Id")

	if responseID2 != contextID {
		t.Fatalf("response header %q should match context value %q", responseID2, contextID)
	}
}

func TestRequestIDWithConfig_ResponseHeaderWithExistingID(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequestIDWithConfig(RequestIDConfig{
		ResponseHeader: true,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-Id", "existing-id-999")
	r.ServeHTTP(w, req)

	responseID := w.Header().Get("X-Request-Id")
	if responseID != "existing-id-999" {
		t.Fatalf("expected response header existing-id-999, got %s", responseID)
	}
}

func TestRequestIDWithConfig_ExtraHeaders(t *testing.T) {
	tests := map[string]struct {
		setupRequest     func() *http.Request
		expectedResponse string
	}{
		"reads from primary header": {
			func() *http.Request {
				req, _ := http.NewRequest("GET", "/", nil)
				req.Header.Set("X-Request-Id", "primary-id")
				return req
			},
			"primary-id",
		},
		"falls back to first extra header": {
			func() *http.Request {
				req, _ := http.NewRequest("GET", "/", nil)
				req.Header.Set("X-Trace-Id", "trace-id")
				return req
			},
			"trace-id",
		},
		"falls back to second extra header": {
			func() *http.Request {
				req, _ := http.NewRequest("GET", "/", nil)
				req.Header.Set("X-Correlation-Id", "corr-id")
				return req
			},
			"corr-id",
		},
		"primary takes precedence over extra headers": {
			func() *http.Request {
				req, _ := http.NewRequest("GET", "/", nil)
				req.Header.Set("X-Request-Id", "primary-id")
				req.Header.Set("X-Trace-Id", "trace-id")
				return req
			},
			"primary-id",
		},
		"first extra header takes precedence over second": {
			func() *http.Request {
				req, _ := http.NewRequest("GET", "/", nil)
				req.Header.Set("X-Trace-Id", "trace-id")
				req.Header.Set("X-Correlation-Id", "corr-id")
				return req
			},
			"trace-id",
		},
		"generates ID when no headers present": {
			func() *http.Request {
				req, _ := http.NewRequest("GET", "/", nil)
				return req
			},
			"",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(RequestIDWithConfig(RequestIDConfig{
				ExtraHeaders: []string{"X-Trace-Id", "X-Correlation-Id"},
			}))

			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				requestID := GetReqID(r.Context())
				w.Write([]byte(requestID))
			})

			w := httptest.NewRecorder()
			r.ServeHTTP(w, test.setupRequest())

			if test.expectedResponse == "" {
				// For generated IDs, just verify it's not empty
				if w.Body.String() == "" {
					t.Fatal("expected a generated request ID, got empty string")
				}
			} else if w.Body.String() != test.expectedResponse {
				t.Fatalf("expected %s, got %s", test.expectedResponse, w.Body.String())
			}
		})
	}
}

func TestRequestIDWithConfig_CustomHeader(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequestIDWithConfig(RequestIDConfig{
		Header: "X-Custom-Req-Id",
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		requestID := GetReqID(r.Context())
		w.Write([]byte(requestID))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-Custom-Req-Id", "custom-header-id")
	r.ServeHTTP(w, req)

	if w.Body.String() != "custom-header-id" {
		t.Fatalf("expected custom-header-id, got %s", w.Body.String())
	}
}

func TestRequestIDWithConfig_DefaultGenerator(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequestIDWithConfig(RequestIDConfig{}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		requestID := GetReqID(r.Context())
		w.Write([]byte(requestID))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	if w.Body.String() == "" {
		t.Fatal("expected a generated request ID, got empty string")
	}
}

func TestRequestIDWithConfig_ResponseHeaderWithCustomHeader(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequestIDWithConfig(RequestIDConfig{
		Header:         "X-Trace-Id",
		ResponseHeader: true,
		Generator: func(r *http.Request) string {
			return "gen-trace-id"
		},
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	responseID := w.Header().Get("X-Trace-Id")
	if responseID != "gen-trace-id" {
		t.Fatalf("expected X-Trace-Id response header gen-trace-id, got %s", responseID)
	}
}

func TestRequestIDWithConfig_NoResponseHeaderByDefault(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequestIDWithConfig(RequestIDConfig{
		Generator: func(r *http.Request) string {
			return "test-id"
		},
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	responseID := w.Header().Get("X-Request-Id")
	if responseID != "" {
		t.Fatalf("expected no X-Request-Id response header by default, got %s", responseID)
	}
}

func TestGetReqID_NilContext(t *testing.T) {
	if GetReqID(nil) != "" {
		t.Fatal("expected empty string for nil context")
	}
}

func TestGetReqID_NoRequestID(t *testing.T) {
	ctx := context.Background()
	if GetReqID(ctx) != "" {
		t.Fatal("expected empty string for context without request ID")
	}
}
