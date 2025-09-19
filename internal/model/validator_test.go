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

func (t TestFieldType) Validate(ctx model.ValidationContext) error {
	if t == "" {
		return model.ErrInvalidFieldValue
	}
	if err := ctx.ValidateField(string(t)); err != nil {
		return err
	}

	return nil
}

func TestValidateStruct(t *testing.T) {
	// Create a test struct
	type TestStruct struct {
		TestField       TestFieldType
		OtherField      string
		unexportedField int
	}

	type OtherStruct struct {
		TestField TestFieldType
	}

	// Setup test validators
	testValidators := &config.TypeValidators{
		{
			TypeName: "model_test.TestStruct",
			Fields: config.FieldValidators{
				{
					FieldName: "TestField",
					Rules: []config.ValidationRule{
						{
							Type:          "enum",
							AllowedValues: []string{"valid_value1", "valid_value2"},
						},
					},
				},
			},
		},
	}

	// Set global validators
	model.SetGlobalTypeValidators(testValidators)

	defer model.SetGlobalTypeValidators(&config.TypeValidators{})

	tests := map[string]struct {
		structPtr any
		expectErr bool
		errMsg    string
	}{
		"Valid struct with valid field": {
			structPtr: &TestStruct{
				TestField:       TestFieldType("valid_value1"),
				unexportedField: 0, // irrelevant for validation, should be ignored
			},
			expectErr: false,
		},
		"Valid struct with invalid field": {
			structPtr: &TestStruct{
				TestField: TestFieldType("invalid_value"),
			},
			expectErr: true,
			errMsg:    "invalid field value",
		},
		"Struct with no validators configured": {
			structPtr: &OtherStruct{
				TestField: TestFieldType("any_value"),
			},
			expectErr: false,
		},
		"Non-struct pointer should return error": {
			structPtr: "not a struct",
			expectErr: true,
			errMsg:    "expected a pointer to struct",
		},
		"Non-pointer should return error": {
			structPtr: TestStruct{},
			expectErr: true,
			errMsg:    "expected a pointer to struct",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := model.ValidateStruct(test.structPtr)

			if test.expectErr {
				require.Error(t, err)

				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationContextFromType(t *testing.T) {
	type TestStruct struct {
		TestField TestFieldType
	}

	test := &TestStruct{TestField: TestFieldType("valid_value1")}

	// Setup test validators
	tests := map[string]struct {
		structPtr    any
		fieldPtr     any
		expectErr    bool
		expectErrMsg string
	}{
		"Valid struct and field": {
			structPtr: test,
			fieldPtr:  &test.TestField, // Pointer to string for field name
			expectErr: false,
		},
		"Non-pointer struct should return error": {
			structPtr:    *test,
			fieldPtr:     &test.TestField,
			expectErr:    true,
			expectErrMsg: "expected a pointer to struct",
		},
		"Non-struct pointer should return error": {
			structPtr:    new(int),
			fieldPtr:     new(string),
			expectErr:    true,
			expectErrMsg: "expected a pointer to struct",
		},
		"Non-pointer field should return error": {
			structPtr:    test,
			fieldPtr:     test.TestField,
			expectErr:    true,
			expectErrMsg: "expected a pointer to field",
		},
		"Field not in struct should return error": {
			structPtr:    test,
			fieldPtr:     new(string),
			expectErr:    true,
			expectErrMsg: "field is not a struct member",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, err := model.ValidationContextFromType(test.structPtr, test.fieldPtr)

			if test.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectErrMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, ctx)
			}
		})
	}
}
