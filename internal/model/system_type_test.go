package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/model"
)

func TestSystemTypeValidation(t *testing.T) {
	// Save original global config
	originalConfig := config.GetGlobalConfig()

	defer func() {
		config.SetGlobalConfig(originalConfig)
	}()

	t.Run("with validation config", func(t *testing.T) {
		// Set up validation configuration
		cfg := &config.Config{
			FieldValidation: []config.FieldValidation{
				{
					FieldName: "system.type",
					Rules: []config.ValidationRule{
						{
							Type:          "enum",
							AllowedValues: []string{"system", "application", "service"},
						},
					},
				},
			},
		}
		config.SetGlobalConfig(cfg)

		tests := map[string]struct {
			system    model.SystemType
			expectErr bool
		}{
			"Valid system type - system": {
				system:    "system",
				expectErr: false,
			},
			"Valid system type - application": {
				system:    "application",
				expectErr: false,
			},
			"Valid system type - service": {
				system:    "service",
				expectErr: false,
			},
			"Invalid system type - empty": {
				system:    "",
				expectErr: true,
			},
			"Invalid system type - unknown": {
				system:    "unknown",
				expectErr: true,
			},
			"Invalid system type - case mismatch": {
				system:    "SYSTEM",
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
	})

	t.Run("without validation config", func(t *testing.T) {
		// Clear validation configuration
		config.SetGlobalConfig(nil)

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
				err := test.system.Validate()

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
