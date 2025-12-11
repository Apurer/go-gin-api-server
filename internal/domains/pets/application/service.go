package application

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/types"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/ports"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/service"
	meterName  = "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/service"
)

// Service orchestrates the pets bounded context use cases.
type Service struct {
	repo    ports.Repository
	tracer  trace.Tracer
	logger  *slog.Logger
	metrics serviceMetrics
}

// ServiceOption configures instrumentation for the service.
type ServiceOption func(*Service)

// WithLogger injects a slog logger.
func WithLogger(logger *slog.Logger) ServiceOption {
	return func(s *Service) {
		s.logger = logger
	}
}

// WithTracer injects a tracer implementation.
func WithTracer(tr trace.Tracer) ServiceOption {
	return func(s *Service) {
		s.tracer = tr
	}
}

// WithMeter injects the meter used to create service metrics instruments.
func WithMeter(m metric.Meter) ServiceOption {
	return func(s *Service) {
		s.metrics = newServiceMetrics(m)
	}
}

type serviceMetrics struct {
	petsCreated metric.Int64Counter
	petsUpdated metric.Int64Counter
	petsDeleted metric.Int64Counter
	petsGroomed metric.Int64Counter
}

// NewService wires the pets service with its dependencies.
func NewService(repo ports.Repository, opts ...ServiceOption) *Service {
	s := &Service{
		repo:    repo,
		tracer:  otel.Tracer(tracerName),
		logger:  defaultLogger(),
		metrics: newServiceMetrics(otel.GetMeterProvider().Meter(meterName)),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.tracer == nil {
		s.tracer = otel.Tracer(tracerName)
	}
	if s.logger == nil {
		s.logger = defaultLogger()
	}
	if s.metrics == (serviceMetrics{}) {
		s.metrics = newServiceMetrics(otel.GetMeterProvider().Meter(meterName))
	}
	return s
}

// AddPet persists a new pet aggregate.
func (s *Service) AddPet(ctx context.Context, input types.AddPetInput) (*types.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.AddPet", attribute.Int64("pet.id", input.ID))
	defer span.End()

	s.logInfo(ctx, "adding pet", slog.Int64("pet.id", input.ID))
	pet, err := buildPetFromMutation(input.PetMutationInput)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to build pet", slog.Int64("pet.id", input.ID))
	}
	saved, err := s.repo.Save(ctx, pet)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to persist pet", slog.Int64("pet.id", input.ID))
	}
	s.metrics.recordCreated(ctx, saved.Entity.Status)
	s.logInfo(ctx, "pet added", slog.Int64("pet.id", saved.Entity.ID), slog.String("status", string(saved.Entity.Status)))
	return types.FromDomainProjection(saved), nil
}

// UpdatePet overrides an existing pet with new state.
func (s *Service) UpdatePet(ctx context.Context, input types.UpdatePetInput) (*types.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.UpdatePet", attribute.Int64("pet.id", input.ID))
	defer span.End()

	s.logInfo(ctx, "updating pet", slog.Int64("pet.id", input.ID))
	projection, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to load pet", slog.Int64("pet.id", input.ID))
	}
	if err := applyPartialMutation(projection.Entity, input.PetMutationInput); err != nil {
		return nil, s.handleError(ctx, span, err, "failed to apply mutation", slog.Int64("pet.id", input.ID))
	}
	saved, err := s.repo.Save(ctx, projection.Entity)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to persist pet", slog.Int64("pet.id", input.ID))
	}
	s.metrics.recordUpdated(ctx, saved.Entity.Status)
	s.logInfo(ctx, "pet updated", slog.Int64("pet.id", saved.Entity.ID), slog.String("status", string(saved.Entity.Status)))
	return types.FromDomainProjection(saved), nil
}

