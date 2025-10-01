package model_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	typespb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/model"
)

func TestSystemValidation(t *testing.T) {
	// given
	// Setup validators for SystemType which uses the new validation system
	testValidators := &config.TypeValidators{
		{
			TypeName: "model.System",
			Fields: config.FieldValidators{
				{
					FieldName: "Type",
					Rules: []config.ValidationRule{
						{
							Type:          "enum",
							AllowedValues: []string{"system", "app", "api"},
						},
					},
				},
			},
		},
	}

	// Set global validators
	model.SetGlobalTypeValidators(testValidators)
	model.RegisterValidatorsForTypes(model.System{})
	defer model.ClearGlobalTypeValidators()

	claimTrue := true
	tenantID := uuid.New().String()
	tests := map[string]struct {
		system    model.System
		expectErr bool
		errMsg    string
	}{
		"Valid system data": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Type:          "system",
				Labels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
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
				Type:          "system",
			},
			expectErr: true,
			errMsg:    model.FieldValueMustNotBeEmptyMsg + ": ExternalID",
		},
		"System data missing L2KeyID": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				HasL1KeyClaim: &claimTrue,
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Type:          "system",
			},
			expectErr: true,
			errMsg:    model.FieldValueMustNotBeEmptyMsg + ": L2KeyID",
		},
		"System data missing Region": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Type:          "system",
			},
			expectErr: true,
			errMsg:    model.FieldValueMustNotBeEmptyMsg + ": Region",
		},
		"System data empty Region": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Type:          "system",
			},
			expectErr: true,
			errMsg:    model.FieldValueMustNotBeEmptyMsg + ": Region",
		},
		"System status unspecified": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Type:          "system",
			},
			expectErr: true,
			errMsg:    model.FieldValueMustNotBeEmptyMsg + ": Status",
		},
		"System type missing": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Type:          model.SystemType(""),
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
			},
			expectErr: true,
			errMsg:    model.FieldValueMustNotBeEmptyMsg + ": Type",
		},
		"System type incorrect": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Type:          "invalid_type",
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
			},
			expectErr: true,
			errMsg:    model.InvalidFieldValueMsg + ": 'invalid_type' for field 'Type'",
		},
		"Missing label key": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Type:          "system",
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Labels: map[string]string{
					"": "value",
				},
			},
			expectErr: true,
			errMsg:    model.FieldContainsEmptyKeysMsg + ": Labels",
		},
		"Missing label value": {
			system: model.System{
				ExternalID:    "1234567890-asdfghjkl~qwertyuio._zxcvbnmp",
				TenantID:      &tenantID,
				Region:        "REGION_EU",
				L2KeyID:       "d2c16bd0-1398-43ca-aa76-580e9b0c3713",
				HasL1KeyClaim: &claimTrue,
				Type:          "system",
				Status:        model.Status(typespb.Status_STATUS_AVAILABLE.String()),
				Labels: map[string]string{
					"key": "",
				},
			},
			expectErr: true,
			errMsg:    model.FieldContainsEmptyValuesMsg + ": Labels",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when
			err := test.system.Validate()

			// then
			if test.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.errMsg)
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
		Type:       "system",
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
