// Package httperrors provides structured error types and utilities for HTTP services
package httperrors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Error represents a structured HTTP error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Err     error  `json:"-"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Details)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Err
}

// WriteJSON writes the error as JSON to the response
func (e *Error) WriteJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.Code)
	json.NewEncoder(w).Encode(e)
}

// ValidationError represents a validation error
type ValidationError struct {
	FieldErrors    map[string][]string `json:"fieldErrors"`
	NonFieldErrors []string            `json:"nonFieldErrors"`
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if len(e.NonFieldErrors) > 0 {
		return e.NonFieldErrors[0]
	}
	if len(e.FieldErrors) > 0 {
		for field, errors := range e.FieldErrors {
			if len(errors) > 0 {
				return fmt.Sprintf("%s: %s", field, errors[0])
			}
		}
	}
	return "validation failed"
}

// WriteJSON writes the validation error as JSON to the response
func (e *ValidationError) WriteJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(e)
}

// APIError represents an error from an external API
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Err     error  `json:"-"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("API error (code %d): %s - %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("API error (code %d): %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *APIError) Unwrap() error {
	return e.Err
}

// WriteJSON writes the API error as JSON to the response
func (e *APIError) WriteJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.Code)
	json.NewEncoder(w).Encode(e)
}

// NetworkError represents a network-related error
type NetworkError struct {
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// Error implements the error interface
func (e *NetworkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *NetworkError) Unwrap() error {
	return e.Err
}

// NewError creates a new HTTP error
func NewError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithDetails creates a new HTTP error with details
func NewErrorWithDetails(code int, message, details string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// NewErrorWithErr creates a new HTTP error wrapping an underlying error
func NewErrorWithErr(code int, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
		Details: err.Error(),
	}
}

// NewValidationError creates a new validation error
func NewValidationError(fieldErrors map[string][]string, nonFieldErrors []string) *ValidationError {
	return &ValidationError{
		FieldErrors:    fieldErrors,
		NonFieldErrors: nonFieldErrors,
	}
}

// NewAPIError creates a new API error
func NewAPIError(code int, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
	}
}

// NewAPIErrorWithDetails creates a new API error with details
func NewAPIErrorWithDetails(code int, message, details string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// NewAPIErrorWithErr creates a new API error wrapping an underlying error
func NewAPIErrorWithErr(code int, message string, err error) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Err:     err,
		Details: err.Error(),
	}
}

// NewNetworkError creates a new network error
func NewNetworkError(message string, err error) *NetworkError {
	return &NetworkError{
		Message: message,
		Err:     err,
	}
}

// IsValidationError checks if an error is a ValidationError
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// IsAPIError checks if an error is an APIError
func IsAPIError(err error) bool {
	var ae *APIError
	return errors.As(err, &ae)
}

// IsNetworkError checks if an error is a NetworkError
func IsNetworkError(err error) bool {
	var ne *NetworkError
	return errors.As(err, &ne)
}

// IsHTTPError checks if an error is an HTTP Error
func IsHTTPError(err error) bool {
	var he *Error
	return errors.As(err, &he)
}