// UpdatePetWithForm handles the simplified form flow.
func (s *Service) UpdatePetWithForm(ctx context.Context, input types.UpdatePetWithFormInput) (*types.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.UpdatePetWithForm", attribute.Int64("pet.id", input.ID))
	defer span.End()

	s.logInfo(ctx, "updating pet via form", slog.Int64("pet.id", input.ID))
	projection, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to load pet", slog.Int64("pet.id", input.ID))
	}
	existing := projection.Entity
	if input.Name != nil && *input.Name != "" {
		_ = existing.Rename(*input.Name)
	}
	if input.Status != nil && *input.Status != "" {
		existing.UpdateStatus(domain.Status(*input.Status))
	}
	saved, err := s.repo.Save(ctx, existing)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to persist pet", slog.Int64("pet.id", input.ID))
	}
	s.metrics.recordUpdated(ctx, saved.Entity.Status)
	s.logInfo(ctx, "pet updated via form", slog.Int64("pet.id", saved.Entity.ID), slog.String("status", string(saved.Entity.Status)))
	return types.FromDomainProjection(saved), nil
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
	stringStatuses := statusesToStrings(statuses)
	ctx, span := s.startSpan(ctx, "Service.FindByStatus", attribute.StringSlice("pet.statuses.requested", stringStatuses))
	defer span.End()

	s.logInfo(ctx, "finding pets by status", slog.Any("statuses", stringStatuses))
	result, err := s.repo.FindByStatus(ctx, statuses)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to find pets by status", slog.Any("statuses", stringStatuses))
	}
	span.SetAttributes(attribute.Int("pet.result.count", len(result)))
	projections := types.FromDomainProjectionList(result)
	s.logInfo(ctx, "found pets by status", slog.Int("count", len(projections)))
	return projections, nil
}

// FindByTags searches pets matching any supplied tag name.
func (s *Service) FindByTags(ctx context.Context, input types.FindPetsByTagsInput) ([]*types.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.FindByTags", attribute.StringSlice("pet.tags.requested", input.Tags))
	defer span.End()

	s.logInfo(ctx, "finding pets by tags", slog.Any("tags", input.Tags))
	result, err := s.repo.FindByTags(ctx, input.Tags)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to find pets by tags", slog.Any("tags", input.Tags))
	}
	span.SetAttributes(attribute.Int("pet.result.count", len(result)))
	projections := types.FromDomainProjectionList(result)
	s.logInfo(ctx, "found pets by tags", slog.Int("count", len(projections)))
	return projections, nil
}

// GetByID loads a single pet aggregate.
func (s *Service) GetByID(ctx context.Context, input types.PetIdentifier) (*types.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.GetByID", attribute.Int64("pet.id", input.ID))
	defer span.End()

	s.logInfo(ctx, "loading pet", slog.Int64("pet.id", input.ID))
	projection, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to load pet", slog.Int64("pet.id", input.ID))
	}
	s.logInfo(ctx, "pet loaded", slog.Int64("pet.id", projection.Entity.ID), slog.String("status", string(projection.Entity.Status)))
	return types.FromDomainProjection(projection), nil
}

// Delete removes a pet.
func (s *Service) Delete(ctx context.Context, input types.PetIdentifier) error {
	ctx, span := s.startSpan(ctx, "Service.Delete", attribute.Int64("pet.id", input.ID))
	defer span.End()

	s.logInfo(ctx, "deleting pet", slog.Int64("pet.id", input.ID))
	if err := s.repo.Delete(ctx, input.ID); err != nil {
		return s.handleError(ctx, span, err, "failed to delete pet", slog.Int64("pet.id", input.ID))
	}
	s.metrics.recordDeleted(ctx)
	s.logInfo(ctx, "pet deleted", slog.Int64("pet.id", input.ID))
	return nil
}

// GroomPet applies a transient grooming operation and persists the resulting hair length.
func (s *Service) GroomPet(ctx context.Context, input types.GroomPetInput) (*types.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.GroomPet",
		attribute.Int64("pet.id", input.ID),
		attribute.Float64("pet.groom.initial_length_cm", input.InitialHairLengthCm),
		attribute.Float64("pet.groom.trim_by_cm", input.TrimByCm),
	)
	defer span.End()

	s.logInfo(ctx, "grooming pet", slog.Int64("pet.id", input.ID))
	projection, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to load pet", slog.Int64("pet.id", input.ID))
	}
	op := domain.GroomingOperation{InitialLengthCm: input.InitialHairLengthCm, TrimByCm: input.TrimByCm}
	if err := projection.Entity.Groom(op); err != nil {
		return nil, s.handleError(ctx, span, err, "failed to groom pet", slog.Int64("pet.id", input.ID))
	}
	saved, err := s.repo.Save(ctx, projection.Entity)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to persist groomed pet", slog.Int64("pet.id", input.ID))
	}
	s.metrics.recordGroomed(ctx)
	s.logInfo(ctx, "pet groomed", slog.Int64("pet.id", saved.Entity.ID), slog.Float64("pet.hair_length_cm", saved.Entity.HairLengthCm))
	return types.FromDomainProjection(saved), nil
}

