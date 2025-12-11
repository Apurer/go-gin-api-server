package mapper

import (
	"errors"
	"time"

	petstypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/types"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
)

// Category is the HTTP representation of a pet category.
type Category struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Tag is the HTTP representation of a pet tag.
type Tag struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// ExternalReference mirrors the API payload describing a linked provider record.
type ExternalReference struct {
	Provider   string            `json:"provider,omitempty"`
	ID         string            `json:"externalId,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// GroomingOperation carries transient grooming data.
type GroomingOperation struct {
	InitialHairLengthCm *float64 `json:"initialHairLengthCm"`
	TrimByCm            *float64 `json:"trimByCm"`
}

var (
	errMissingInitial = errors.New("initialHairLengthCm is required")
	errMissingTrim    = errors.New("trimByCm is required")
)

// MutationPet captures inbound payloads for create/update flows while preserving field presence.
type MutationPet struct {
	ID                int64              `json:"id,omitempty"`
	Category          *Category          `json:"category,omitempty"`
	Name              *string            `json:"name,omitempty"`
	PhotoURLs         *[]string          `json:"photoUrls,omitempty"`
	Tags              *[]Tag             `json:"tags,omitempty"`
	Status            *string            `json:"status,omitempty"`
	HairLengthCm      *float64           `json:"hairLengthCm,omitempty"`
	ExternalReference *ExternalReference `json:"externalReference,omitempty"`
}

// Pet is the HTTP representation used for mapping between transport and domain responses.
type Pet struct {
	ID                int64              `json:"id,omitempty"`
	Category          *Category          `json:"category,omitempty"`
	Name              string             `json:"name"`
	PhotoURLs         []string           `json:"photoUrls"`
	Tags              []Tag              `json:"tags,omitempty"`
	Status            string             `json:"status,omitempty"`
	HairLengthCm      *float64           `json:"hairLengthCm,omitempty"`
	ExternalReference *ExternalReference `json:"externalReference,omitempty"`
	CreatedAt         time.Time          `json:"createdAt,omitempty"`
	UpdatedAt         time.Time          `json:"updatedAt,omitempty"`
}

// ToDomainPet maps a transport Pet into the domain aggregate.
func ToDomainPet(input Pet) (*domain.Pet, error) {
	pet, err := domain.NewPet(input.ID, input.Name, input.PhotoURLs)
	if err != nil {
		return nil, err
	}
	if input.Category != nil {
		cat := domain.Category{ID: input.Category.ID, Name: input.Category.Name}
		pet.UpdateCategory(&cat)
	}
	var tags []domain.Tag
	for _, t := range input.Tags {
		tags = append(tags, domain.Tag{ID: t.ID, Name: t.Name})
	}
	pet.ReplaceTags(tags)
	pet.UpdateStatus(domain.Status(input.Status))
	if input.HairLengthCm != nil {
		if err := pet.UpdateHairLength(*input.HairLengthCm); err != nil {
			return nil, err
		}
	}
	if input.ExternalReference != nil {
		pet.UpdateExternalReference(&domain.ExternalReference{
			Provider:   input.ExternalReference.Provider,
			ID:         input.ExternalReference.ID,
			Attributes: CloneAttributes(input.ExternalReference.Attributes),
		})
	}
	return pet, nil
}

// FromDomainPet maps a domain aggregate into a transport Pet.
func FromDomainPet(p *domain.Pet) Pet {
	var cat *Category
	if p.Category != nil {
		copy := *p.Category
		cat = &Category{ID: copy.ID, Name: copy.Name}
	}
	var tags []Tag
	for _, t := range p.Tags {
		tags = append(tags, Tag{ID: t.ID, Name: t.Name})
	}
	var hair *float64
	if p.HairLengthCm > 0 {
		value := p.HairLengthCm
		hair = &value
	}
	var external *ExternalReference
	if p.ExternalRef != nil {
		external = &ExternalReference{
			Provider:   p.ExternalRef.Provider,
			ID:         p.ExternalRef.ID,
			Attributes: CloneAttributes(p.ExternalRef.Attributes),
		}
	}
	return Pet{
		ID:                p.ID,
		Category:          cat,
		Name:              p.Name,
		PhotoURLs:         append([]string{}, p.PhotoURLs...),
		Tags:              tags,
		Status:            string(p.Status),
		HairLengthCm:      hair,
		ExternalReference: external,
	}
}

// ToMutationInput converts a mutation payload into an application mutation input while preserving field presence.
func ToMutationInput(model MutationPet) petstypes.PetMutationInput {
	input := petstypes.PetMutationInput{ID: model.ID}
	if model.Name != nil {
		name := *model.Name
		input.Name = &name
	}
	if model.PhotoURLs != nil {
		urls := append([]string{}, (*model.PhotoURLs)...)
		input.PhotoURLs = &urls
	}
	if model.Category != nil {
		input.Category = &petstypes.CategoryInput{ID: model.Category.ID, Name: model.Category.Name}
	}
	if model.Tags != nil {
		tags := make([]petstypes.TagInput, 0, len(*model.Tags))
		for _, tag := range *model.Tags {
			tags = append(tags, petstypes.TagInput{ID: tag.ID, Name: tag.Name})
		}
		input.Tags = &tags
	}
	if model.Status != nil {
		status := *model.Status
		input.Status = &status
	}
	if model.HairLengthCm != nil {
		input.HairLengthCm = ClonePointer(model.HairLengthCm)
	}
	if model.ExternalReference != nil {
		input.ExternalReference = ToExternalReferenceInput(model.ExternalReference)
	}
	return input
}

// ToExternalReferenceInput converts transport external reference to application input.
func ToExternalReferenceInput(ref *ExternalReference) *petstypes.ExternalReferenceInput {
	if ref == nil {
		return nil
	}
	return &petstypes.ExternalReferenceInput{
		Provider:   ref.Provider,
		ID:         ref.ID,
		Attributes: CloneAttributes(ref.Attributes),
	}
}

// FromExternalReference maps domain reference to transport representation.
func FromExternalReference(ref *domain.ExternalReference) *ExternalReference {
	if ref == nil {
		return nil
	}
	return &ExternalReference{
		Provider:   ref.Provider,
		ID:         ref.ID,
		Attributes: CloneAttributes(ref.Attributes),
	}
}

// ClonePointer duplicates a float pointer to avoid aliasing mutable transport values.
func ClonePointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// FromProjection maps a projection into a transport pet enriched with metadata.
func FromProjection(projection *petstypes.PetProjection) Pet {
	pet := FromDomainPet(projection.Pet)
	pet.CreatedAt = projection.Metadata.CreatedAt
	pet.UpdatedAt = projection.Metadata.UpdatedAt
	return pet
}

// FromProjectionList maps a slice of projections into transport pets with metadata.
func FromProjectionList(list []*petstypes.PetProjection) []Pet {
	result := make([]Pet, 0, len(list))
	for _, projection := range list {
		result = append(result, FromProjection(projection))
	}
	return result
}

// FromDomainPetList maps a slice of domain aggregates to transport Pets.
func FromDomainPetList(list []*domain.Pet) []Pet {
	resp := make([]Pet, 0, len(list))
	for _, p := range list {
		resp = append(resp, FromDomainPet(p))
	}
	return resp
}

// ToGroomPetInput validates the grooming payload and produces the application DTO.
func ToGroomPetInput(id int64, input GroomingOperation) (petstypes.GroomPetInput, error) {
	if input.InitialHairLengthCm == nil {
		return petstypes.GroomPetInput{}, errMissingInitial
	}
	if input.TrimByCm == nil {
		return petstypes.GroomPetInput{}, errMissingTrim
	}
	return petstypes.GroomPetInput{ID: id, InitialHairLengthCm: *input.InitialHairLengthCm, TrimByCm: *input.TrimByCm}, nil
}

// CloneAttributes duplicates the attribute map to prevent shared references.
func CloneAttributes(attrs map[string]string) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	copy := make(map[string]string, len(attrs))
	for k, v := range attrs {
		copy[k] = v
	}
	return copy
}
