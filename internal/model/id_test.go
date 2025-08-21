package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/model"
)

func TestIDValidation(t *testing.T) {
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
			err := test.id.Validate()
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
