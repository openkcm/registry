package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tenantpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/model"
)

func TestTenantValidation(t *testing.T) {
	tenantStatusActive := model.TenantStatus(tenantpb.Status_STATUS_ACTIVE.String())

	tests := map[string]struct {
		tenant    model.Tenant
		expectErr bool
		errMsg    string
	}{
		"Valid tenant data": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "owner_type",
				OwnerID:   "customer_id",
				Role:      "ROLE_TRIAL",
			},
			expectErr: false,
		},
		"Tenant data missing name": {
			tenant: model.Tenant{
				Name:      "",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "owner_type",
				OwnerID:   "customer_id",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
			errMsg:    "name is empty",
		},
		"Tenant data empty ID": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "owner_type",
				OwnerID:   "customer_id",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
			errMsg:    "id is empty",
		},
		"Tenant data missing id": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "owner_type",
				OwnerID:   "customer_id",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
			errMsg:    "id is empty",
		},
		"Tenant data missing region": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Status:    tenantStatusActive,
				OwnerType: "owner_type",
				OwnerID:   "customer_id",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
			errMsg:    "region is empty",
		},
		"Tenant data empty owner type": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "",
				OwnerID:   "customer_id",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
			errMsg:    "owner type is empty",
		},
		"Tenant data missing owner type": {
			tenant: model.Tenant{
				Name:    "SuccessFactor",
				ID:      "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:  "CMK_REGION_EU",
				Status:  tenantStatusActive,
				OwnerID: "customer_id",
				Role:    "ROLE_TRIAL",
			},
			expectErr: true,
			errMsg:    "owner type is empty",
		},
		"Tenant data empty owner id": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "some_owner",
				OwnerID:   "",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
			errMsg:    "owner id is empty",
		},
		"Tenant data missing owner id": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "some_owner",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
			errMsg:    "owner id is empty",
		},
		"Tenant data empty role": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "owner_type",
				OwnerID:   "customer_id",
				Role:      "",
			},
			expectErr: true,
			errMsg:    "role is invalid",
		},
		"Tenant data empty label key": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "owner_type",
				OwnerID:   "customer_id",
				Role:      "ROLE_TRIAL",
				Labels: map[string]string{
					"": "value",
				},
			},
			expectErr: true,
			errMsg:    "labels include empty string",
		},
		"Tenant data empty label value": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "owner_type",
				OwnerID:   "customer_id",
				Role:      "ROLE_TRIAL",
				Labels: map[string]string{
					"key": "",
				},
			},
			expectErr: true,
			errMsg:    "labels include empty string",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := test.tenant.Validate(model.EmptyValidationContext)
			if test.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTenantToProto(t *testing.T) {
	labelKey := "key1"
	tenant := model.Tenant{
		ID:              "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
		Name:            "SuccessFactor",
		Region:          "CMK_REGION_EU",
		OwnerType:       "owner_type",
		OwnerID:         "customer_id",
		Status:          model.TenantStatus(tenantpb.Status_STATUS_ACTIVE.String()),
		StatusUpdatedAt: time.Date(2025, 6, 3, 12, 0, 0, 0, time.UTC),
		Role:            "ROLE_TRIAL",
		Labels: map[string]string{
			labelKey: "value1",
		},
		UpdatedAt: time.Date(2025, 8, 5, 12, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2025, 8, 5, 12, 0, 0, 0, time.UTC),
	}

	protoTenant := tenant.ToProto()

	assert.Equal(t, tenant.ID.String(), protoTenant.GetId())
	assert.Equal(t, tenant.Name.String(), protoTenant.GetName())
	assert.Equal(t, tenant.Region.String(), protoTenant.GetRegion())
	assert.Equal(t, tenant.OwnerType.String(), protoTenant.GetOwnerType())
	assert.Equal(t, tenant.OwnerID.String(), protoTenant.GetOwnerId())
	assert.Equal(t, tenantpb.Status(tenantpb.Status_value[string(tenant.Status)]), protoTenant.GetStatus())
	assert.Equal(t, tenant.StatusUpdatedAt.UTC().Format(time.RFC3339Nano), protoTenant.GetStatusUpdatedAt())
	assert.Equal(t, tenantpb.Role(tenantpb.Role_value[string(tenant.Role)]), protoTenant.GetRole())
	assert.Len(t, protoTenant.GetLabels(), 1)
	assert.Equal(t, tenant.Labels[labelKey], protoTenant.GetLabels()[labelKey])
	assert.Equal(t, tenant.UpdatedAt.UTC().Format(time.RFC3339Nano), protoTenant.GetUpdatedAt())
	assert.Equal(t, tenant.CreatedAt.UTC().Format(time.RFC3339Nano), protoTenant.GetCreatedAt())
}
