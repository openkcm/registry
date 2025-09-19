package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/model"
)

type CustomValidationContext struct{}

func (c CustomValidationContext) ValidateField(fieldValue any) error {
	if fieldValue != "system" {
		return model.ErrInvalidFieldValue
	}
	return nil
}

func TestSystemTypeValidation(t *testing.T) {
	t.Run("with validation config", func(t *testing.T) {
		tests := map[string]struct {
			system    model.SystemType
			expectErr bool
		}{
			"Valid system type": {
				system:    "system",
				expectErr: false,
			},
			"Invalid system type - empty": {
				system:    "",
				expectErr: true,
			},
		}

		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				// when
				err := test.system.Validate(CustomValidationContext{})

				// then
				if test.expectErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("without validation config", func(t *testing.T) {
		tests := map[string]struct {
			system    model.SystemType
			expectErr bool
		}{
			"Any value should be valid without config": {
				system:    "any-value",
				expectErr: false,
			},
			"Empty value should be invalid without config": {
				system:    "",
				expectErr: true,
			},
		}

		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				// when
				err := test.system.Validate(model.EmptyValidationContext)

				// then
				if test.expectErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}
