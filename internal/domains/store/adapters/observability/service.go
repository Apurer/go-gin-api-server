package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	storedomain "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/domain"
	storeports "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/ports"
)

const tracerName = "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/adapters/observability/service"

// Service decorates the store service with tracing, logging, and metrics.
type Service struct {
	inner   storeports.Service
	tracer  trace.Tracer
	logger  *slog.Logger
	metrics serviceMetrics
}

type Option func(*Service)

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

func WithTracer(tr trace.Tracer) Option {
	return func(s *Service) {
		s.tracer = tr
	}
}

func WithMeter(m metric.Meter) Option {
	return func(s *Service) {
		s.metrics = newServiceMetrics(m)
	}
}

// New wraps the core store service.
func New(inner storeports.Service, opts ...Option) storeports.Service {
	s := &Service{
		inner:   inner,
		tracer:  nooptrace.NewTracerProvider().Tracer(tracerName),
		logger:  slog.New(slog.NewTextHandler(nil, nil)),
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
	return s
}

func (s *Service) PlaceOrder(ctx context.Context, order *storedomain.Order) (*storedomain.Order, error) {
	ctx, span := s.tracer.Start(ctx, "StoreService.PlaceOrder",
		trace.WithAttributes(attribute.Int64("order.id", order.ID), attribute.Int64("order.pet_id", order.PetID)))
	defer span.End()

	s.logInfo(ctx, "placing order", slog.Int64("order.id", order.ID), slog.Int64("order.pet_id", order.PetID))
	result, err := s.inner.PlaceOrder(ctx, order)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to place order", slog.Int64("order.id", order.ID))
	}
	s.metrics.recordPlaced(ctx, result.Status)
	s.logInfo(ctx, "order placed", slog.Int64("order.id", result.ID), slog.String("status", string(result.Status)))
	return result, nil
}

func (s *Service) GetOrderByID(ctx context.Context, id int64) (*storedomain.Order, error) {
	ctx, span := s.tracer.Start(ctx, "StoreService.GetOrderByID", trace.WithAttributes(attribute.Int64("order.id", id)))
	defer span.End()

	s.logInfo(ctx, "loading order", slog.Int64("order.id", id))
	result, err := s.inner.GetOrderByID(ctx, id)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to load order", slog.Int64("order.id", id))
	}
	s.logInfo(ctx, "order loaded", slog.Int64("order.id", result.ID), slog.String("status", string(result.Status)))
	return result, nil
}

func (s *Service) DeleteOrder(ctx context.Context, id int64) error {
	ctx, span := s.tracer.Start(ctx, "StoreService.DeleteOrder", trace.WithAttributes(attribute.Int64("order.id", id)))
	defer span.End()

	s.logInfo(ctx, "deleting order", slog.Int64("order.id", id))
	if err := s.inner.DeleteOrder(ctx, id); err != nil {
		return s.handleError(ctx, span, err, "failed to delete order", slog.Int64("order.id", id))
	}
	s.metrics.recordDeleted(ctx)
	s.logInfo(ctx, "order deleted", slog.Int64("order.id", id))
	return nil
}

func (s *Service) Inventory(ctx context.Context) (map[string]int32, error) {
	ctx, span := s.tracer.Start(ctx, "StoreService.Inventory")
	defer span.End()

	s.logInfo(ctx, "calculating inventory")
	result, err := s.inner.Inventory(ctx)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to calculate inventory")
	}
	span.SetAttributes(attribute.Int("inventory.status.count", len(result)))
	return result, nil
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
	if span != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	s.logError(ctx, msg, err, attrs...)
	return err
}

type serviceMetrics struct {
	ordersPlaced  metric.Int64Counter
	ordersDeleted metric.Int64Counter
}

func newServiceMetrics(m metric.Meter) serviceMetrics {
	if m == nil {
		return serviceMetrics{}
	}
	ordersPlaced, _ := m.Int64Counter("store.service.orders_placed", metric.WithDescription("Number of orders placed"))
	ordersDeleted, _ := m.Int64Counter("store.service.orders_deleted", metric.WithDescription("Number of orders deleted"))
	return serviceMetrics{ordersPlaced: ordersPlaced, ordersDeleted: ordersDeleted}
}

func (m serviceMetrics) recordPlaced(ctx context.Context, status storedomain.Status) {
	if m.ordersPlaced != nil {
		m.ordersPlaced.Add(ctx, 1, metric.WithAttributes(attribute.String("order.status", string(status))))
	}
}

func (m serviceMetrics) recordDeleted(ctx context.Context) {
	if m.ordersDeleted != nil {
		m.ordersDeleted.Add(ctx, 1)
	}
}

var _ storeports.Service = (*Service)(nil)
