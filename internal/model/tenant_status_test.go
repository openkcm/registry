package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/model"
)

func TestTenantStatus_ValidateTransition(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus model.TenantStatus
		targetStatus  pb.Status
		expErr        error
		expErrMsg     string
	}{
		{
			name:          "Valid transition from REQUESTED to PROVISIONING",
			currentStatus: model.TenantStatus(pb.Status_STATUS_REQUESTED.String()),
			targetStatus:  pb.Status_STATUS_PROVISIONING,
		},
		{
			name:          "Invalid transition from ACTIVE to BLOCKED",
			currentStatus: model.TenantStatus(pb.Status_STATUS_ACTIVE.String()),
			targetStatus:  pb.Status_STATUS_BLOCKED,
			expErr:        model.ErrInvalidTransition,
			expErrMsg:     "invalid tenant status transition from STATUS_ACTIVE to STATUS_BLOCKED",
		},
		{
			name:          "Current status is UNSPECIFIED",
			currentStatus: "",
			targetStatus:  pb.Status_STATUS_ACTIVE,
			expErr:        model.ErrInvalidTransition,
			expErrMsg:     "invalid tenant status transition from STATUS_UNSPECIFIED to STATUS_ACTIVE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.currentStatus.ValidateTransition(tt.targetStatus)
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				assert.Equal(t, tt.expErrMsg, err.Error())

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestTenantStatus_IsActive(t *testing.T) {
	tests := map[string]struct {
		status   model.TenantStatus
		expected bool
	}{
		"Valid available status": {
			status:   model.TenantStatus(pb.Status_STATUS_ACTIVE.String()),
			expected: true,
		},
		"Valid unavailable status": {
			status:   model.TenantStatus(pb.Status_STATUS_BLOCKED.String()),
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := test.status.IsActive()
			assert.Equal(t, test.expected, res)
		})
	}
}
