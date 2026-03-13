package httpjson

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, JSON{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	var result JSON
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["key"] != "value" {
		t.Errorf("body = %v", result)
	}
}

func TestWriteJSONWithStatus(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSONWithStatus(w, http.StatusCreated, JSON{"id": 42})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
	var result JSON
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["id"] != float64(42) {
		t.Errorf("body = %v", result)
	}
}

func TestWriteJSONBytes(t *testing.T) {
	w := httptest.NewRecorder()
	data := []byte(`{"raw":true}`)
	WriteJSONBytes(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != `{"raw":true}` {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestWriteJSONAllowHTML(t *testing.T) {
	w := httptest.NewRecorder()
	err := WriteJSONAllowHTML(w, JSON{"html": "<b>bold</b>"})
	if err != nil {
		t.Fatal(err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<b>bold</b>") {
		t.Errorf("HTML should not be escaped, got %q", body)
	}
}

func TestWriteJSON_HTMLEscaped(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, JSON{"html": "<script>alert(1)</script>"})

	body := w.Body.String()
	if strings.Contains(body, "<script>") {
		t.Errorf("HTML should be escaped in WriteJSON, got %q", body)
	}
}

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	body := strings.NewReader(`{"name":"test"}`)
	r := &http.Request{Body: io.NopCloser(body)}

	var p payload
	if err := DecodeJSON(r, &p); err != nil {
		t.Fatal(err)
	}
	if p.Name != "test" {
		t.Errorf("Name = %q, want %q", p.Name, "test")
	}
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{bad json}`)
	r := &http.Request{Body: io.NopCloser(body)}

	type payload struct{}
	var p payload
	if err := DecodeJSON(r, &p); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeJSONWithLimit(t *testing.T) {
	type payload struct {
		Msg string `json:"msg"`
	}

	body := strings.NewReader(`{"msg":"hello"}`)
	r := &http.Request{Body: io.NopCloser(body)}

	var p payload
	if err := DecodeJSONWithLimit(r, &p, 1024); err != nil {
		t.Fatal(err)
	}
	if p.Msg != "hello" {
		t.Errorf("Msg = %q", p.Msg)
	}
}

func TestDecodeJSONWithLimit_Exceeded(t *testing.T) {
	big := `{"msg":"` + strings.Repeat("x", 200) + `"}`
	body := strings.NewReader(big)
	r := &http.Request{Body: io.NopCloser(body)}

	type payload struct {
		Msg string `json:"msg"`
	}
	var p payload
	err := DecodeJSONWithLimit(r, &p, 10)
	if err == nil {
		t.Error("expected error for oversized body")
	}
}

func TestSendErrorJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	SendErrorJSON(w, r, nil, http.StatusBadRequest, io.EOF, "bad request")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	var result JSON
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["error"] != "bad request" {
		t.Errorf("body = %v", result)
	}
}

func TestParseDateRange_Valid(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?from=2025-01-01T00:00:00&to=2025-12-31T23:59:59", nil)
	from, to, err := ParseDateRange(r)
	if err != nil {
		t.Fatal(err)
	}
	if from.Year() != 2025 || from.Month() != 1 {
		t.Errorf("from = %v", from)
	}
	if to.Year() != 2025 || to.Month() != 12 {
		t.Errorf("to = %v", to)
	}
}

func TestParseDateRange_Invalid(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?from=bad&to=2025-01-01T00:00:00", nil)
	_, _, err := ParseDateRange(r)
	if err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestWriteJSON_UnencodableType(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, func() {})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for unencodable type", w.Code)
	}
}

func TestEncodeJSON_RoundTrip(t *testing.T) {
	data := JSON{"count": 42, "items": []string{"a", "b"}}
	buf, err := encodeJSON(data, true)
	if err != nil {
		t.Fatal(err)
	}

	var decoded JSON
	if err := json.NewDecoder(bytes.NewReader(buf)).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["count"] != float64(42) {
		t.Errorf("count = %v", decoded["count"])
	}
}
