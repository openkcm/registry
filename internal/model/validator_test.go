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
type TestMapType map[string]string
type TestArrayType [2]string
type TestSliceType []string
type TestIntType int

type TestCustomType struct {
	Value string
}

func (t TestCustomType) ValidateCustomRule() error {
	if t.Value == "invalid_value" {
		return model.ErrInvalidFieldValue
	}
	return nil
}
func TestValidateStruct(t *testing.T) {
	// Create a test struct
	type TestStruct struct {
		TestField       TestFieldType  `validators:"non-empty"`
		TestMap         TestMapType    `validators:"map"`
		TestArray       TestArrayType  `validators:"array"`
		TestSlice       TestSliceType  `validators:"array"`
		TestCustomType  TestCustomType `validators:"custom"`
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
	model.RegisterValidatorsForTypes(TestStruct{})
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		structPtr any
		expectErr bool
		errMsg    string
	}{
		"Valid struct with valid field": {
			structPtr: &TestStruct{
				TestField: TestFieldType("valid_value1"),
				TestArray: TestArrayType([]string{
					"valid_value1", "valid_value2"}),
				unexportedField: 0, // irrelevant for validation, should be ignored
			},
			expectErr: false,
		},
		"Valid struct with invalid field": {
			structPtr: &TestStruct{
				TestField: TestFieldType("invalid_value"),
				TestArray: TestArrayType([]string{
					"valid_value1", "valid_value2"}),
			},
			expectErr: true,
			errMsg:    model.InvalidFieldValueMsg,
		},
		"Struct with empty field": {
			structPtr: &TestStruct{
				TestField: TestFieldType(""),
				TestArray: TestArrayType([]string{
					"valid_value1", "valid_value2"}),
			},
			expectErr: true,
			errMsg:    "must not be empty",
		},
		"Struct with valid map": {
			structPtr: &TestStruct{
				TestField: TestFieldType("valid_value1"),
				TestMap: TestMapType{
					"valid_value1": "valid_value2",
				},
				TestArray: TestArrayType([]string{"valid_value1", "valid_value2"}),
			},
		},
		"Struct with invalid map key": {
			structPtr: &TestStruct{
				TestField: TestFieldType("valid_value1"),
				TestMap: TestMapType{
					"": "valid_value2",
				},
				TestArray: TestArrayType([]string{
					"valid_value1", "valid_value2"}),
			},
			expectErr: true,
		},
		"Struct with invalid map value": {
			structPtr: &TestStruct{
				TestField: TestFieldType("valid_value1"),
				TestMap: TestMapType{
					"valid_value1": "",
				},
				TestArray: TestArrayType([]string{
					"valid_value1", "valid_value2"}),
			},
			expectErr: true,
		},
		"Struct with valid array": {
			structPtr: &TestStruct{
				TestField: TestFieldType("valid_value1"),
				TestArray: TestArrayType{
					"valid_value1", "valid_value2",
				},
			},
		},
		"Struct with invalid array": {
			structPtr: &TestStruct{
				TestField: TestFieldType("valid_value1"),
				TestArray: TestArrayType{
					"valid_value1", "",
				},
			},
			expectErr: true,
		},
		"Struct with valid slice": {
			structPtr: &TestStruct{
				TestField: TestFieldType("valid_value1"),
				TestArray: TestArrayType{
					"valid_value1", "valid_value2",
				},
				TestSlice: TestSliceType{
					"valid_value1", "valid_value2",
				},
			},
		},
		"Struct with invalid slice": {
			structPtr: &TestStruct{
				TestField: TestFieldType("valid_value1"),
				TestArray: TestArrayType{
					"valid_value1", "valid_value2",
				},
				TestSlice: TestSliceType{
					"valid_value1", "",
				},
			},
			expectErr: true,
		},
		"Struct with invalid custom type": {
			structPtr: &TestStruct{
				TestField: TestFieldType("valid_value1"),
				TestCustomType: TestCustomType{
					Value: "invalid_value",
				},
			},
			expectErr: true,
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

func TestValidateField(t *testing.T) {
	// Create a test struct
	type TestStruct struct {
		TestField       TestFieldType `validators:"non-empty"`
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
	model.RegisterValidatorsForTypes(TestStruct{})
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		testFunc  func() (any, any)
		expectErr bool
		errMsg    string
	}{
		"Valid struct with valid field": {
			testFunc: func() (any, any) {
				t := &TestStruct{
					TestField:       TestFieldType("valid_value1"),
					unexportedField: 0, // irrelevant for validation, should be ignored
				}
				return t, &t.TestField
			},
			expectErr: false,
		},
		"Valid struct with invalid field": {
			testFunc: func() (any, any) {
				t := &TestStruct{
					TestField:       TestFieldType("invalid_value1"),
					unexportedField: 0, // irrelevant for validation, should be ignored
				}
				return t, &t.TestField
			},
			expectErr: true,
			errMsg:    model.InvalidFieldValueMsg,
		},
		"Struct with empty field": {
			testFunc: func() (any, any) {
				t := &TestStruct{
					TestField:       TestFieldType(""),
					unexportedField: 0, // irrelevant for validation, should be ignored
				}
				return t, &t.TestField
			},
			expectErr: true,
			errMsg:    "must not be empty",
		},
		"Struct with no validators configured": {
			testFunc: func() (any, any) {
				t := &OtherStruct{
					TestField: TestFieldType("any_value"),
				}
				return t, &t.TestField
			},
			expectErr: false,
		},
		"Non-struct pointer should return error": {
			testFunc: func() (any, any) {
				t := "not a struct"
				return t, nil
			},
			expectErr: true,
			errMsg:    "expected a pointer to struct",
		},
		"Non-pointer struct should return error": {
			testFunc: func() (any, any) {
				t := TestStruct{
					TestField:       TestFieldType("valid_value1"),
					unexportedField: 0, // irrelevant for validation, should be ignored
				}
				return t, &t.TestField
			},
			expectErr: true,
			errMsg:    "expected a pointer to struct",
		},
		"Non-pointer field should return error": {
			testFunc: func() (any, any) {
				t := &TestStruct{
					TestField:       TestFieldType("valid_value1"),
					unexportedField: 0, // irrelevant for validation, should be ignored
				}
				return t, t.TestField
			},
			expectErr: true,
			errMsg:    "expected a pointer to field",
		},
		"Non-member field should return error": {
			testFunc: func() (any, any) {
				t := &TestStruct{
					TestField:       TestFieldType("valid_value1"),
					unexportedField: 0, // irrelevant for validation, should be ignored
				}
				tf := TestFieldType("valid_value1")
				return t, &tf
			},
			expectErr: true,
			errMsg:    "field is not a struct member",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			structPtr, fieldPtr := test.testFunc()

			err := model.ValidateField(structPtr, fieldPtr)

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

func TestValidateFieldWithConvertibleTypes(t *testing.T) {
	// Create a test struct
	type TestStruct struct {
		TestField TestIntType `validators:"non-empty"`
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
							AllowedValues: []string{"1", "2", "3"},
						},
					},
				},
			},
		},
	}

	// Set global validators
	model.SetGlobalTypeValidators(testValidators)
	model.RegisterValidatorsForTypes(TestStruct{})
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		fieldValue TestIntType
		expectErr  bool
		errMsg     string
	}{
		"Valid int value": {
			fieldValue: TestIntType(1),
			expectErr:  false,
		},
		"Invalid int value": {
			fieldValue: TestIntType(4),
			expectErr:  true,
			errMsg:     model.InvalidFieldValueMsg,
		},
		"Zero int value": {
			fieldValue: TestIntType(0),
			expectErr:  true,
			errMsg:     model.FieldValueMustNotBeEmptyMsg,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			testStruct := &TestStruct{
				TestField: test.fieldValue, // initial value
			}
			err := model.ValidateField(testStruct, &testStruct.TestField)

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
func TestInvalidValidationRuleType(t *testing.T) {
	// given
	type TestStruct struct {
		TestField TestFieldType `validators:"invalid-rule"`
	}

	model.RegisterValidatorsForTypes(TestStruct{})
	defer model.ClearGlobalTypeValidators()

	// when
	err := model.ValidateStruct(&TestStruct{})

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid validation rule type: invalid-rule")
}

func TestInvalidSliceType(t *testing.T) {
	type TestStruct struct {
		TestSlice TestFieldType `validators:"array"`
	}
	model.RegisterValidatorsForTypes(TestStruct{})
	defer model.ClearGlobalTypeValidators()

	err := model.ValidateStruct(&TestStruct{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "field is not an array or slice: TestSlice")
}

func TestInvalidMapType(t *testing.T) {
	type TestStruct struct {
		TestMap TestFieldType `validators:"map"`
	}
	model.RegisterValidatorsForTypes(TestStruct{})
	defer model.ClearGlobalTypeValidators()

	err := model.ValidateStruct(&TestStruct{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "field is not a map: TestMap")
}
