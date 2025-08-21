package model_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	typespb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/registry/internal/model"
)

func TestSystemValidation(t *testing.T) {
	// given
	claimTrue := true
	tenantID := uuid.New().String()
	tests := map[string]struct {
		system    model.System
		expectErr bool
	}{
		"Valid system data": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Type:          model.SystemTypeSystem,
			},
			expectErr: false,
		},
		"System data missing ExternalID": {
			system: model.System{
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Type:          model.SystemTypeSystem,
			},
			expectErr: true,
		},
		"System data missing L2KeyID": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				HasL1KeyClaim: &claimTrue,
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Type:          model.SystemTypeSystem,
			},
			expectErr: true,
		},
		"System data missing Region": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Type:          model.SystemTypeSystem,
			},
			expectErr: true,
		},
		"System data empty Region": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Type:          model.SystemTypeSystem,
			},
			expectErr: true,
		},
		"System status unspecified": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Type:          model.SystemTypeSystem,
			},
			expectErr: true,
		},
		"System type incorrect": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Type:          "invalid_type",
			},
			expectErr: true,
		},
		"Missing label key": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Type:          "invalid_type",
				Labels: map[string]string{
					"": "value",
				},
			},
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when
			err := test.system.Validate()

			// then
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSystemToProto(t *testing.T) {
	tenantID := uuid.NewString()
	labelKey := "key1"
	system := model.System{
		ExternalID: model.ExternalID(uuid.NewString()),
		TenantID:   &tenantID,
		Region:     "REGION_EU",
		L2KeyID:    model.L2KeyID(uuid.NewString()),
		Status:     model.Status(typespb.Status_STATUS_AVAILABLE.String()),
		Type:       model.SystemTypeSystem,
		Labels: map[string]string{
			labelKey: "value1",
		},
		UpdatedAt: time.Date(2025, 8, 5, 12, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2025, 8, 5, 12, 0, 0, 0, time.UTC),
	}

	protoSystem := system.ToProto()

	assert.Equal(t, string(system.ExternalID), protoSystem.GetExternalId())
	assert.Equal(t, *system.TenantID, protoSystem.GetTenantId())
	assert.Equal(t, string(system.Region), protoSystem.GetRegion())
	assert.Equal(t, string(system.L2KeyID), protoSystem.GetL2KeyId())
	assert.False(t, protoSystem.GetHasL1KeyClaim())
	assert.Equal(t, typespb.Status(typespb.Status_value[string(system.Status)]), protoSystem.GetStatus())
	assert.Equal(t, string(system.Type), protoSystem.GetType())
	assert.Len(t, protoSystem.GetLabels(), 1)
	assert.Equal(t, system.Labels[labelKey], protoSystem.GetLabels()[labelKey])
	assert.Equal(t, system.UpdatedAt.UTC().Format(time.RFC3339Nano), protoSystem.GetUpdatedAt())
	assert.Equal(t, system.CreatedAt.UTC().Format(time.RFC3339Nano), protoSystem.GetCreatedAt())
}