// UploadImageResult describes the metadata returned by the upload flow.
type UploadImageResult struct {
	Code    int32
	Type    string
	Message string
}

// UploadImage stores metadata about an uploaded asset. For demo it simply tracks message.
func (s *Service) UploadImage(ctx context.Context, input types.UploadImageInput) (*UploadImageResult, error) {
	ctx, span := s.startSpan(ctx, "Service.UploadImage",
		attribute.Int64("pet.id", input.ID),
		attribute.String("asset.filename", input.Filename),
	)
	defer span.End()

	s.logInfo(ctx, "upload image metadata", slog.Int64("pet.id", input.ID), slog.String("filename", input.Filename))
	if _, err := s.repo.GetByID(ctx, input.ID); err != nil {
		return nil, s.handleError(ctx, span, err, "failed to load pet before upload", slog.Int64("pet.id", input.ID))
	}
	msg := fmt.Sprintf("image '%s' stored for pet %d", input.Filename, input.ID)
	if input.Metadata != "" {
		msg = fmt.Sprintf("%s (%s)", msg, input.Metadata)
		span.SetAttributes(attribute.String("asset.metadata", input.Metadata))
	}
	return &UploadImageResult{Code: 200, Type: "upload", Message: msg}, nil
}

// List exposes all pets for inventory calculations or admin use cases.
func (s *Service) List(ctx context.Context) ([]*types.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.List")
	defer span.End()

	s.logInfo(ctx, "listing pets")
	result, err := s.repo.List(ctx)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to list pets")
	}
	span.SetAttributes(attribute.Int("pet.result.count", len(result)))
	projections := types.FromDomainProjectionList(result)
	s.logInfo(ctx, "listed pets", slog.Int("count", len(projections)))
	return projections, nil
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newServiceMetrics(m metric.Meter) serviceMetrics {
	if m == nil {
		return serviceMetrics{}
	}
	petsCreated, _ := m.Int64Counter("pets.service.created", metric.WithDescription("Number of pets created"))
	petsUpdated, _ := m.Int64Counter("pets.service.updated", metric.WithDescription("Number of pets updated"))
	petsDeleted, _ := m.Int64Counter("pets.service.deleted", metric.WithDescription("Number of pets deleted"))
	petsGroomed, _ := m.Int64Counter("pets.service.groomed", metric.WithDescription("Number of grooming operations"))
	return serviceMetrics{
		petsCreated: petsCreated,
		petsUpdated: petsUpdated,
		petsDeleted: petsDeleted,
		petsGroomed: petsGroomed,
	}
}

func (m serviceMetrics) recordCreated(ctx context.Context, status domain.Status) {
	addCounter(ctx, m.petsCreated, 1, attribute.String("pet.status", string(status)))
}

func (m serviceMetrics) recordUpdated(ctx context.Context, status domain.Status) {
	addCounter(ctx, m.petsUpdated, 1, attribute.String("pet.status", string(status)))
}

func (m serviceMetrics) recordDeleted(ctx context.Context) {
	addCounter(ctx, m.petsDeleted, 1)
}

func (m serviceMetrics) recordGroomed(ctx context.Context) {
	addCounter(ctx, m.petsGroomed, 1)
}

func addCounter(ctx context.Context, counter metric.Int64Counter, value int64, attrs ...attribute.KeyValue) {
	if counter == nil {
		return
	}
	counter.Add(ctx, value, metric.WithAttributes(attrs...))
}

func statusesToStrings(statuses []domain.Status) []string {
	result := make([]string, 0, len(statuses))
	for _, status := range statuses {
		result = append(result, string(status))
	}
	return result
}

func (s *Service) startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := s.tracer
	if tracer == nil {
		tracer = otel.Tracer(tracerName)
	}
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func (s *Service) logInfo(ctx context.Context, msg string, attrs ...slog.Attr) {
	if s.logger == nil {
		return
	}
	s.logger.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

func (s *Service) logError(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	if s.logger == nil {
		return
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	s.logger.LogAttrs(ctx, slog.LevelError, msg, attrs...)
}

func (s *Service) handleError(ctx context.Context, span trace.Span, err error, msg string, attrs ...slog.Attr) error {
	if err == nil {
		return nil
	}
	if span != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	s.logError(ctx, msg, err, attrs...)
	return err
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

var _ Port = (*Service)(nil)
