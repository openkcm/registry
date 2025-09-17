package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/model"
)

func TestL2KeyIDValidation(t *testing.T) {
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
			err := test.l2KeyID.Validate(model.EmptyValidationContext)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
