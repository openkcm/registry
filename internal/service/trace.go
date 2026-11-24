package service

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// InjectTraceContext extracts the active span's W3C traceparent and tracestate from ctx.
// Both values are empty when no span is active.
func InjectTraceContext(ctx context.Context) (string, string) {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier.Get("traceparent"), carrier.Get("tracestate")
}

// StartLinkedSpan starts a new span linked to the trace identified by traceparent/tracestate.
// When traceparent is empty or invalid no link is added.
// The caller is responsible for ending the returned span.
func StartLinkedSpan(ctx context.Context, name, traceparent, tracestate string) (context.Context, trace.Span) {
	var opts []trace.SpanStartOption
	if link, ok := extractSpanLink(traceparent, tracestate); ok {
		opts = append(opts, trace.WithLinks(link))
	}
	ctx, span := otel.GetTracerProvider().Tracer("").Start(ctx, name, opts...) //nolint:spancheck // caller owns the span
	return ctx, span                                                           //nolint:spancheck // caller owns the span
}

func extractSpanLink(traceparent, tracestate string) (trace.Link, bool) {
	if traceparent == "" {
		return trace.Link{}, false
	}
	carrier := propagation.MapCarrier{"traceparent": traceparent, "tracestate": tracestate}
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return trace.Link{}, false
	}
	return trace.Link{SpanContext: spanCtx}, true
}
