package model_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	typespb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/validation"
)

func TestSystemToProto(t *testing.T) {
	tenantID := uuid.NewString()
	labelKey := "key1"
	externalID := uuid.NewString()
	system := model.RegionalSystem{
		SystemID: fmt.Sprintf("%s-%s", externalID, "SYSTEM"),
		Region:   "REGION_EU",
		L2KeyID:  uuid.NewString(),
		Status:   typespb.Status_STATUS_AVAILABLE.String(),
		Labels: map[string]string{
			labelKey: "value1",
		},
		UpdatedAt: time.Date(2025, 8, 5, 12, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2025, 8, 5, 12, 0, 0, 0, time.UTC),
		System: &model.System{
			ID:         fmt.Sprintf("%s-%s", externalID, "SYSTEM"),
			ExternalID: externalID,
			TenantID:   &tenantID,
			Type:       "SYSTEM",
			UpdatedAt:  time.Date(2025, 8, 5, 12, 0, 0, 0, time.UTC),
			CreatedAt:  time.Date(2025, 8, 5, 12, 0, 0, 0, time.UTC),
		},
	}

	protoSystem, err := system.ToProto()
	require.NoError(t, err)

	assert.Equal(t, system.System.ExternalID, protoSystem.GetExternalId())
	assert.Equal(t, *system.System.TenantID, protoSystem.GetTenantId())
	assert.Equal(t, system.Region, protoSystem.GetRegion())
	assert.Equal(t, system.L2KeyID, protoSystem.GetL2KeyId())
	assert.False(t, protoSystem.GetHasL1KeyClaim())
	assert.Equal(t, typespb.Status(typespb.Status_value[system.Status]), protoSystem.GetStatus())
	assert.Equal(t, system.System.Type, protoSystem.GetType())
	assert.Len(t, protoSystem.GetLabels(), 1)
	assert.Equal(t, system.Labels[labelKey], protoSystem.GetLabels()[labelKey])
	assert.Equal(t, system.UpdatedAt.UTC().Format(time.RFC3339Nano), protoSystem.GetUpdatedAt())
	assert.Equal(t, system.CreatedAt.UTC().Format(time.RFC3339Nano), protoSystem.GetCreatedAt())

	system.System = nil
	protoSystem, err = system.ToProto()
	require.Nil(t, protoSystem)
	require.ErrorIs(t, err, model.ErrSystemNotLoaded)
}

func TestRegionalSystemValidations(t *testing.T) {
	// given
	v, err := validation.New(validation.Config{
		Models: []validation.Model{&model.RegionalSystem{}},
	})
	assert.NoError(t, err)

	validSystem := model.RegionalSystem{
		SystemID: uuid.NewString(),
		Region:   "REGION_US",
		Status:   typespb.Status_STATUS_AVAILABLE.String(),
		L2KeyID:  uuid.NewString(),
		Labels: map[string]string{
			"env": "prod",
		},
	}

	type mutateSystem func(s model.RegionalSystem) model.RegionalSystem

	tests := []struct {
		name   string
		mutate mutateSystem
		expErr error
	}{
		{
			name: "should return error for empty ExternalID",
			mutate: func(s model.RegionalSystem) model.RegionalSystem {
				s.SystemID = ""
				return s
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should return error for empty Region",
			mutate: func(s model.RegionalSystem) model.RegionalSystem {
				s.Region = ""
				return s
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should return error for invalid Status",
			mutate: func(s model.RegionalSystem) model.RegionalSystem {
				s.Status = "INVALID_STATUS"
				return s
			},
			expErr: validation.ErrValueNotAllowed,
		},
		{
			name: "should return error for empty L2KeyID",
			mutate: func(s model.RegionalSystem) model.RegionalSystem {
				s.L2KeyID = ""
				return s
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should pass for valid System",
			mutate: func(s model.RegionalSystem) model.RegionalSystem {
				return s
			},
			expErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			system := tt.mutate(validSystem)
			values, err := validation.GetValues(&system)
			assert.NoError(t, err)

			err = v.ValidateAll(values)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestSystemConstraint(t *testing.T) {
	// given
	constraint := model.RegionalSystemStatusConstraint{}
	tests := []struct {
		name   string
		value  any
		expErr error
	}{
		{
			name:   "should return error for wrong type",
			value:  123,
			expErr: validation.ErrWrongType,
		},
		{
			name:   "should return error for invalid status",
			value:  "INVALID_STATUS",
			expErr: validation.ErrValueNotAllowed,
		},
		{
			name:   "should pass for valid status",
			value:  typespb.Status_STATUS_AVAILABLE.String(),
			expErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			err := constraint.Validate(tt.value)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}
