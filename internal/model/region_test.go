package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithRegion struct {
	Region model.Region `validators:"non-empty"`
}

func TestRegionValidation(t *testing.T) {
	typeWithRegion := TypeWithRegion{}
	model.RegisterValidatorsForTypes(typeWithRegion)
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		region    model.Region
		expectErr bool
	}{
		"Valid region": {
			region:    "REGION_EU",
			expectErr: false,
		},
		"Invalid region": {
			region:    "",
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			typeWithRegion.Region = test.region
			err := model.ValidateField(&typeWithRegion, &typeWithRegion.Region)
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, model.ErrFieldValueMustNotBeEmpty)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
