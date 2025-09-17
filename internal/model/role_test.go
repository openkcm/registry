package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/model"
)

func TestRoleValidation(t *testing.T) {
	tests := map[string]struct {
		role      model.Role
		expectErr bool
	}{
		"Valid role": {
			role:      "ROLE_LIVE",
			expectErr: false,
		},
		"Empty role": {
			role:      "",
			expectErr: true,
		},
		"Unspecified role": {
			role:      "ROLE_UNSPECFIED",
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := test.role.Validate(model.EmptyValidationContext)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
