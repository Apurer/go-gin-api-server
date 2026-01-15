package errors

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ContentTypeProblemJSON is the media type for Problem Details responses.
const ContentTypeProblemJSON = "application/problem+json"

// Responder provides methods to send Problem Details responses.
type Responder struct {
	// BaseURI is prepended to problem type URIs if they are relative.
	BaseURI string
}

// NewResponder creates a new problem responder with optional base URI.
func NewResponder(baseURI string) *Responder {
	return &Responder{BaseURI: baseURI}
}

// DefaultResponder uses relative URIs for problem types.
var DefaultResponder = NewResponder("")

// Respond sends a ProblemDetail response with proper content type.
func (r *Responder) Respond(c *gin.Context, problem ProblemDetail) {
	if r.BaseURI != "" && len(problem.Type) > 0 && problem.Type[0] == '/' {
		problem.Type = r.BaseURI + problem.Type
	}
	if problem.Instance == "" {
		problem.Instance = c.Request.URL.Path
	}
	c.Header("Content-Type", ContentTypeProblemJSON)
	c.JSON(problem.Status, problem)
}

// RespondError converts a standard error to a ProblemDetail and responds.
// It checks if the error is already a ProblemDetail, otherwise wraps it.
func (r *Responder) RespondError(c *gin.Context, err error) {
	var problem ProblemDetail
	if errors.As(err, &problem) {
		r.Respond(c, problem)
		return
	}
	// Default to internal server error for unknown errors
	r.Respond(c, ErrInternal.WithDetail(err.Error()))
}

// NotFound sends a 404 problem response.
func (r *Responder) NotFound(c *gin.Context, resourceType string, identifier any) {
	r.Respond(c, NewNotFoundProblem(resourceType, identifier))
}

// BadRequest sends a 400 problem response.
func (r *Responder) BadRequest(c *gin.Context, detail string) {
	r.Respond(c, ErrBadRequest.WithDetail(detail))
}

// ValidationFailed sends a 400 problem response with field errors.
func (r *Responder) ValidationFailed(c *gin.Context, fieldErrors map[string]string) {
	r.Respond(c, NewValidationProblem(fieldErrors))
}

// InternalError sends a 500 problem response.
func (r *Responder) InternalError(c *gin.Context, detail string) {
	r.Respond(c, ErrInternal.WithDetail(detail))
}

// Respond is a convenience function using the default responder.
func Respond(c *gin.Context, problem ProblemDetail) {
	DefaultResponder.Respond(c, problem)
}

// RespondError is a convenience function using the default responder.
func RespondError(c *gin.Context, err error) {
	DefaultResponder.RespondError(c, err)
}

// ErrorMapper maps domain/application errors to ProblemDetail.
type ErrorMapper func(err error) (ProblemDetail, bool)

// ChainedResponder supports custom error mapping.
type ChainedResponder struct {
	*Responder
	mappers []ErrorMapper
}

// NewChainedResponder creates a responder with custom error mappers.
func NewChainedResponder(baseURI string, mappers ...ErrorMapper) *ChainedResponder {
	return &ChainedResponder{
		Responder: NewResponder(baseURI),
		mappers:   mappers,
	}
}

// AddMapper adds an error mapper to the chain.
func (r *ChainedResponder) AddMapper(mapper ErrorMapper) {
	r.mappers = append(r.mappers, mapper)
}

// RespondError tries each mapper before falling back to default handling.
func (r *ChainedResponder) RespondError(c *gin.Context, err error) {
	for _, mapper := range r.mappers {
		if problem, ok := mapper(err); ok {
			r.Respond(c, problem)
			return
		}
	}
	r.Responder.RespondError(c, err)
}

// HTTPStatusFromError extracts HTTP status from an error if possible.
func HTTPStatusFromError(err error) int {
	var problem ProblemDetail
	if errors.As(err, &problem) {
		return problem.Status
	}
	return http.StatusInternalServerError
}
