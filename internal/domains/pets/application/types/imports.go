package types

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Apurer/go-gin-api-server/internal/domains/pets/domain"
)

// ErrIncompleteImport indicates that a partner payload cannot hydrate a Pet aggregate yet.
var ErrIncompleteImport = errors.New("partner import candidate missing required fields")

// PartnerImportCandidate captures the subset of partner data we can safely extract before validation.
type PartnerImportCandidate struct {
	Title             string
	Photos            []string
	Availability      string
	Labels            map[string]string
	ExternalReference *ExternalReferenceInput
}

// MissingFields lists which mandatory fields are absent to materialize a domain pet.
func (c PartnerImportCandidate) MissingFields() []string {
	var missing []string
	if strings.TrimSpace(c.Title) == "" {
		missing = append(missing, "title")
	}
	if len(c.Photos) == 0 {
		missing = append(missing, "photos")
	}
	return missing
}

// Validate checks whether the candidate can hydrate a domain aggregate.
func (c PartnerImportCandidate) Validate() error {
	if missing := c.MissingFields(); len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrIncompleteImport, strings.Join(missing, ", "))
	}
	return nil
}

// ToDomainPet attempts to materialize a domain pet from the import candidate.
func (c PartnerImportCandidate) ToDomainPet() (*domain.Pet, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	photos := append([]string{}, c.Photos...)
	pet, err := domain.NewPet(0, c.Title, photos)
	if err != nil {
		return nil, err
	}
	if c.Availability != "" {
		if err := pet.UpdateStatus(domain.Status(strings.ToLower(c.Availability))); err != nil {
			return nil, err
		}
	}
	if c.ExternalReference != nil {
		ref := domain.ExternalReference{
			Provider:   c.ExternalReference.Provider,
			ID:         c.ExternalReference.ID,
			Attributes: cloneStringMap(c.ExternalReference.Attributes),
		}
		pet.UpdateExternalReference(&ref)
	}
	if len(c.Labels) > 0 {
		tags := make([]domain.Tag, 0, len(c.Labels))
		for key := range c.Labels {
			tags = append(tags, domain.Tag{Name: key})
		}
		pet.ReplaceTags(tags)
	}
	return pet, nil
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	copy := make(map[string]string, len(source))
	for k, v := range source {
		copy[k] = v
	}
	return copy
}
