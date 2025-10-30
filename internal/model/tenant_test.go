package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	tenantpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/validation"
)

const tenantOwnerType1 = "ownerType1"

func TestTenantLabelsValidation(t *testing.T) {
	tenantStatusActive := model.TenantStatus(tenantpb.Status_STATUS_ACTIVE.String())

	tests := map[string]struct {
		tenant model.Tenant
	}{
		"Tenant data missing label key": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: tenantOwnerType1,
				OwnerID:   "owner-id-123",
				Role:      "ROLE_TRIAL",
				Labels: map[string]string{
					"": "value",
				},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := test.tenant.Labels.Validate()
			assert.Error(t, err)
		})
	}
}

func TestTenantValidation(t *testing.T) {
	tenantStatusActive := model.TenantStatus(tenantpb.Status_STATUS_ACTIVE.String())

	tests := map[string]struct {
		tenant    model.Tenant
		expectErr bool
	}{
		"Valid tenant data": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: tenantOwnerType1,
				OwnerID:   "owner-id-123",
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
				OwnerType: tenantOwnerType1,
				OwnerID:   "owner-id-123",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
		},
		"Tenant data missing ID": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: tenantOwnerType1,
				OwnerID:   "owner-id-123",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
		},
		"Empty id": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: tenantOwnerType1,
				OwnerID:   "owner-id-123",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
		},
		"Tenant data missing region": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Status:    tenantStatusActive,
				OwnerType: tenantOwnerType1,
				OwnerID:   "owner-id-123",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
		},
		"Tenant data empty owner type": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: "",
				OwnerID:   "owner-id-123",
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
		},
		"Tenant data missing owner id": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: tenantOwnerType1,
				Role:      "ROLE_TRIAL",
			},
			expectErr: true,
		},
		"Tenant data missing role": {
			tenant: model.Tenant{
				Name:      "SuccessFactor",
				ID:        "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				Region:    "CMK_REGION_EU",
				Status:    tenantStatusActive,
				OwnerType: tenantOwnerType1,
				OwnerID:   "owner-id-123",
				Role:      "",
			},
			expectErr: true,
		},
	}

	v, err := validation.New(validation.Config{
		Models: []validation.Model{&model.Tenant{}},
	})
	assert.NoError(t, err)

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			valuesByID, err := validation.GetValues(&test.tenant)
			assert.NoError(t, err)
			err = v.ValidateAll(valuesByID)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTenantRoleConstraints(t *testing.T) {
	// given
	subj := model.TenantRoleConstraint{}

	t.Run("role value", func(t *testing.T) {
		for role := range tenantpb.Role_value {
			t.Run(role, func(t *testing.T) {
				// when
				err := subj.Validate(role)

				// then
				if role == tenantpb.Role_ROLE_UNSPECIFIED.String() {
					assert.Error(t, err)
					return
				}
				assert.NoError(t, err)
			})
		}

		t.Run("should return error for empty value", func(t *testing.T) {
			// when
			err := subj.Validate("")

			// then
			assert.Error(t, err)
		})
	})
}

func TestTenantToProto(t *testing.T) {
	labelKey := "key1"
	tenant := model.Tenant{
		ID:              "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
		Name:            "SuccessFactor",
		Region:          "CMK_REGION_EU",
		OwnerType:       "owner_type",
		OwnerID:         "owner-id-123",
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

	assert.Equal(t, tenant.ID, protoTenant.GetId())
	assert.Equal(t, tenant.Name, protoTenant.GetName())
	assert.Equal(t, tenant.Region, protoTenant.GetRegion())
	assert.Equal(t, tenant.OwnerType, protoTenant.GetOwnerType())
	assert.Equal(t, tenant.OwnerID, protoTenant.GetOwnerId())
	assert.Equal(t, tenantpb.Status(tenantpb.Status_value[string(tenant.Status)]), protoTenant.GetStatus())
	assert.Equal(t, tenant.StatusUpdatedAt.UTC().Format(time.RFC3339Nano), protoTenant.GetStatusUpdatedAt())
	assert.Equal(t, tenantpb.Role(tenantpb.Role_value[tenant.Role]), protoTenant.GetRole())
	assert.Len(t, protoTenant.GetLabels(), 1)
	assert.Equal(t, tenant.Labels[labelKey], protoTenant.GetLabels()[labelKey])
	assert.Equal(t, tenant.UpdatedAt.UTC().Format(time.RFC3339Nano), protoTenant.GetUpdatedAt())
	assert.Equal(t, tenant.CreatedAt.UTC().Format(time.RFC3339Nano), protoTenant.GetCreatedAt())
}
