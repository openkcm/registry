package interceptor_test

import (
	"context"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/openkcm/registry/internal/interceptor"
)

func TestMetricsUnaryInterceptor(t *testing.T) {
	ctx := t.Context()
	app := &commoncfg.Application{}

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	met, err := interceptor.InitMeters(ctx, app, meter)
	if err != nil {
		require.NoError(t, err)
	}

	handler := func(_ context.Context, _ any) (any, error) {
		time.Sleep(51 * time.Millisecond)
		return nil, status.Error(codes.OK, "OK")
	}

	_, err = met.UnaryInterceptor(
		t.Context(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/test.method"},
		handler,
	)
	if err != nil {
		require.NoError(t, err)
	}

	var out metricdata.ResourceMetrics

	err = reader.Collect(ctx, &out)
	if err != nil {
		t.Fatal(err)
	}

	// Find the counter value in collected metrics
	var (
		requestCount                int64
		countExists, durationExists bool
	)

	for _, scopeMetrics := range out.ScopeMetrics {
		for _, m := range scopeMetrics.Metrics {
			switch m.Name {
			case "grpc.request_count":
				countExists = true
				dp, ok := m.Data.(metricdata.Sum[int64])
				assert.True(t, ok, "unexpected data type")

				requestCount = dp.DataPoints[0].Value
				assert.Equal(t, int64(1), requestCount, "unexpected request count")
			case "grpc.request_duration":
				durationExists = true
				dp, ok := m.Data.(metricdata.Histogram[float64])
				assert.True(t, ok, "unexpected data type")
				assert.Len(t, dp.DataPoints, 1, "unexpected amount of data points")
				assert.Equal(t, uint64(1), dp.DataPoints[0].Count, "unexpected request duration count")
				assert.Equal(t, uint64(1), dp.DataPoints[0].BucketCounts[5], "unexpected request duration bucket number (should be <75ms)")
			}
		}
	}

	assert.True(t, countExists, "request count metric not found")
	assert.True(t, durationExists, "request duration metric not found")
}

func TestMetricsStreamInterceptor(t *testing.T) {
	ctx := t.Context()
	app := &commoncfg.Application{}
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	met, err := interceptor.InitMeters(ctx, app, meter)
	if err != nil {
		require.NoError(t, err)
	}

	handler := func(_ any, _ grpc.ServerStream) error {
		time.Sleep(51 * time.Millisecond) // Simulate processing time
		return status.Error(codes.OK, "OK")
	}

	err = met.StreamInterceptor(
		nil,
		nil,
		&grpc.StreamServerInfo{FullMethod: "/test.method"},
		handler,
	)
	if err != nil {
		require.NoError(t, err)
	}

	var out metricdata.ResourceMetrics

	err = reader.Collect(ctx, &out)
	if err != nil {
		t.Fatal(err)
	}

	// Find the counter value in collected metrics
	var requestCount int64

	for _, scopeMetrics := range out.ScopeMetrics {
		for _, m := range scopeMetrics.Metrics {
			switch m.Name {
			case "grpc.request_count":
				dp, ok := m.Data.(metricdata.Sum[int64])
				assert.True(t, ok, "unexpected data type")

				requestCount = dp.DataPoints[0].Value
				assert.Equal(t, int64(1), requestCount, "unexpected request count")
			case "grpc.request_duration":
				dp, ok := m.Data.(metricdata.Histogram[float64])
				assert.True(t, ok, "unexpected data type")
				assert.Len(t, dp.DataPoints, 1, "unexpected amount of data points")
				assert.Equal(t, uint64(1), dp.DataPoints[0].Count, "unexpected request duration count")
				assert.Equal(t, uint64(1), dp.DataPoints[0].BucketCounts[5], "unexpected request duration bucket number (should be <75ms)")
			}
		}
	}
}
