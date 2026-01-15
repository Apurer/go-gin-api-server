package application

import (
	"errors"
	"fmt"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/domain"
)

var (
	// ErrInvalidInput signals the request violated a domain invariant.
	ErrInvalidInput = errors.New("invalid order input")
)

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrInvalidPetID) ||
		errors.Is(err, domain.ErrInvalidQuantity) ||
		errors.Is(err, domain.ErrInvalidStatus) {
		return fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	return err
}
