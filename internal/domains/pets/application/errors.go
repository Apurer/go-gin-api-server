package application

import (
	"errors"
	"fmt"

	"github.com/Apurer/go-gin-api-server/internal/domains/pets/domain"
)

var (
	// ErrInvalidInput signals the request violated a domain invariant.
	ErrInvalidInput = errors.New("invalid pet input")
	// ErrPartnerSync wraps failures pushing changes to an external partner.
	ErrPartnerSync = errors.New("partner sync failed")
)

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrEmptyName) ||
		errors.Is(err, domain.ErrEmptyPhotos) ||
		errors.Is(err, domain.ErrInvalidHair) ||
		errors.Is(err, domain.ErrInvalidGrooming) ||
		errors.Is(err, domain.ErrInvalidStatus) {
		return fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	return err
}
