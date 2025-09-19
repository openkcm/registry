package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithID struct {
	ID model.ID `validators:"non-empty"`
}

func TestIDValidation(t *testing.T) {
	typeWithID := TypeWithID{}
	model.RegisterValidatorsForTypes(typeWithID)
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		id        model.ID
		expectErr bool
	}{
		"Empty ID should fail validation": {
			id:        "",
			expectErr: true,
		},
		"Non empty id should pass validation": {
			id:        "1234567890",
			expectErr: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			typeWithID.ID = test.id
			err := model.ValidateField(&typeWithID, &typeWithID.ID)
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, model.ErrFieldValueMustNotBeEmpty)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
