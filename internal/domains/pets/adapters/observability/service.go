package observability

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	pettypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/types"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/ports"
)

const tracerName = "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/observability/service"

// Service decorates a pets application port with tracing, logging, and metrics.
type Service struct {
	inner   ports.Service
	tracer  trace.Tracer
	logger  *slog.Logger
	metrics serviceMetrics
}

type Option func(*Service)

// WithLogger injects a slog logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

// WithTracer injects a tracer implementation.
func WithTracer(tr trace.Tracer) Option {
	return func(s *Service) {
		s.tracer = tr
	}
}

// WithMeter injects the meter used to create service metrics instruments.
func WithMeter(m metric.Meter) Option {
	return func(s *Service) {
		s.metrics = newServiceMetrics(m)
	}
}

// New wires a decorator around the core service.
func New(inner ports.Service, opts ...Option) ports.Service {
	s := &Service{
		inner:   inner,
		tracer:  nooptrace.NewTracerProvider().Tracer(tracerName),
		logger:  defaultLogger(),
		metrics: newServiceMetrics(nil),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.tracer == nil {
		s.tracer = nooptrace.NewTracerProvider().Tracer(tracerName)
	}
	if s.logger == nil {
		s.logger = defaultLogger()
	}
	return s
}

// AddPet persists a new pet aggregate with instrumentation.
func (s *Service) AddPet(ctx context.Context, input pettypes.AddPetInput) (*pettypes.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.AddPet", attribute.Int64("pet.id", input.ID))
	defer span.End()

	s.logInfo(ctx, "adding pet", slog.Int64("pet.id", input.ID))
	result, err := s.inner.AddPet(ctx, input)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to add pet", slog.Int64("pet.id", input.ID))
	}
	if result != nil && result.Pet != nil {
		s.metrics.recordCreated(ctx, result.Pet.Status)
		s.logInfo(ctx, "pet added", slog.Int64("pet.id", result.Pet.ID), slog.String("status", string(result.Pet.Status)))
	}
	return result, nil
}

// UpdatePet overrides an existing pet with new state.
func (s *Service) UpdatePet(ctx context.Context, input pettypes.UpdatePetInput) (*pettypes.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.UpdatePet", attribute.Int64("pet.id", input.ID))
	defer span.End()

	s.logInfo(ctx, "updating pet", slog.Int64("pet.id", input.ID))
	result, err := s.inner.UpdatePet(ctx, input)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to update pet", slog.Int64("pet.id", input.ID))
	}
	if result != nil && result.Pet != nil {
		s.metrics.recordUpdated(ctx, result.Pet.Status)
		s.logInfo(ctx, "pet updated", slog.Int64("pet.id", result.Pet.ID), slog.String("status", string(result.Pet.Status)))
	}
	return result, nil
}

// UpdatePetWithForm handles the simplified form flow.
func (s *Service) UpdatePetWithForm(ctx context.Context, input pettypes.UpdatePetWithFormInput) (*pettypes.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.UpdatePetWithForm", attribute.Int64("pet.id", input.ID))
	defer span.End()

	s.logInfo(ctx, "updating pet via form", slog.Int64("pet.id", input.ID))
	result, err := s.inner.UpdatePetWithForm(ctx, input)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to update pet via form", slog.Int64("pet.id", input.ID))
	}
	if result != nil && result.Pet != nil {
		s.metrics.recordUpdated(ctx, result.Pet.Status)
		s.logInfo(ctx, "pet updated via form", slog.Int64("pet.id", result.Pet.ID), slog.String("status", string(result.Pet.Status)))
	}
	return result, nil
}

// FindByStatus searches pets matching any of the provided statuses.
func (s *Service) FindByStatus(ctx context.Context, input pettypes.FindPetsByStatusInput) ([]*pettypes.PetProjection, error) {
	statuses := attribute.StringSlice("pet.statuses.requested", input.Statuses)
	ctx, span := s.startSpan(ctx, "Service.FindByStatus", statuses)
	defer span.End()

	s.logInfo(ctx, "finding pets by status", slog.Any("statuses", input.Statuses))
	result, err := s.inner.FindByStatus(ctx, input)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to find pets by status", slog.Any("statuses", input.Statuses))
	}
	span.SetAttributes(attribute.Int("pet.result.count", len(result)))
	s.logInfo(ctx, "found pets by status", slog.Int("count", len(result)))
	return result, nil
}

