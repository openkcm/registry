package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/model"
)

func TestSystemTypeValidation(t *testing.T) {
	tests := map[string]struct {
		system    model.SystemType
		expectErr bool
	}{
		"Valid system type": {
			system:    "system",
			expectErr: false,
		},
		"Invalid system type": {
			system:    "",
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when
			err := test.system.Validate()

			// then
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
