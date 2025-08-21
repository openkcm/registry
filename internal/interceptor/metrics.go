package interceptor

import (
	"context"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const ErrDomainMetrics = "metrics"

func InitMeters(ctx context.Context, cfgApp *commoncfg.Application, meter metric.Meter) (*Meters, error) {
	var err error

	requestCounts, err := meter.Int64Counter(
		"grpc.request_count",
		metric.WithDescription("Counter of gRPC requests, partitioned by method and status."),
	)
	if err != nil {
		return nil, oops.In(ErrDomainMetrics).
			WithContext(ctx).
			Wrapf(err, "creating grpc_request_count meter")
	}

	requestDurations, err := meter.Float64Histogram(
		"grpc.request_duration",
		metric.WithDescription("Incoming end to end duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, oops.In(ErrDomainMetrics).
			WithContext(ctx).
			Wrapf(err, "creating grpc_request_duration meter")
	}

	return &Meters{
		application:      cfgApp,
		requestCounts:    requestCounts,
		requestDurations: requestDurations,
	}, nil
}

// Meters helps with collecting metrics for prometheus from grpc.
type Meters struct {
	application      *commoncfg.Application
	requestCounts    metric.Int64Counter
	requestDurations metric.Float64Histogram
}

// UnaryInterceptor tracks the duration and count of unary gRPC calls.
func (m *Meters) UnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	requestStartTime := time.Now()
	resp, err := handler(ctx, req)
	elapsedTime := float64(time.Since(requestStartTime)) / float64(time.Millisecond)

	statusCode := status.Code(err).String()

	// Meters logic
	attrs := metric.WithAttributes(
		otlp.CreateAttributesFrom(*m.application,
			attribute.String(commoncfg.AttrOperation, info.FullMethod),
			attribute.String("status", statusCode),
		)...,
	)
	m.requestDurations.Record(ctx, elapsedTime, attrs)
	m.requestCounts.Add(ctx, 1, attrs)

	return resp, err
}

// StreamInterceptor tracks the duration and count of streaming gRPC calls.
func (m *Meters) StreamInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	requestStartTime := time.Now()
	err := handler(srv, stream)
	elapsedTime := float64(time.Since(requestStartTime)) / float64(time.Millisecond)

	statusCode := status.Code(err).String()

	// Meters logic
	attrs := metric.WithAttributes(
		otlp.CreateAttributesFrom(*m.application,
			attribute.String(commoncfg.AttrOperation, info.FullMethod),
			attribute.String("status", statusCode),
		)...,
	)
	m.requestDurations.Record(context.Background(), elapsedTime, attrs)
	m.requestCounts.Add(context.Background(), 1, attrs)

	return err
}
