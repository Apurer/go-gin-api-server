package application

import (
	"errors"
	"fmt"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
)

// ErrInvalidInput signals the request violated a domain invariant.
var ErrInvalidInput = errors.New("invalid pet input")

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrEmptyName) ||
		errors.Is(err, domain.ErrEmptyPhotos) ||
		errors.Is(err, domain.ErrInvalidHair) ||
		errors.Is(err, domain.ErrInvalidGrooming) {
		return fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	return err
}
