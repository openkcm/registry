package service_test

import (
	"context"
	"testing"

	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/openkcm/registry/internal/service"
)

// installOTelForOrbital registers a recording TracerProvider + W3C propagator globally.
func installOTelForOrbital(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	origTP := otel.GetTracerProvider()
	origProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(origTP)
		otel.SetTextMapPropagator(origProp)
	})
	return sr
}

// TestBuildJobWithTrace_InjectsLabels verifies that PrepareJob stores the active span's
// traceparent/tracestate in the job labels (AC-03 enqueue side).
func TestBuildJobWithTrace_InjectsLabels(t *testing.T) {
	installOTelForOrbital(t)

	ctx, parentSpan := otel.Tracer("test").Start(t.Context(), "parent")
	defer parentSpan.End()

	job := service.BuildJobWithTraceForTest(ctx, []byte(`{}`), "ext-1", "my-job")

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	wantTraceparent := carrier.Get("traceparent")
	require.NotEmpty(t, wantTraceparent)

	assert.Equal(t, wantTraceparent, job.Labels["traceparent"],
		"traceparent label must match the active span")
}

// TestBuildJobWithTrace_NoLabels_WhenNoTrace verifies that no traceparent label is added
// when no span is active.
func TestBuildJobWithTrace_NoLabels_WhenNoTrace(t *testing.T) {
	installOTelForOrbital(t)

	job := service.BuildJobWithTraceForTest(context.Background(), []byte(`{}`), "ext-2", "my-job")

	assert.Empty(t, job.Labels, "labels must be empty when no span is active")
}

// TestResolveTasks_CreatesLinkedSpan verifies that resolveTasks starts a span linked to
// the traceparent stored in job labels (AC-03 processing side).
func TestResolveTasks_CreatesLinkedSpan(t *testing.T) {
	sr := installOTelForOrbital(t)

	// Build a valid traceparent from a real span.
	parentCtx, parentSpan := otel.Tracer("test").Start(context.Background(), "parent")
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(parentCtx, carrier)
	parentSpan.End()
	parentTraceparent := carrier.Get("traceparent")
	require.NotEmpty(t, parentTraceparent)

	// Create an Orbital with a nil manager — resolveTasks does not use the manager.
	o := service.NewOrbitalWithManagerForTest(nil)
	o.RegisterJobHandler("test-resolve-job", &noopJobHandler{})

	job := orbital.NewJob("test-resolve-job", nil).WithExternalID("ext-resolve").
		WithLabels(orbital.Labels{"traceparent": parentTraceparent})

	_, err := o.InvokeResolveTasksForTest(context.Background(), job)
	require.NoError(t, err)

	ended := sr.Ended()
	childIdx := -1
	for i, s := range ended {
		if s.Name() == "registry.job.test-resolve-job" {
			childIdx = i
			break
		}
	}
	require.NotEqual(t, -1, childIdx, "registry.job.test-resolve-job span not recorded")
	links := ended[childIdx].Links()
	require.Len(t, links, 1, "span must have exactly one link")

	wantTraceID := parentSpan.SpanContext().TraceID()
	assert.Equal(t, wantTraceID, links[0].SpanContext.TraceID(),
		"link must point to the parent trace ID")
}

// noopJobHandler satisfies service.JobHandler for testing.
type noopJobHandler struct{}

func (h *noopJobHandler) ConfirmJob(_ context.Context, _ orbital.Job) (orbital.JobConfirmerResult, error) {
	return orbital.ContinueJobConfirmer(), nil
}
func (h *noopJobHandler) ResolveTasks(_ context.Context, _ orbital.Job, _ map[string]orbital.TargetManager) (orbital.TaskResolverResult, error) {
	return orbital.CompleteTaskResolver(), nil
}
func (h *noopJobHandler) HandleJobDone(_ context.Context, _ orbital.Job) error     { return nil }
func (h *noopJobHandler) HandleJobCanceled(_ context.Context, _ orbital.Job) error { return nil }
func (h *noopJobHandler) HandleJobFailed(_ context.Context, _ orbital.Job) error   { return nil }

// Compile-time check.
var _ service.JobHandler = (*noopJobHandler)(nil)
