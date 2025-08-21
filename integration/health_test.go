//go:build integration
// +build integration

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealth(t *testing.T) {
	// given
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer conn.Close()

	subj := healthgrpc.NewHealthClient(conn)

	ctx := t.Context()

	t.Run("Health check should succeed", func(t *testing.T) {
		// when
		resp, err := subj.Check(ctx, &healthgrpc.HealthCheckRequest{})

		// then
		assert.NotNil(t, resp)
		assert.NoError(t, err)
	})
}
