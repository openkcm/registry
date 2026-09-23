package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tenantconfiggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant_config/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
)

func TestTenantConfigTableName(t *testing.T) {
	assert.Equal(t, "tenant_configs", (&model.TenantConfig{}).TableName())
}

func TestTenantConfigPaginationKey(t *testing.T) {
	cfg := &model.TenantConfig{TenantID: "t-1"}

	key := cfg.PaginationKey()

	assert.Equal(t, "t-1", key[repository.TenantIDField])
}

func TestTenantConfigToProto(t *testing.T) {
	t.Run("nil system limit produces no Values", func(t *testing.T) {
		cfg := &model.TenantConfig{
			TenantID:     "t-1",
			Status:       tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_ACTIVE.String(),
			ErrorMessage: "",
			CreatedAt:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		}

		resp := cfg.ToProto()

		require.NotNil(t, resp)
		assert.Equal(t, "t-1", resp.GetTenantId())
		assert.Equal(t, tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_ACTIVE, resp.GetStatus())
		assert.Empty(t, resp.GetErrorMessage())
		assert.Nil(t, resp.GetValues())
		assert.Equal(t, "2025-01-01T00:00:00Z", resp.GetCreatedAt())
		assert.Equal(t, "2025-06-01T00:00:00Z", resp.GetUpdatedAt())
	})

	t.Run("set system limit is forwarded in Values", func(t *testing.T) {
		sl := int32(100)
		cfg := &model.TenantConfig{
			TenantID:    "t-2",
			SystemLimit: &sl,
			Status:      tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING.String(),
		}

		resp := cfg.ToProto()

		require.NotNil(t, resp)
		assert.Equal(t, "t-2", resp.GetTenantId())
		assert.Equal(t, tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING, resp.GetStatus())
		require.NotNil(t, resp.GetValues())
		assert.Equal(t, int32(100), resp.GetValues().GetSystemLimit())
	})

	t.Run("error message is preserved", func(t *testing.T) {
		cfg := &model.TenantConfig{
			TenantID:     "t-3",
			Status:       tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING_ERROR.String(),
			ErrorMessage: "job failed: timeout",
		}

		resp := cfg.ToProto()

		require.NotNil(t, resp)
		assert.Equal(t, tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING_ERROR, resp.GetStatus())
		assert.Equal(t, "job failed: timeout", resp.GetErrorMessage())
	})
}
