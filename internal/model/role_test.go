package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithRole struct {
	Role model.Role `validators:"custom"`
}

func TestRoleValidation(t *testing.T) {
	typeWithRole := TypeWithRole{}
	model.RegisterValidatorsForTypes(typeWithRole)
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		role      model.Role
		expectErr bool
		err       error
	}{
		"Valid role": {
			role:      "ROLE_LIVE",
			expectErr: false,
		},
		"Empty role": {
			role:      "",
			expectErr: true,
			err:       model.ErrFieldValueMustNotBeEmpty,
		},
		"Unspecified role": {
			role:      "ROLE_UNSPECIFIED",
			expectErr: true,
			err:       model.ErrInvalidFieldValue,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			typeWithRole.Role = test.role
			err := model.ValidateField(&typeWithRole, &typeWithRole.Role)
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
