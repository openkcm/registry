package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithL2KeyID struct {
	L2KeyID model.L2KeyID `validators:"non-empty"`
}

func TestL2KeyIDValidation(t *testing.T) {
	typeWithL2KeyID := TypeWithL2KeyID{}
	model.RegisterValidatorsForTypes(typeWithL2KeyID)
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		l2KeyID   model.L2KeyID
		expectErr bool
	}{
		"Valid name": {
			l2KeyID:   "key-123",
			expectErr: false,
		},
		"Empty name": {
			l2KeyID:   "",
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			typeWithL2KeyID.L2KeyID = test.l2KeyID
			err := model.ValidateField(&typeWithL2KeyID, &typeWithL2KeyID.L2KeyID)
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, model.ErrFieldValueMustNotBeEmpty)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
