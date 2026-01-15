// Package errors provides RFC 7807 Problem Details for HTTP APIs.
package errors

import (
	"fmt"
	"net/http"
)

// ProblemDetail represents an RFC 7807 Problem Details response.
// See: https://www.rfc-editor.org/rfc/rfc7807
type ProblemDetail struct {
	// Type is a URI reference that identifies the problem type.
	Type string `json:"type"`
	// Title is a short, human-readable summary of the problem type.
	Title string `json:"title"`
	// Status is the HTTP status code for this occurrence.
	Status int `json:"status"`
	// Detail is a human-readable explanation specific to this occurrence.
	Detail string `json:"detail,omitempty"`
	// Instance is a URI reference that identifies the specific occurrence.
	Instance string `json:"instance,omitempty"`
	// Extensions holds additional problem-specific properties.
	Extensions map[string]any `json:"extensions,omitempty"`
}

// Error implements the error interface.
func (p ProblemDetail) Error() string {
	if p.Detail != "" {
		return fmt.Sprintf("%s: %s", p.Title, p.Detail)
	}
	return p.Title
}

// WithDetail returns a copy with the given detail message.
func (p ProblemDetail) WithDetail(detail string) ProblemDetail {
	p.Detail = detail
	return p
}

// WithInstance returns a copy with the given instance URI.
func (p ProblemDetail) WithInstance(instance string) ProblemDetail {
	p.Instance = instance
	return p
}

// WithExtension returns a copy with an additional extension property.
func (p ProblemDetail) WithExtension(key string, value any) ProblemDetail {
	if p.Extensions == nil {
		p.Extensions = make(map[string]any)
	}
	p.Extensions[key] = value
	return p
}

// Common problem types as URI references.
const (
	TypeValidation    = "/problems/validation-error"
	TypeNotFound      = "/problems/not-found"
	TypeConflict      = "/problems/conflict"
	TypeInternal      = "/problems/internal-error"
	TypeUnauthorized  = "/problems/unauthorized"
	TypeForbidden     = "/problems/forbidden"
	TypeBadRequest    = "/problems/bad-request"
	TypeUnprocessable = "/problems/unprocessable-entity"
)

// Pre-defined problem templates for common scenarios.
var (
	// ErrNotFound indicates the requested resource was not found.
	ErrNotFound = ProblemDetail{
		Type:   TypeNotFound,
		Title:  "Resource Not Found",
		Status: http.StatusNotFound,
	}

	// ErrValidation indicates the request failed validation.
	ErrValidation = ProblemDetail{
		Type:   TypeValidation,
		Title:  "Validation Error",
		Status: http.StatusBadRequest,
	}

	// ErrBadRequest indicates the request was malformed.
	ErrBadRequest = ProblemDetail{
		Type:   TypeBadRequest,
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
	}

	// ErrConflict indicates a conflict with the current state.
	ErrConflict = ProblemDetail{
		Type:   TypeConflict,
		Title:  "Conflict",
		Status: http.StatusConflict,
	}

	// ErrInternal indicates an unexpected server error.
	ErrInternal = ProblemDetail{
		Type:   TypeInternal,
		Title:  "Internal Server Error",
		Status: http.StatusInternalServerError,
	}

	// ErrUnauthorized indicates missing or invalid authentication.
	ErrUnauthorized = ProblemDetail{
		Type:   TypeUnauthorized,
		Title:  "Unauthorized",
		Status: http.StatusUnauthorized,
	}

	// ErrForbidden indicates the action is not allowed.
	ErrForbidden = ProblemDetail{
		Type:   TypeForbidden,
		Title:  "Forbidden",
		Status: http.StatusForbidden,
	}

	// ErrUnprocessable indicates the request was understood but cannot be processed.
	ErrUnprocessable = ProblemDetail{
		Type:   TypeUnprocessable,
		Title:  "Unprocessable Entity",
		Status: http.StatusUnprocessableEntity,
	}
)

// NewValidationProblem creates a validation error with field-level details.
func NewValidationProblem(fieldErrors map[string]string) ProblemDetail {
	return ErrValidation.WithExtension("fields", fieldErrors)
}

// NewNotFoundProblem creates a not found error for a specific resource.
func NewNotFoundProblem(resourceType string, identifier any) ProblemDetail {
	return ErrNotFound.
		WithDetail(fmt.Sprintf("%s with identifier '%v' not found", resourceType, identifier)).
		WithExtension("resourceType", resourceType).
		WithExtension("identifier", identifier)
}
