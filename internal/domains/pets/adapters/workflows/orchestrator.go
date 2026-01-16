package workflows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	oteltrace "go.opentelemetry.io/otel/trace"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	petstypes "github.com/Apurer/go-gin-api-server/internal/domains/pets/application/types"
	"github.com/Apurer/go-gin-api-server/internal/domains/pets/ports"
	petworkflows "github.com/Apurer/go-gin-api-server/internal/platform/temporal/workflows/pets"
)

var (
	_ ports.WorkflowOrchestrator = (*TemporalPetWorkflows)(nil)
	_ ports.WorkflowOrchestrator = (*InlinePetWorkflows)(nil)
)

// TemporalPetWorkflows starts pet workflows on a Temporal cluster.
type TemporalPetWorkflows struct {
	client    client.Client
	taskQueue string
}

// NewTemporalPetWorkflows wires a Temporal client into the orchestrator.
func NewTemporalPetWorkflows(c client.Client) *TemporalPetWorkflows {
	return &TemporalPetWorkflows{client: c, taskQueue: petworkflows.PetCreationTaskQueue}
}

// CreatePet starts the Temporal workflow that persists a pet aggregate.
func (o *TemporalPetWorkflows) CreatePet(ctx context.Context, input petstypes.AddPetInput) (*petstypes.PetProjection, error) {
	if o == nil || o.client == nil {
		return nil, errors.New("temporal pet workflows not configured")
	}
	traceComponent := workflowTraceComponent(ctx)
	workflowID := buildPetCreationWorkflowID(input, traceComponent)
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: o.taskQueue,
	}
	run, err := o.client.ExecuteWorkflow(
		ctx,
		options,
		petworkflows.PetCreationWorkflow,
		petworkflows.PetCreationWorkflowInput{Command: input, TraceID: traceComponent},
	)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) && strings.TrimSpace(input.IdempotencyKey) != "" {
			existingRun := o.client.GetWorkflow(ctx, workflowID, alreadyStarted.RunId)
			var projection petstypes.PetProjection
			if err := existingRun.Get(ctx, &projection); err != nil {
				return nil, err
			}
			return &projection, nil
		}
		return nil, err
	}
	var projection petstypes.PetProjection
	if err := run.Get(ctx, &projection); err != nil {
		return nil, err
	}
	return &projection, nil
}

// InlinePetWorkflows executes the service directly without Temporal, useful for tests or dev fallbacks.
type InlinePetWorkflows struct {
	service ports.Service
}

// NewInlinePetWorkflows wraps the pets service for synchronous execution.
func NewInlinePetWorkflows(service ports.Service) *InlinePetWorkflows {
	return &InlinePetWorkflows{service: service}
}

// CreatePet delegates to the application service without durable orchestration.
func (o *InlinePetWorkflows) CreatePet(ctx context.Context, input petstypes.AddPetInput) (*petstypes.PetProjection, error) {
	if o == nil || o.service == nil {
		return nil, errors.New("inline pet workflows not configured")
	}
	return o.service.AddPet(ctx, input)
}

func buildPetCreationWorkflowID(input petstypes.AddPetInput, traceComponent string) string {
	if key := strings.TrimSpace(input.IdempotencyKey); key != "" {
		return fmt.Sprintf("pet-creation-idem-%s", hashIdempotencyKey(key))
	}
	idComponent := input.PetMutationInput.ID
	if idComponent == 0 {
		idComponent = time.Now().UnixNano()
	}
	return fmt.Sprintf("pet-creation-%d-%s", idComponent, traceComponent)
}

func hashIdempotencyKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	// Use the first 16 hex chars to keep workflow IDs readable while remaining deterministic.
	return hex.EncodeToString(sum[:8])
}

func workflowTraceComponent(ctx context.Context) string {
	traceComponent := workflowTraceID(ctx)
	if traceComponent != "" {
		return traceComponent
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

func workflowTraceID(ctx context.Context) string {
	span := oteltrace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}
	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		return ""
	}
	traceID := spanCtx.TraceID()
	if !traceID.IsValid() {
		return ""
	}
	return traceID.String()
}
