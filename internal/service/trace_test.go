package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/openkcm/registry/internal/service"
)

// installOTel registers a recording TracerProvider and the W3C propagator globally.
// It restores the originals when the test ends.
func installOTel(t *testing.T) *tracetest.SpanRecorder {
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

func TestInjectTraceContext_NoActiveTrace(t *testing.T) {
	installOTel(t)
	tp, ts := service.InjectTraceContext(context.Background())
	assert.Empty(t, tp)
	assert.Empty(t, ts)
}

func TestInjectTraceContext_WithActiveSpan(t *testing.T) {
	installOTel(t)
	ctx, span := otel.Tracer("test").Start(context.Background(), "parent")
	defer span.End()

	tp, ts := service.InjectTraceContext(ctx)
	assert.NotEmpty(t, tp, "traceparent must be set when a span is active")
	_ = ts // tracestate may be empty
}

func TestStartLinkedSpan_EmptyTraceparent_NoLink(t *testing.T) {
	sr := installOTel(t)
	ctx, span := service.StartLinkedSpan(context.Background(), "child", "", "")
	span.End()

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "child", spans[0].Name())
	assert.Empty(t, spans[0].Links(), "no link expected when traceparent is empty")
	_ = ctx
}

func TestStartLinkedSpan_InvalidTraceparent_NoLink(t *testing.T) {
	sr := installOTel(t)
	ctx, span := service.StartLinkedSpan(context.Background(), "child", "not-a-valid-traceparent", "")
	span.End()

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Empty(t, spans[0].Links(), "no link expected for invalid traceparent")
	_ = ctx
}

func TestStartLinkedSpan_ValidTraceparent_HasLink(t *testing.T) {
	rec := installOTel(t)

	// Build a valid traceparent from a real span.
	parentCtx, parentSpan := otel.Tracer("test").Start(context.Background(), "parent")
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(parentCtx, carrier)
	parentSpan.End()
	parentTraceparent := carrier.Get("traceparent")
	require.NotEmpty(t, parentTraceparent)

	_, childSpan := service.StartLinkedSpan(context.Background(), "child", parentTraceparent, "")
	childSpan.End()

	ended := rec.Ended()
	childIdx := -1
	for i, s := range ended {
		if s.Name() == "child" {
			childIdx = i
			break
		}
	}
	require.NotEqual(t, -1, childIdx, "child span not recorded")
	links := ended[childIdx].Links()
	require.Len(t, links, 1)

	wantTraceID := parentSpan.SpanContext().TraceID()
	assert.Equal(t, wantTraceID, links[0].SpanContext.TraceID(),
		"link must point to the parent trace")
}
