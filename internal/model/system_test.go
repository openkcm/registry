package model_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	typespb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/validation"
)

func TestSystemToProto(t *testing.T) {
	tenantID := uuid.NewString()
	labelKey := "key1"
	system := model.System{
		ExternalID: uuid.NewString(),
		TenantID:   &tenantID,
		Region:     "REGION_EU",
		L2KeyID:    uuid.NewString(),
		Status:     typespb.Status_STATUS_AVAILABLE.String(),
		Type:       "SYSTEM",
		Labels: map[string]string{
			labelKey: "value1",
		},
		UpdatedAt: time.Date(2025, 8, 5, 12, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2025, 8, 5, 12, 0, 0, 0, time.UTC),
	}

	protoSystem := system.ToProto()

	assert.Equal(t, system.ExternalID, protoSystem.GetExternalId())
	assert.Equal(t, *system.TenantID, protoSystem.GetTenantId())
	assert.Equal(t, system.Region, protoSystem.GetRegion())
	assert.Equal(t, system.L2KeyID, protoSystem.GetL2KeyId())
	assert.False(t, protoSystem.GetHasL1KeyClaim())
	assert.Equal(t, typespb.Status(typespb.Status_value[system.Status]), protoSystem.GetStatus())
	assert.Equal(t, system.Type, protoSystem.GetType())
	assert.Len(t, protoSystem.GetLabels(), 1)
	assert.Equal(t, system.Labels[labelKey], protoSystem.GetLabels()[labelKey])
	assert.Equal(t, system.UpdatedAt.UTC().Format(time.RFC3339Nano), protoSystem.GetUpdatedAt())
	assert.Equal(t, system.CreatedAt.UTC().Format(time.RFC3339Nano), protoSystem.GetCreatedAt())
}

func TestSystemValidations(t *testing.T) {
	// given
	v, err := validation.New(validation.Config{
		Models: []validation.Model{&model.System{}},
	})
	assert.NoError(t, err)

	validSystem := model.System{
		ExternalID: uuid.NewString(),
		Region:     "REGION_US",
		Status:     typespb.Status_STATUS_AVAILABLE.String(),
		L2KeyID:    uuid.NewString(),
		Type:       "SYSTEM_TYPE",
		Labels: map[string]string{
			"env": "prod",
		},
	}

	type mutateSystem func(s model.System) model.System

	tests := []struct {
		name   string
		mutate mutateSystem
		expErr error
	}{
		{
			name: "should return error for empty ExternalID",
			mutate: func(s model.System) model.System {
				s.ExternalID = ""
				return s
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should return error for empty Region",
			mutate: func(s model.System) model.System {
				s.Region = ""
				return s
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should return error for invalid Status",
			mutate: func(s model.System) model.System {
				s.Status = "INVALID_STATUS"
				return s
			},
			expErr: validation.ErrValueNotAllowed,
		},
		{
			name: "should return error for empty L2KeyID",
			mutate: func(s model.System) model.System {
				s.L2KeyID = ""
				return s
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should return error for empty Type",
			mutate: func(s model.System) model.System {
				s.Type = ""
				return s
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should pass for valid System",
			mutate: func(s model.System) model.System {
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
	constraint := model.SystemStatusConstraint{}
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