// FindByTags searches pets matching any supplied tag name.
func (s *Service) FindByTags(ctx context.Context, input pettypes.FindPetsByTagsInput) ([]*pettypes.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.FindByTags", attribute.StringSlice("pet.tags.requested", input.Tags))
	defer span.End()

	s.logInfo(ctx, "finding pets by tags", slog.Any("tags", input.Tags))
	result, err := s.inner.FindByTags(ctx, input)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to find pets by tags", slog.Any("tags", input.Tags))
	}
	span.SetAttributes(attribute.Int("pet.result.count", len(result)))
	s.logInfo(ctx, "found pets by tags", slog.Int("count", len(result)))
	return result, nil
}

// GetByID loads a single pet aggregate.
func (s *Service) GetByID(ctx context.Context, input pettypes.PetIdentifier) (*pettypes.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.GetByID", attribute.Int64("pet.id", input.ID))
	defer span.End()

	s.logInfo(ctx, "loading pet", slog.Int64("pet.id", input.ID))
	result, err := s.inner.GetByID(ctx, input)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to load pet", slog.Int64("pet.id", input.ID))
	}
	if result != nil && result.Pet != nil {
		s.logInfo(ctx, "pet loaded", slog.Int64("pet.id", result.Pet.ID), slog.String("status", string(result.Pet.Status)))
	}
	return result, nil
}

// Delete removes a pet.
func (s *Service) Delete(ctx context.Context, input pettypes.PetIdentifier) error {
	ctx, span := s.startSpan(ctx, "Service.Delete", attribute.Int64("pet.id", input.ID))
	defer span.End()

	s.logInfo(ctx, "deleting pet", slog.Int64("pet.id", input.ID))
	if err := s.inner.Delete(ctx, input); err != nil {
		return s.handleError(ctx, span, err, "failed to delete pet", slog.Int64("pet.id", input.ID))
	}
	s.metrics.recordDeleted(ctx)
	s.logInfo(ctx, "pet deleted", slog.Int64("pet.id", input.ID))
	return nil
}

// GroomPet applies a transient grooming operation and persists the resulting hair length.
func (s *Service) GroomPet(ctx context.Context, input pettypes.GroomPetInput) (*pettypes.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.GroomPet",
		attribute.Int64("pet.id", input.ID),
		attribute.Float64("pet.groom.initial_length_cm", input.InitialHairLengthCm),
		attribute.Float64("pet.groom.trim_by_cm", input.TrimByCm),
	)
	defer span.End()

	s.logInfo(ctx, "grooming pet", slog.Int64("pet.id", input.ID))
	result, err := s.inner.GroomPet(ctx, input)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to groom pet", slog.Int64("pet.id", input.ID))
	}
	if result != nil && result.Pet != nil {
		s.metrics.recordGroomed(ctx)
		s.logInfo(ctx, "pet groomed", slog.Int64("pet.id", result.Pet.ID), slog.Float64("pet.hair_length_cm", result.Pet.HairLengthCm))
	}
	return result, nil
}

// UploadImage stores metadata about an uploaded asset.
func (s *Service) UploadImage(ctx context.Context, input pettypes.UploadImageInput) (*ports.UploadImageResult, error) {
	ctx, span := s.startSpan(ctx, "Service.UploadImage",
		attribute.Int64("pet.id", input.ID),
		attribute.String("asset.filename", input.Filename),
	)
	defer span.End()

	s.logInfo(ctx, "upload image metadata", slog.Int64("pet.id", input.ID), slog.String("filename", input.Filename))
	result, err := s.inner.UploadImage(ctx, input)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to upload image", slog.Int64("pet.id", input.ID))
	}
	if input.Metadata != "" {
		span.SetAttributes(attribute.String("asset.metadata", input.Metadata))
	}
	return result, nil
}

// List exposes all pets for inventory calculations or admin use cases.
func (s *Service) List(ctx context.Context) ([]*pettypes.PetProjection, error) {
	ctx, span := s.startSpan(ctx, "Service.List")
	defer span.End()

	s.logInfo(ctx, "listing pets")
	result, err := s.inner.List(ctx)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to list pets")
	}
	span.SetAttributes(attribute.Int("pet.result.count", len(result)))
	s.logInfo(ctx, "listed pets", slog.Int("count", len(result)))
	return result, nil
}

func (s *Service) startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := s.tracer
	if tracer == nil {
		tracer = nooptrace.NewTracerProvider().Tracer(tracerName)
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

func defaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type serviceMetrics struct {
	petsCreated metric.Int64Counter
	petsUpdated metric.Int64Counter
	petsDeleted metric.Int64Counter
	petsGroomed metric.Int64Counter
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

var _ ports.Service = (*Service)(nil)
