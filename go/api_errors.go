package petstoreserver

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apierrors "github.com/Apurer/go-gin-api-server/internal/shared/errors"
)

// respondProblem maps a ProblemDetail through the shared responder.
func respondProblem(c *gin.Context, problem apierrors.ProblemDetail) {
	apierrors.Respond(c, problem)
}

// respondError preserves the existing call sites while returning RFC 7807 responses.
func respondError(c *gin.Context, status int, err error) {
	if err == nil {
		return
	}
	var problem apierrors.ProblemDetail
	switch status {
	case http.StatusBadRequest:
		problem = apierrors.ErrBadRequest.WithDetail(err.Error())
	case http.StatusNotFound:
		problem = apierrors.ErrNotFound.WithDetail(err.Error())
	case http.StatusUnauthorized:
		problem = apierrors.ErrUnauthorized.WithDetail(err.Error())
	case http.StatusForbidden:
		problem = apierrors.ErrForbidden.WithDetail(err.Error())
	default:
		problem = apierrors.ErrInternal.WithDetail(err.Error())
	}
	respondProblem(c, problem)
}
