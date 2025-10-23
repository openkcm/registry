package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithExternalID struct {
	ExternalID model.ExternalID `validators:"non-empty"`
}

func TestExternalID_Validate(t *testing.T) {
	typeWithExternalID := TypeWithExternalID{}
	model.RegisterValidatorsForTypes(typeWithExternalID)
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		externalID model.ExternalID
		expectErr  bool
	}{
		"Empty ExternalID should fail validation": {
			externalID: "",
			expectErr:  true,
		},
		"ExternalID with value should pass the validation": {
			externalID: "valid-external-id",
			expectErr:  false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			typeWithExternalID.ExternalID = test.externalID
			err := model.ValidateField(&typeWithExternalID, &typeWithExternalID.ExternalID)
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, model.ErrFieldValueMustNotBeEmpty)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
