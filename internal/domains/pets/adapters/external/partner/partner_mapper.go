package partner

import (
	"strconv"
	"strings"

	partnerclient "github.com/Apurer/go-gin-api-server/internal/clients/http/partner"
	petstypes "github.com/Apurer/go-gin-api-server/internal/domains/pets/application/types"
	"github.com/Apurer/go-gin-api-server/internal/domains/pets/domain"
)

// ToPayload converts the local domain aggregate into the partner payload shape.
func ToPayload(p *domain.Pet) partnerclient.PetPayload {
	availability := strings.ToUpper(string(p.Status))
	if availability == "" {
		availability = "AVAILABLE"
	}
	labels := make(map[string]string, len(p.Tags))
	for _, tag := range p.Tags {
		labels[tag.Name] = "true"
	}
	if p.ExternalRef != nil {
		for k, v := range p.ExternalRef.Attributes {
			labels[k] = v
		}
	}
	var labelsPtr *map[string]string
	if len(labels) > 0 {
		labelsPtr = &labels
	}
	return partnerclient.PetPayload{
		Reference:    strconv.FormatInt(p.ID, 10),
		Title:        p.Name,
		Photos:       append([]string{}, p.PhotoURLs...),
		Labels:       labelsPtr,
		Availability: availability,
	}
}

// FromPayload builds an import candidate the application layer can vet before hydrating a domain pet.
func FromPayload(payload partnerclient.PetPayload) petstypes.PartnerImportCandidate {
	photos := append([]string{}, payload.Photos...)
	availability := strings.TrimSpace(payload.Availability)
	if availability == "" {
		availability = "available"
	}
	labels := cloneLabels(payload.Labels)
	var externalRef *petstypes.ExternalReferenceInput
	if payload.Reference != "" {
		externalRef = &petstypes.ExternalReferenceInput{
			Provider:   "partner",
			ID:         payload.Reference,
			Attributes: cloneLabels(payload.Labels),
		}
	}
	return petstypes.PartnerImportCandidate{
		Title:             strings.TrimSpace(payload.Title),
		Photos:            photos,
		Availability:      availability,
		Labels:            labels,
		ExternalReference: externalRef,
	}
}

func cloneLabels(labels *map[string]string) map[string]string {
	if labels == nil || len(*labels) == 0 {
		return nil
	}
	copy := make(map[string]string, len(*labels))
	for k, v := range *labels {
		copy[k] = v
	}
	return copy
}
