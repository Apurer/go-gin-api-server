package application

import (
	"context"
	"fmt"

	types "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/types"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/ports"
)

// Service orchestrates the pets bounded context use cases.
type Service struct {
	repo ports.Repository
}

// NewService wires the pets service with its dependencies.
func NewService(repo ports.Repository) *Service {
	return &Service{repo: repo}
}

// AddPet persists a new pet aggregate.
func (s *Service) AddPet(ctx context.Context, input types.AddPetInput) (*types.PetProjection, error) {
	pet, err := buildPetFromMutation(input.PetMutationInput)
	if err != nil {
		return nil, mapError(err)
	}
	saved, err := s.repo.Save(ctx, pet)
	if err != nil {
		return nil, mapError(err)
	}
	return saved, nil
}

// UpdatePet overrides an existing pet with new state.
func (s *Service) UpdatePet(ctx context.Context, input types.UpdatePetInput) (*types.PetProjection, error) {
	projection, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := applyPartialMutation(projection.Pet, input.PetMutationInput); err != nil {
		return nil, mapError(err)
	}
	saved, err := s.repo.Save(ctx, projection.Pet)
	if err != nil {
		return nil, mapError(err)
	}
	return saved, nil
}

// UpdatePetWithForm handles the simplified form flow.
func (s *Service) UpdatePetWithForm(ctx context.Context, input types.UpdatePetWithFormInput) (*types.PetProjection, error) {
	projection, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, mapError(err)
	}
	existing := projection.Pet
	if input.Name != nil && *input.Name != "" {
		_ = existing.Rename(*input.Name)
	}
	if input.Status != nil && *input.Status != "" {
		existing.UpdateStatus(domain.Status(*input.Status))
	}
	saved, err := s.repo.Save(ctx, existing)
	if err != nil {
		return nil, mapError(err)
	}
	return saved, nil
}

// FindByStatus searches pets matching any of the provided statuses.
func (s *Service) FindByStatus(ctx context.Context, input types.FindPetsByStatusInput) ([]*types.PetProjection, error) {
	statuses := make([]domain.Status, 0, len(input.Statuses))
	for _, status := range input.Statuses {
		statuses = append(statuses, domain.Status(status))
	}
	if len(statuses) == 0 {
		statuses = []domain.Status{domain.StatusAvailable}
	}
	result, err := s.repo.FindByStatus(ctx, statuses)
	if err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

// FindByTags searches pets matching any supplied tag name.
func (s *Service) FindByTags(ctx context.Context, input types.FindPetsByTagsInput) ([]*types.PetProjection, error) {
	result, err := s.repo.FindByTags(ctx, input.Tags)
	if err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

// GetByID loads a single pet aggregate.
func (s *Service) GetByID(ctx context.Context, input types.PetIdentifier) (*types.PetProjection, error) {
	projection, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, mapError(err)
	}
	return projection, nil
}

// Delete removes a pet.
func (s *Service) Delete(ctx context.Context, input types.PetIdentifier) error {
	if err := s.repo.Delete(ctx, input.ID); err != nil {
		return mapError(err)
	}
	return nil
}

// GroomPet applies a transient grooming operation and persists the resulting hair length.
func (s *Service) GroomPet(ctx context.Context, input types.GroomPetInput) (*types.PetProjection, error) {
	projection, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, mapError(err)
	}
	op := domain.GroomingOperation{InitialLengthCm: input.InitialHairLengthCm, TrimByCm: input.TrimByCm}
	if err := projection.Pet.Groom(op); err != nil {
		return nil, mapError(err)
	}
	saved, err := s.repo.Save(ctx, projection.Pet)
	if err != nil {
		return nil, mapError(err)
	}
	return saved, nil
}

// UploadImage stores metadata about an uploaded asset. For demo it simply tracks message.
func (s *Service) UploadImage(ctx context.Context, input types.UploadImageInput) (*ports.UploadImageResult, error) {
	if _, err := s.repo.GetByID(ctx, input.ID); err != nil {
		return nil, mapError(err)
	}
	msg := fmt.Sprintf("image '%s' stored for pet %d", input.Filename, input.ID)
	if input.Metadata != "" {
		msg = fmt.Sprintf("%s (%s)", msg, input.Metadata)
	}
	return &ports.UploadImageResult{Code: 200, Type: "upload", Message: msg}, nil
}

// List exposes all pets for inventory calculations or admin use cases.
func (s *Service) List(ctx context.Context) ([]*types.PetProjection, error) {
	result, err := s.repo.List(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func buildPetFromMutation(input types.PetMutationInput) (*domain.Pet, error) {
	if input.Name == nil {
		return nil, domain.ErrEmptyName
	}
	if input.PhotoURLs == nil {
		return nil, domain.ErrEmptyPhotos
	}
	photos := append([]string{}, (*input.PhotoURLs)...)
	pet, err := domain.NewPet(input.ID, *input.Name, photos)
	if err != nil {
		return nil, err
	}
	if input.Status != nil {
		pet.UpdateStatus(domain.Status(*input.Status))
	} else {
		pet.UpdateStatus("")
	}
	partial := input
	partial.Name = nil
	partial.PhotoURLs = nil
	partial.Status = nil
	if err := applyPartialMutation(pet, partial); err != nil {
		return nil, err
	}
	return pet, nil
}

func applyPartialMutation(target *domain.Pet, input types.PetMutationInput) error {
	if input.Name != nil {
		if err := target.Rename(*input.Name); err != nil {
			return err
		}
	}
	if input.PhotoURLs != nil {
		if err := target.ReplacePhotos(*input.PhotoURLs); err != nil {
			return err
		}
	}
	if input.Category != nil {
		if input.Category.ID == 0 && input.Category.Name == "" {
			target.UpdateCategory(nil)
		} else {
			cat := domain.Category{ID: input.Category.ID, Name: input.Category.Name}
			target.UpdateCategory(&cat)
		}
	}
	if input.Tags != nil {
		tags := make([]domain.Tag, 0, len(*input.Tags))
		for _, t := range *input.Tags {
			tags = append(tags, domain.Tag{ID: t.ID, Name: t.Name})
		}
		target.ReplaceTags(tags)
	}
	if input.Status != nil {
		target.UpdateStatus(domain.Status(*input.Status))
	}
	if input.HairLengthCm != nil {
		if err := target.UpdateHairLength(*input.HairLengthCm); err != nil {
			return err
		}
	}
	if input.ExternalReference != nil {
		if input.ExternalReference.Provider == "" && input.ExternalReference.ID == "" && len(input.ExternalReference.Attributes) == 0 {
			target.UpdateExternalReference(nil)
		} else {
			target.UpdateExternalReference(&domain.ExternalReference{
				Provider:   input.ExternalReference.Provider,
				ID:         input.ExternalReference.ID,
				Attributes: cloneAttributes(input.ExternalReference.Attributes),
			})
		}
	}
	return nil
}

func cloneAttributes(attrs map[string]string) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	copy := make(map[string]string, len(attrs))
	for k, v := range attrs {
		copy[k] = v
	}
	return copy
}

var _ ports.Service = (*Service)(nil)
