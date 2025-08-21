package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/model"
)

func TestNameValidation(t *testing.T) {
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
			err := test.name.Validate()
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
