package model

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/registry/internal/config"
)

var (
	ErrInvalidFieldValue = errors.New("invalid field value")
	ErrNoPointerToStruct = errors.New("expected a pointer to struct")
)

// GlobalTypeValidators holds the validation configuration.
type GlobalTypeValidators struct {
	validators *config.TypeValidators
	mu         sync.RWMutex
}

var globalValidators = &GlobalTypeValidators{}

// SetGlobalTypeValidators sets the configuration.
func SetGlobalTypeValidators(validators *config.TypeValidators) {
	globalValidators.mu.Lock()
	defer globalValidators.mu.Unlock()

	globalValidators.validators = validators
}

// GetGlobalTypeValidators returns the configuration.
func GetGlobalTypeValidators() *config.TypeValidators {
	globalValidators.mu.RLock()
	defer globalValidators.mu.RUnlock()

	return globalValidators.validators
}

// Validator defines the methods for validation.
type Validator interface {
	Validate() error
}

// ValidateAll goes through the given validators and calls their Validate method.
// It stops and returns at the first error encountered, if any. If all validate successfully, it returns nil.
func ValidateAll(v ...Validator) error {
	for i := range v {
		err := v[i].Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// ValidateStruct validates a struct using reflection to automatically match field names
// with the configured validators. It extracts field names from GORM column tags.
// If no specific validators are configured for a type, it falls back to validating all fields.
func ValidateStruct(structPtr interface{}, typeName string) error {
	v := reflect.ValueOf(structPtr)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("%w, got %T", ErrNoPointerToStruct, structPtr)
	}

	structValue := v.Elem()
	structType := structValue.Type()

	err := applyConfiguredFieldValidators(typeName, structType, structValue)
	if err != nil {
		return err
	}

	return invokeValidators(structType, structValue)
}

// applyConfiguredFieldValidators applies field validators based on the configuration.
func applyConfiguredFieldValidators(typeName string, structType reflect.Type, structValue reflect.Value) error {
	// Get validators for this specific type
	typeValidators := getValidatorsForType(typeName)

	for i := range structValue.NumField() {
		field := structType.Field(i)
		fieldValue := structValue.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Extract field name from GORM tag
		fieldName := extractFieldNameFromTag(field)
		if fieldName == "" {
			continue
		}

		// Find validator for this specific field
		if validator := getFieldValidator(fieldName, typeValidators); validator != nil {
			if err := validateField(validator, fieldName, fieldValue); err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
		}
	}

	return nil
}

// invokeValidators calls the Validate method on all fields that implement the Validator interface.
func invokeValidators(structType reflect.Type, structValue reflect.Value) error {
	// Call Validate on all fields that implement Validator interface
	var validators []Validator

	for i := range structValue.NumField() {
		fieldValue := structValue.Field(i)
		fieldType := structType.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Handle pointer fields
		if fieldValue.Kind() == reflect.Ptr {
			if !fieldValue.IsNil() {
				if validatorInterface, ok := fieldValue.Interface().(Validator); ok {
					validators = append(validators, validatorInterface)
				}
			}
		} else {
			// Handle non-pointer fields
			if validatorInterface, ok := fieldValue.Interface().(Validator); ok {
				validators = append(validators, validatorInterface)
			}

			// Special handling for Labels field - need to get its address
			if fieldType.Name == "Labels" {
				// Get the address of the field so we can validate it as a pointer
				if fieldValue.CanAddr() {
					addrValue := fieldValue.Addr()
					if validatorInterface, ok := addrValue.Interface().(Validator); ok {
						validators = append(validators, validatorInterface)
					}
				}
			}
		}
	}

	return ValidateAll(validators...)
}

// ValidateField validates a value against the configured field validation rules.
func validateField(fieldValidator *config.FieldValidator, fieldName string, fieldValue reflect.Value) error {
	if fieldValidator == nil {
		return nil
	}

	if err := applyFieldValidationRules(fieldName, fieldValue, *fieldValidator); err != nil {
		return err
	}

	return nil
}

func applyFieldValidationRules(fieldName string, fieldValue reflect.Value, validator config.FieldValidator) error {
	fieldValueAsString := fieldValue.String()
	for _, rule := range validator.Rules {
		if rule.Type == config.RuleTypeEnum {
			for _, allowedValue := range rule.AllowedValues {
				if fieldValueAsString == allowedValue {
					return nil
				}
			}

			return fmt.Errorf("%w: '%s' for field '%s', allowed values: %v",
				ErrInvalidFieldValue, fieldValue, fieldName, rule.AllowedValues)
		}
	}

	return nil
}

// getFieldValidator returns the field validation for a given field name.
func getFieldValidator(fieldName string, fieldValidators config.FieldValidators) *config.FieldValidator {
	for _, fieldValidator := range fieldValidators {
		if fieldValidator.FieldName == fieldName {
			return &fieldValidator
		}
	}

	return nil
}

// extractFieldNameFromTag extracts the column name from GORM tag or falls back to field name.
func extractFieldNameFromTag(field reflect.StructField) string {
	gormTag := field.Tag.Get("gorm")
	if gormTag == "" {
		return ""
	}

	// Parse GORM tag to find column name
	parts := strings.Split(gormTag, ";")
	for _, part := range parts {
		if strings.HasPrefix(part, "column:") {
			columnName := strings.TrimPrefix(part, "column:")
			return columnName
		}
	}

	return ""
}

// getValidatorsForType returns validators for a specific type.
func getValidatorsForType(typeName string) config.FieldValidators {
	// Get global validators and filter by type
	globalValidators := GetGlobalTypeValidators()
	if globalValidators == nil {
		return nil
	}

	if typeValidators, exists := (*globalValidators)[typeName]; exists {
		return typeValidators
	}

	return nil
}
