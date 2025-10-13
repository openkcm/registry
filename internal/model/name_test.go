package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithName struct {
	Name model.Name `validators:"non-empty"`
}

func TestNameValidation(t *testing.T) {
	typeWithName := TypeWithName{}
	model.RegisterValidatorsForTypes(typeWithName)
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		name      model.Name
		expectErr bool
	}{
		"Valid name": {
			name:      "SuccessFactor",
			expectErr: false,
		},
		"Empty name": {
			name:      "",
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			typeWithName.Name = test.name
			err := model.ValidateField(&typeWithName, &typeWithName.Name)
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, model.ErrFieldValueMustNotBeEmpty)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
