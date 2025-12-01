package workflows

import (
	"context"
	"errors"
	"fmt"
	"time"

	oteltrace "go.opentelemetry.io/otel/trace"
	"go.temporal.io/sdk/client"

	petworkflows "github.com/GIT_USER_ID/GIT_REPO_ID/internal/durable/temporal/workflows/pets"
	petsapp "github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/application"
	petstypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/application/types"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/ports"
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
	options := client.StartWorkflowOptions{
		ID:        buildPetCreationWorkflowID(input, traceComponent),
		TaskQueue: o.taskQueue,
	}
	run, err := o.client.ExecuteWorkflow(
		ctx,
		options,
		petworkflows.PetCreationWorkflow,
		petworkflows.PetCreationWorkflowInput{Command: input, TraceID: traceComponent},
	)
	if err != nil {
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
	service *petsapp.Service
}

// NewInlinePetWorkflows wraps the pets service for synchronous execution.
func NewInlinePetWorkflows(service *petsapp.Service) *InlinePetWorkflows {
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
	idComponent := input.PetMutationInput.ID
	if idComponent == 0 {
		idComponent = time.Now().UnixNano()
	}
	return fmt.Sprintf("pet-creation-%d-%s", idComponent, traceComponent)
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
