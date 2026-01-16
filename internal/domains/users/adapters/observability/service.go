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

	userdomain "github.com/Apurer/go-gin-api-server/internal/domains/users/domain"
	userports "github.com/Apurer/go-gin-api-server/internal/domains/users/ports"
)

const tracerName = "github.com/Apurer/go-gin-api-server/internal/domains/users/adapters/observability/service"

// Service decorates the user service with tracing, logging, and metrics.
type Service struct {
	inner   userports.Service
	tracer  trace.Tracer
	logger  *slog.Logger
	metrics serviceMetrics
}

type Option func(*Service)

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) { s.logger = logger }
}

func WithTracer(tr trace.Tracer) Option {
	return func(s *Service) { s.tracer = tr }
}

func WithMeter(m metric.Meter) Option {
	return func(s *Service) { s.metrics = newServiceMetrics(m) }
}

// New wraps the core user service.
func New(inner userports.Service, opts ...Option) userports.Service {
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

func (s *Service) CreateUser(ctx context.Context, user *userdomain.User) (*userdomain.User, error) {
	ctx, span := s.tracer.Start(ctx, "UserService.CreateUser", trace.WithAttributes(attribute.String("user.username", user.Username)))
	defer span.End()
	s.logInfo(ctx, "creating user", slog.String("username", user.Username))
	result, err := s.inner.CreateUser(ctx, user)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to create user", slog.String("username", user.Username))
	}
	s.metrics.recordCreated(ctx)
	s.logInfo(ctx, "user created", slog.String("username", result.Username))
	return result, nil
}

func (s *Service) CreateUsers(ctx context.Context, users []*userdomain.User) ([]*userdomain.User, error) {
	ctx, span := s.tracer.Start(ctx, "UserService.CreateUsers", trace.WithAttributes(attribute.Int("user.batch.count", len(users))))
	defer span.End()
	result, err := s.inner.CreateUsers(ctx, users)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to create users")
	}
	s.metrics.recordCreated(ctx)
	return result, nil
}

func (s *Service) GetByUsername(ctx context.Context, username string) (*userdomain.User, error) {
	ctx, span := s.tracer.Start(ctx, "UserService.GetByUsername", trace.WithAttributes(attribute.String("user.username", username)))
	defer span.End()
	return s.inner.GetByUsername(ctx, username)
}

func (s *Service) Delete(ctx context.Context, username string) error {
	ctx, span := s.tracer.Start(ctx, "UserService.Delete", trace.WithAttributes(attribute.String("user.username", username)))
	defer span.End()
	if err := s.inner.Delete(ctx, username); err != nil {
		return s.handleError(ctx, span, err, "failed to delete user", slog.String("username", username))
	}
	s.metrics.recordDeleted(ctx)
	return nil
}

func (s *Service) Update(ctx context.Context, username string, updated *userdomain.User) (*userdomain.User, error) {
	ctx, span := s.tracer.Start(ctx, "UserService.Update", trace.WithAttributes(attribute.String("user.username", username)))
	defer span.End()
	result, err := s.inner.Update(ctx, username, updated)
	if err != nil {
		return nil, s.handleError(ctx, span, err, "failed to update user", slog.String("username", username))
	}
	s.metrics.recordUpdated(ctx)
	return result, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	ctx, span := s.tracer.Start(ctx, "UserService.Login", trace.WithAttributes(attribute.String("user.username", username)))
	defer span.End()
	token, err := s.inner.Login(ctx, username, password)
	if err != nil {
		return "", s.handleError(ctx, span, err, "login failed", slog.String("username", username))
	}
	s.metrics.recordLogin(ctx)
	return token, nil
}

func (s *Service) Logout(ctx context.Context, username string) {
	ctx, span := s.tracer.Start(ctx, "UserService.Logout", trace.WithAttributes(attribute.String("user.username", username)))
	defer span.End()
	s.inner.Logout(ctx, username)
}

func (s *Service) handleError(ctx context.Context, span trace.Span, err error, msg string, attrs ...slog.Attr) error {
	if span != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	s.logError(ctx, msg, err, attrs...)
	return err
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

type serviceMetrics struct {
	usersCreated metric.Int64Counter
	usersUpdated metric.Int64Counter
	usersDeleted metric.Int64Counter
	logins       metric.Int64Counter
}

func newServiceMetrics(m metric.Meter) serviceMetrics {
	if m == nil {
		return serviceMetrics{}
	}
	created, _ := m.Int64Counter("users.service.created", metric.WithDescription("Number of users created"))
	updated, _ := m.Int64Counter("users.service.updated", metric.WithDescription("Number of users updated"))
	deleted, _ := m.Int64Counter("users.service.deleted", metric.WithDescription("Number of users deleted"))
	logins, _ := m.Int64Counter("users.service.logins", metric.WithDescription("Number of successful logins"))
	return serviceMetrics{usersCreated: created, usersUpdated: updated, usersDeleted: deleted, logins: logins}
}

func (m serviceMetrics) recordCreated(ctx context.Context) {
	if m.usersCreated != nil {
		m.usersCreated.Add(ctx, 1)
	}
}

func (m serviceMetrics) recordUpdated(ctx context.Context) {
	if m.usersUpdated != nil {
		m.usersUpdated.Add(ctx, 1)
	}
}

func (m serviceMetrics) recordDeleted(ctx context.Context) {
	if m.usersDeleted != nil {
		m.usersDeleted.Add(ctx, 1)
	}
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (m serviceMetrics) recordLogin(ctx context.Context) {
	if m.logins != nil {
		m.logins.Add(ctx, 1)
	}
}

var _ userports.Service = (*Service)(nil)
