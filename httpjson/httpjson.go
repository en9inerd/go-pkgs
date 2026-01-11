// Package httpjson provides common helpers for JSON-based HTTP services
package httpjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// JSON is a convenience alias for a generic JSON object
type JSON map[string]any

// encodeJSON encodes data to JSON with HTML escaping control
func encodeJSON(data any, escapeHTML bool) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(escapeHTML)
	if err := enc.Encode(data); err != nil {
		return nil, fmt.Errorf("json encoding failed: %w", err)
	}
	return buf.Bytes(), nil
}

// writeResponse writes JSON bytes with status code
func writeResponse(w http.ResponseWriter, data []byte, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if code != 0 {
		w.WriteHeader(code)
	}
	_, _ = w.Write(data)
}

// WriteJSON encodes and writes JSON to the response with HTTP 200
func WriteJSON(w http.ResponseWriter, data any) {
	encoded, err := encodeJSON(data, true)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	writeResponse(w, encoded, 0)
}

// WriteJSONWithStatus encodes and writes JSON with the given HTTP status code
func WriteJSONWithStatus(w http.ResponseWriter, code int, data any) {
	encoded, err := encodeJSON(data, true)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	writeResponse(w, encoded, code)
}

// WriteJSONBytes writes pre-encoded JSON bytes to the response
func WriteJSONBytes(w http.ResponseWriter, data []byte) {
	writeResponse(w, data, 0)
}

// WriteJSONAllowHTML encodes and writes JSON with HTML characters unescaped.
//
// SECURITY WARNING: This function does not escape HTML characters in JSON values.
// Only use this function if you are certain that:
//   - The JSON will be properly escaped when rendered in HTML on the client side
//   - The JSON data does not contain user-controlled content that could lead to XSS
//
// For most use cases, use WriteJSON instead, which escapes HTML characters by default.
func WriteJSONAllowHTML(w http.ResponseWriter, v any) error {
	data, err := encodeJSON(v, false)
	if err != nil {
		return err
	}
	WriteJSONBytes(w, data)
	return nil
}

// ParseDateRange extracts "from" and "to" query parameters and parses them as time.Time
func ParseDateRange(r *http.Request) (from, to time.Time, err error) {
	parseTimestamp := func(ts string) (time.Time, error) {
		formats := []string{
			"2006-01-02T15:04:05.000000000",
			"2006-01-02T15:04:05",
			"2006-01-02T15:04",
			"20060102",
			time.RFC3339,
			time.RFC3339Nano,
		}
		for _, f := range formats {
			if t, e := time.Parse(f, ts); e == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("can't parse date %q", ts)
	}

	if from, err = parseTimestamp(r.URL.Query().Get("from")); err != nil {
		return from, to, fmt.Errorf("invalid 'from' time: %w", err)
	}

	if to, err = parseTimestamp(r.URL.Query().Get("to")); err != nil {
		return from, to, fmt.Errorf("invalid 'to' time: %w", err)
	}
	return from, to, nil
}

// DecodeJSON decodes JSON from request body into the given struct.
// The request body should be limited using SizeLimit middleware or http.MaxBytesReader
// to prevent DoS attacks via large JSON payloads.
func DecodeJSON[T any](r *http.Request, target *T) error {
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

// DecodeJSONWithLimit decodes JSON from request body into the given struct with a size limit.
// This prevents DoS attacks via large JSON payloads.
func DecodeJSONWithLimit[T any](r *http.Request, target *T, maxSize int64) error {
	limitedBody := http.MaxBytesReader(nil, r.Body, maxSize)
	if err := json.NewDecoder(limitedBody).Decode(&target); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}