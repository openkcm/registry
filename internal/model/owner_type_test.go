package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithOwnerType struct {
	OwnerType model.OwnerType `validators:"non-empty"`
}

func TestOwnerTypeValidation(t *testing.T) {
	typeWithOwnerType := TypeWithOwnerType{}
	model.RegisterValidatorsForTypes(typeWithOwnerType)
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		ownerType model.OwnerType
		expectErr bool
	}{
		"Valid OwnerType - CustomerID": {
			ownerType: "owner type",
			expectErr: false,
		},
		"Invalid OwnerType": {
			ownerType: "",
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when
			typeWithOwnerType.OwnerType = test.ownerType
			err := model.ValidateField(&typeWithOwnerType, &typeWithOwnerType.OwnerType)

			// then
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, model.ErrFieldValueMustNotBeEmpty)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
