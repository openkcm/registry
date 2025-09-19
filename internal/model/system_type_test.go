package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/model"
)

type TypeWithSystemType struct {
	SystemType model.SystemType `validators:"non-empty"`
}

func TestSystemTypeValidation(t *testing.T) {
	typeWithSystemType := TypeWithSystemType{}
	model.RegisterValidatorsForTypes(typeWithSystemType)
	defer model.ClearGlobalTypeValidators()

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
			typeWithSystemType.SystemType = test.system
			err := model.ValidateField(&typeWithSystemType, &typeWithSystemType.SystemType)

			// then
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, model.ErrFieldValueMustNotBeEmpty)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSystemTypeValidationWithConfig(t *testing.T) {
	// Set up validation configuration for system types
	testValidators := &config.TypeValidators{
		{
			TypeName: "model_test.TypeWithSystemType",
			Fields: config.FieldValidators{
				{
					FieldName: "SystemType",
					Rules: []config.ValidationRule{
						{
							Type:          "enum",
							AllowedValues: []string{"system", "test"},
						},
					},
				},
			},
		},
	}

	model.SetGlobalTypeValidators(testValidators)
	typeWithSystemType := TypeWithSystemType{}
	model.RegisterValidatorsForTypes(typeWithSystemType)
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		systemType model.SystemType
		expectErr  bool
		err        error
		errMsg     string
	}{
		"Valid system type": {
			systemType: "system",
			expectErr:  false,
		},
		"Valid test type": {
			systemType: "test",
			expectErr:  false,
		},
		"Invalid type - empty": {
			systemType: "",
			expectErr:  true,
			err:        model.ErrFieldValueMustNotBeEmpty,
			errMsg:     "SystemType",
		},
		"Invalid type - not in allowed values": {
			systemType: "invalid",
			expectErr:  true,
			err:        model.ErrInvalidFieldValue,
			errMsg:     "'SystemType', allowed values: [system test]",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when
			typeWithSystemType.SystemType = test.systemType
			err := model.ValidateField(&typeWithSystemType, &typeWithSystemType.SystemType)

			// then
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, test.err)
				assert.Contains(t, err.Error(), test.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
