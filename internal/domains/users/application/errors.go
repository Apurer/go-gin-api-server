package application

import (
	"errors"
	"fmt"

	"github.com/Apurer/go-gin-api-server/internal/domains/users/domain"
	"github.com/Apurer/go-gin-api-server/internal/domains/users/ports"
)

var (
	// ErrInvalidInput signals the request violated a domain invariant.
	ErrInvalidInput = errors.New("invalid user input")
	// ErrAuthentication wraps authentication failures.
	ErrAuthentication = errors.New("authentication failed")
)

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrEmptyUsername) ||
		errors.Is(err, domain.ErrEmptyPassword) ||
		errors.Is(err, domain.ErrWeakPassword) ||
		errors.Is(err, domain.ErrInvalidEmail) {
		return fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	if errors.Is(err, ports.ErrInvalidCredentials) {
		return fmt.Errorf("%w: %w", ErrAuthentication, err)
	}
	return err
}
