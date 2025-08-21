package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/model"
)

func TestRegionValidation(t *testing.T) {
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
			err := test.region.Validate()
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
