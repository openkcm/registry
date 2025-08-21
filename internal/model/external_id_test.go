package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/model"
)

func TestExternalID_Validate(t *testing.T) {
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
			err := test.externalID.Validate()
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
