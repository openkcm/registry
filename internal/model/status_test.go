package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithStatus struct {
	Status model.Status `validators:"custom"`
}

func TestStatusValidation(t *testing.T) {
	typeWithStatus := TypeWithStatus{}
	model.RegisterValidatorsForTypes(typeWithStatus)
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		status    model.Status
		expectErr bool
		err       error
	}{
		"Valid status": {
			status:    model.Status(pb.Status_STATUS_AVAILABLE.String()),
			expectErr: false,
		},
		"Empty status": {
			status:    "",
			expectErr: true,
			err:       model.ErrFieldValueMustNotBeEmpty,
		},
		"Unspecified status": {
			status:    model.Status(pb.Status_STATUS_UNSPECIFIED.String()),
			expectErr: true,
			err:       model.ErrInvalidFieldValue,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			typeWithStatus.Status = test.status
			err := model.ValidateField(&typeWithStatus, &typeWithStatus.Status)
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStatus_IsAvailable(t *testing.T) {
	tests := map[string]struct {
		status    model.Status
		expected  bool
		expectErr bool
	}{
		"Valid available status": {
			status:    model.Status(pb.Status_STATUS_AVAILABLE.String()),
			expected:  true,
			expectErr: false,
		},
		"Valid unavailable status": {
			status:    model.Status(pb.Status_STATUS_PROCESSING.String()),
			expected:  false,
			expectErr: false,
		},
		"Unspecified status": {
			status:    model.Status(pb.Status_STATUS_UNSPECIFIED.String()),
			expected:  false,
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res, err := test.status.IsAvailable()
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expected, res)
		})
	}
}
