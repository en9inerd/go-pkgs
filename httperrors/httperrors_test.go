package httperrors

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewError(t *testing.T) {
	e := NewError(http.StatusNotFound, "not found")
	if e.Code != 404 {
		t.Errorf("Code = %d, want 404", e.Code)
	}
	if e.Message != "not found" {
		t.Errorf("Message = %q", e.Message)
	}
	if e.Error() != "not found" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestNewErrorWithDetails(t *testing.T) {
	e := NewErrorWithDetails(400, "bad request", "missing field")
	if e.Details != "missing field" {
		t.Errorf("Details = %q", e.Details)
	}
	if e.Error() != "bad request: missing field" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestNewErrorWithErr(t *testing.T) {
	inner := errors.New("connection refused")
	e := NewErrorWithErr(502, "gateway error", inner)
	if !errors.Is(e, inner) {
		t.Error("Unwrap should return inner error")
	}
	if e.Details != "" {
		t.Errorf("Details should be empty (not leak internal error), got %q", e.Details)
	}
}

func TestError_WriteJSON(t *testing.T) {
	e := NewErrorWithDetails(422, "validation failed", "email required")
	w := httptest.NewRecorder()
	e.WriteJSON(w)

	if w.Code != 422 {
		t.Errorf("status = %d, want 422", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["message"] != "validation failed" {
		t.Errorf("body = %v", result)
	}
	if result["details"] != "email required" {
		t.Errorf("details = %v", result["details"])
	}
}

func TestNewValidationError(t *testing.T) {
	ve := NewValidationError(
		map[string][]string{"email": {"required", "invalid format"}},
		[]string{"form expired"},
	)
	if ve.Error() != "form expired" {
		t.Errorf("Error() = %q, want %q", ve.Error(), "form expired")
	}
}

func TestValidationError_ErrorFallsBackToField(t *testing.T) {
	ve := NewValidationError(
		map[string][]string{"name": {"required"}},
		nil,
	)
	got := ve.Error()
	if got != "name: required" {
		t.Errorf("Error() = %q", got)
	}
}

func TestValidationError_ErrorFallsBackToDefault(t *testing.T) {
	ve := NewValidationError(nil, nil)
	if ve.Error() != "validation failed" {
		t.Errorf("Error() = %q", ve.Error())
	}
}

func TestValidationError_WriteJSON(t *testing.T) {
	ve := NewValidationError(
		map[string][]string{"email": {"required"}},
		nil,
	)
	w := httptest.NewRecorder()
	ve.WriteJSON(w)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["fieldErrors"] == nil {
		t.Error("expected fieldErrors in response")
	}
}

func TestNewAPIError(t *testing.T) {
	e := NewAPIError(503, "service unavailable")
	if e.Error() != "API error (code 503): service unavailable" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestNewAPIErrorWithDetails(t *testing.T) {
	e := NewAPIErrorWithDetails(429, "rate limited", "retry after 60s")
	if e.Details != "retry after 60s" {
		t.Errorf("Details = %q", e.Details)
	}
	expected := "API error (code 429): rate limited - retry after 60s"
	if e.Error() != expected {
		t.Errorf("Error() = %q, want %q", e.Error(), expected)
	}
}

func TestNewAPIErrorWithErr(t *testing.T) {
	inner := errors.New("timeout")
	e := NewAPIErrorWithErr(504, "gateway timeout", inner)
	if !errors.Is(e, inner) {
		t.Error("Unwrap should return inner error")
	}
}

func TestAPIError_WriteJSON(t *testing.T) {
	e := NewAPIError(500, "internal error")
	w := httptest.NewRecorder()
	e.WriteJSON(w)

	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["message"] != "internal error" {
		t.Errorf("body = %v", result)
	}
}

func TestNewNetworkError(t *testing.T) {
	inner := errors.New("connection reset")
	e := NewNetworkError("request failed", inner)
	if e.Error() != "request failed: connection reset" {
		t.Errorf("Error() = %q", e.Error())
	}
	if !errors.Is(e, inner) {
		t.Error("Unwrap should return inner error")
	}
}

func TestNetworkError_NilErr(t *testing.T) {
	e := NewNetworkError("no connection", nil)
	if e.Error() != "no connection" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestIsValidationError(t *testing.T) {
	ve := NewValidationError(nil, []string{"bad"})
	if !IsValidationError(ve) {
		t.Error("expected IsValidationError to return true")
	}
	if IsValidationError(errors.New("plain")) {
		t.Error("expected IsValidationError to return false for plain error")
	}
}

func TestIsAPIError(t *testing.T) {
	ae := NewAPIError(500, "fail")
	if !IsAPIError(ae) {
		t.Error("expected IsAPIError to return true")
	}
	if IsAPIError(errors.New("plain")) {
		t.Error("expected IsAPIError to return false for plain error")
	}
}

func TestIsNetworkError(t *testing.T) {
	ne := NewNetworkError("fail", nil)
	if !IsNetworkError(ne) {
		t.Error("expected IsNetworkError to return true")
	}
	if IsNetworkError(errors.New("plain")) {
		t.Error("expected IsNetworkError to return false")
	}
}

func TestNewErrorWithErr_NoDetailsLeak(t *testing.T) {
	inner := errors.New("pq: connection to 10.0.1.5:5432 refused")
	e := NewErrorWithErr(500, "internal error", inner)

	w := httptest.NewRecorder()
	e.WriteJSON(w)

	body := w.Body.String()
	if strings.Contains(body, "10.0.1.5") {
		t.Errorf("internal error details leaked to JSON: %s", body)
	}
}

func TestNewAPIErrorWithErr_NoDetailsLeak(t *testing.T) {
	inner := errors.New("redis: connection refused")
	e := NewAPIErrorWithErr(502, "bad gateway", inner)
	if e.Details != "" {
		t.Errorf("Details should be empty, got %q", e.Details)
	}
	if !errors.Is(e, inner) {
		t.Error("Unwrap should return inner error")
	}
}

func TestIsHTTPError(t *testing.T) {
	he := NewError(404, "not found")
	if !IsHTTPError(he) {
		t.Error("expected IsHTTPError to return true")
	}
	if IsHTTPError(errors.New("plain")) {
		t.Error("expected IsHTTPError to return false")
	}
}
