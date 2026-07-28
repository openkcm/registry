package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/service"
	"github.com/openkcm/registry/internal/validation"
)

func newTenantTestValidation(t *testing.T) *validation.Validation {
	t.Helper()
	v, err := validation.New(validation.Config{
		Models: []validation.Model{&model.Tenant{}},
	})
	require.NoError(t, err)
	return v
}

func TestTerminateTenant(t *testing.T) {
	t.Run("should return InvalidArgument when tenant ID is empty", func(t *testing.T) {
		subj := service.NewTenantForTest(nil, newTenantTestValidation(t))

		resp, err := subj.TerminateTenant(context.Background(), &tenantgrpc.TerminateTenantRequest{
			Id: "",
		})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}
