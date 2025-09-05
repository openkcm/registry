package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/model"
)

// TestFieldType is a simple type that implements the Validator interface for testing.
type TestFieldType string

func (t TestFieldType) Validate() error {
	if t == "" {
		return model.ErrInvalidFieldValue
	}
	return nil
}

func TestValidateStruct(t *testing.T) {
	// Create a test struct
	type TestStruct struct {
		TestField  TestFieldType `gorm:"column:test_field"`
		OtherField string        `gorm:"column:other_field"`
	}

	// Setup test validators
	testValidators := &config.TypeValidators{
		"test_struct": config.FieldValidators{
			{
				FieldName: "test_field",
				Rules: []config.ValidationRule{
					{
						Type:          "enum",
						AllowedValues: []string{"valid_value1", "valid_value2"},
					},
				},
			},
		},
	}

	// Set global validators
	model.SetGlobalTypeValidators(testValidators)

	tests := map[string]struct {
		structPtr interface{}
		typeName  string
		expectErr bool
		errMsg    string
	}{
		"Valid struct with valid field": {
			structPtr: &TestStruct{
				TestField: TestFieldType("valid_value1"),
			},
			typeName:  "test_struct",
			expectErr: false,
		},
		"Invalid struct with invalid field": {
			structPtr: &TestStruct{
				TestField: TestFieldType("invalid_value"),
			},
			typeName:  "test_struct",
			expectErr: true,
			errMsg:    "invalid field value",
		},
		"Struct with no validators configured": {
			structPtr: &TestStruct{
				TestField: TestFieldType("any_value"),
			},
			typeName:  "unknown_type",
			expectErr: false,
		},
		"Non-struct pointer should return error": {
			structPtr: "not a struct",
			typeName:  "test_struct",
			expectErr: true,
			errMsg:    "expected a pointer to struct",
		},
		"Non-pointer should return error": {
			structPtr: TestStruct{},
			typeName:  "test_struct",
			expectErr: true,
			errMsg:    "expected a pointer to struct",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := model.ValidateStruct(test.structPtr, test.typeName)

			if test.expectErr {
				assert.Error(t, err)

				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateStructWithRealModels tests the ValidateStruct function with actual model types.
func TestValidateStructWithRealModels(t *testing.T) {
	// Setup validators for SystemType which uses the new validation system
	testValidators := &config.TypeValidators{
		"system": config.FieldValidators{
			{
				FieldName: "type",
				Rules: []config.ValidationRule{
					{
						Type:          "enum",
						AllowedValues: []string{"web", "mobile", "api"},
					},
				},
			},
		},
	}

	// Set global validators
	model.SetGlobalTypeValidators(testValidators)

	tests := map[string]struct {
		structPtr interface{}
		expectErr bool
	}{
		"Valid system with valid type": {
			structPtr: &model.System{
				ExternalID: model.ExternalID("1234567890-asdfghjkl~qwertyuiop"),
				Region:     model.Region("REGION_EU"),
				L2KeyID:    model.L2KeyID("l2keyid-1234567890"),
				Type:       model.SystemType("web"),
				Status:     model.Status("STATUS_AVAILABLE"), // Valid protobuf status
			},
			expectErr: false,
		},
		"Invalid system with invalid type": {
			structPtr: &model.System{
				ExternalID: model.ExternalID("1234567890-asdfghjkl~qwertyuiop"),
				Region:     model.Region("REGION_EU"),
				L2KeyID:    model.L2KeyID("l2keyid-1234567890"),
				Type:       model.SystemType("desktop"),      // Invalid according to our config
				Status:     model.Status("STATUS_AVAILABLE"), // Valid protobuf status
			},
			expectErr: true,
		},
		"Invalid system with empty type": {
			structPtr: &model.System{
				ExternalID: model.ExternalID("1234567890-asdfghjkl~qwertyuiop"),
				Region:     model.Region("REGION_EU"),
				L2KeyID:    model.L2KeyID("l2keyid-1234567890"),
				Type:       model.SystemType(""),             // Empty type
				Status:     model.Status("STATUS_AVAILABLE"), // Valid protobuf status
			},
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := model.ValidateStruct(test.structPtr, "system")

			if test.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid field value")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractFieldNameFromTag(t *testing.T) {
	// This test verifies the GORM tag parsing logic
	// Since extractFieldNameFromTag is not exported, we test it indirectly through ValidateStruct

	// Create a test struct with GORM tags
	type TestStruct struct {
		FieldWithColumn    model.SystemType `gorm:"column:type;not null"`
		FieldWithoutColumn string           `gorm:"not null"`
		FieldNoGormTag     string
	}

	// Setup validator for "type" field
	testValidators := &config.TypeValidators{
		"test": config.FieldValidators{
			{
				FieldName: "type",
				Rules: []config.ValidationRule{
					{
						Type:          "enum",
						AllowedValues: []string{"valid"},
					},
				},
			},
		},
	}

	model.SetGlobalTypeValidators(testValidators)

	testStruct := &TestStruct{
		FieldWithColumn: model.SystemType("valid"),
	}

	// Should not error because "type" field has valid value
	err := model.ValidateStruct(testStruct, "test")
	assert.NoError(t, err)

	// Test with invalid value
	testStruct.FieldWithColumn = model.SystemType("invalid")
	err = model.ValidateStruct(testStruct, "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid field value")
}
