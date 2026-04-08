package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_EmptyOriginNoOp(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := CORS(CORSConfig{})(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("inner handler was not called")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin should be empty, got %q", got)
	}
}

func TestCORS_PreflightNoMethodsOrHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called for preflight")
	})

	handler := CORS(CORSConfig{Origin: "https://example.com"})(inner)
	req := httptest.NewRequest(http.MethodOptions, "/api", nil)
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Allow-Origin = %q, want %q", got, "https://example.com")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("Allow-Methods should be empty when no methods configured, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Errorf("Allow-Headers = %q, want %q (default)", got, "Content-Type")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("Max-Age = %q, want %q", got, "86400")
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want %q", got, "Origin")
	}
}

func TestCORS_PreflightCustom(t *testing.T) {
	handler := CORS(CORSConfig{
		Origin:  "https://example.com",
		Methods: []string{"PUT", "DELETE"},
		Headers: []string{"Authorization", "X-Custom"},
		MaxAge:  3600,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Access-Control-Request-Method", "PUT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "PUT, DELETE" {
		t.Errorf("Allow-Methods = %q, want %q", got, "PUT, DELETE")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, X-Custom" {
		t.Errorf("Allow-Headers = %q, want %q", got, "Authorization, X-Custom")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "3600" {
		t.Errorf("Max-Age = %q, want %q", got, "3600")
	}
}

func TestCORS_RegularRequest(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS(CORSConfig{Origin: "https://example.com"})(inner)
	req := httptest.NewRequest(http.MethodPost, "/api", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("inner handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Allow-Origin = %q, want %q", got, "https://example.com")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("Allow-Methods should be empty on non-preflight, got %q", got)
	}
}

func TestCORS_WildcardNoVary(t *testing.T) {
	handler := CORS(CORSConfig{Origin: "*"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, want %q", got, "*")
	}
	if got := rec.Header().Get("Vary"); got != "" {
		t.Errorf("Vary should be empty for wildcard, got %q", got)
	}
}

func TestCORS_Credentials(t *testing.T) {
	handler := CORS(CORSConfig{
		Origin:      "https://example.com",
		Credentials: true,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want %q", got, "true")
	}
}

func TestCORS_NoCredentialsByDefault(t *testing.T) {
	handler := CORS(CORSConfig{Origin: "https://example.com"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Allow-Credentials should be empty, got %q", got)
	}
}

func TestCORS_ExposedHeaders(t *testing.T) {
	handler := CORS(CORSConfig{
		Origin:         "https://example.com",
		ExposedHeaders: []string{"X-Request-Id", "X-Total-Count"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-Id, X-Total-Count" {
		t.Errorf("Expose-Headers = %q, want %q", got, "X-Request-Id, X-Total-Count")
	}
}

func TestCORS_NoExposedHeadersByDefault(t *testing.T) {
	handler := CORS(CORSConfig{Origin: "https://example.com"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "" {
		t.Errorf("Expose-Headers should be empty, got %q", got)
	}
}

func TestCORS_NonPreflightOptionsPassesThrough(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS(CORSConfig{Origin: "https://example.com"})(inner)
	req := httptest.NewRequest(http.MethodOptions, "/api", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("non-preflight OPTIONS should pass through to inner handler")
	}
}

func TestCORS_CredentialsWithWildcardOriginPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for Credentials=true with Origin=\"*\"")
		}
	}()
	CORS(CORSConfig{Origin: "*", Credentials: true})
}
