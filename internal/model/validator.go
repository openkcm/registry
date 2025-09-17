package model

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/registry/internal/config"
)

var (
	ErrInvalidFieldValue      = errors.New("invalid field value")
	ErrNoPointerToStruct      = errors.New("expected a pointer to struct")
	ErrNoPointerToField       = errors.New("expected a pointer to field")
	ErrFieldValueIsNotString  = errors.New("field value is not a string")
	ErrFieldIsNotStructMember = errors.New("field is not a struct member")
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

// ValidationContext provides context for field validation.
type ValidationContext interface {
	ValidateField(fieldValue any) error
}

// EmptyValidationContext is global ValidationContext that performs no validation.
// Used for types that do not support configurable validation.
var EmptyValidationContext = EmptyValidationCtx{}

// EmptyValidationCtx is a ValidationContext that performs no validation.
type EmptyValidationCtx struct {
	ValidationContext
}

// ValidateField always returns nil, performing no validation.
func (EmptyValidationCtx) ValidateField(_ any) error {
	return nil
}

// Validator defines the methods for validation.
type Validator interface {
	Validate(ctx ValidationContext) error
}

// ValidateStruct validates a struct using reflection to automatically match field names
// with the configured validators.
// It will call the Validate method on all fields that implement the Validator interface.
// structPtr must be a pointer to a struct.
func ValidateStruct(structPtr any) error {
	v := reflect.ValueOf(structPtr)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("%w, got %T", ErrNoPointerToStruct, structPtr)
	}

	structValue := v.Elem()
	structType := structValue.Type()

	return invokeValidators(structType, structValue)
}

// invokeValidators calls the Validate method on all fields that implement the Validator interface.
func invokeValidators(structType reflect.Type, structValue reflect.Value) error {
	typeName := structType.String()

	fieldValidators := getValidatorsForType(typeName)

	for i := range structValue.NumField() {
		fieldValue := structValue.Field(i)
		fieldType := structType.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Get the Validator interface if the field implements it
		validatorInterface := getValidatorInterface(fieldValue)

		validationContext := FieldValidationContext{}
		if validatorInterface != nil {
			fieldName := getFieldName(fieldType)

			// Find validator for this specific field
			if validator := getFieldValidator(fieldName, fieldValidators); validator != nil {
				validationContext.fieldValidator = *validator
			}
		}

		if validatorInterface != nil {
			err := validatorInterface.Validate(validationContext)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getValidatorInterface(fieldValue reflect.Value) Validator {
	// Skip unexported fields
	if !fieldValue.CanInterface() {
		return nil
	}

	// Handle pointer fields
	var validatorInterface Validator
	if fieldValue.Kind() == reflect.Ptr {
		if !fieldValue.IsNil() {
			if v, ok := fieldValue.Interface().(Validator); ok {
				validatorInterface = v
			}
		}
	}

	if validatorInterface == nil {
		// Handle non-pointer fields
		if v, ok := fieldValue.Interface().(Validator); ok {
			validatorInterface = v
		}
	}

	if validatorInterface == nil {
		// Get the address of the field so we can validate it as a pointer
		if fieldValue.CanAddr() {
			addrValue := fieldValue.Addr()
			if v, ok := addrValue.Interface().(Validator); ok {
				validatorInterface = v
			}
		}
	}

	return validatorInterface
}

// FieldValidationContext provides context for field validation.
type FieldValidationContext struct {
	ValidationContext

	fieldValidator config.FieldValidator
}

// ValidateField validates a field value against the configured field validation rules.
func (v FieldValidationContext) ValidateField(fieldValue any) error {
	if err := validateField(&v.fieldValidator, fieldValue); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return nil
}

// ValidationContextFromType creates a ValidationContext for a specific struct type and field.
// structPtr must be a pointer to a struct.
// fieldPtr must be a pointer to a field within the struct.
func ValidationContextFromType(structPtr any, fieldPtr any) (ValidationContext, error) {
	v := reflect.ValueOf(structPtr)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w, got %T", ErrNoPointerToStruct, structPtr)
	}

	structValue := v.Elem()
	structType := structValue.Type()
	typeName := structType.String()
	fieldPtrValue := reflect.ValueOf(fieldPtr)

	// Get validators for this specific type
	fieldValidators := getValidatorsForType(typeName)

	context, err := getValidationContextForFieldValue(structValue, fieldPtrValue, fieldValidators)
	if err != nil {
		return nil, err
	}
	return context, nil
}

// getValidationContextForFieldValue finds the field in the struct and returns a ValidationContext for it.
func getValidationContextForFieldValue(structValue reflect.Value, fieldPtrValue reflect.Value, fieldValidators config.FieldValidators) (ValidationContext, error) {
	if fieldPtrValue.Kind() == reflect.Ptr {
		validationContext := FieldValidationContext{}
		structType := structValue.Type()

		for i := range structValue.NumField() {
			field := structValue.Field(i)
			if field.CanAddr() && field.Addr().Pointer() == fieldPtrValue.Pointer() {
				fieldName := structType.Field(i).Name
				if validator := getFieldValidator(fieldName, fieldValidators); validator != nil {
					validationContext.fieldValidator = *validator
				}
				return validationContext, nil
			}
		}

		return nil, fmt.Errorf("%w struct: %s, field: %s", ErrFieldIsNotStructMember, structValue.String(), fieldPtrValue.String())
	}

	return nil, fmt.Errorf("%w: %T", ErrNoPointerToField, fieldPtrValue.String())
}

// ValidateField validates a value against the configured field validation rules.
func validateField(fieldValidator *config.FieldValidator, fieldValue any) error {
	if fieldValidator == nil {
		return nil
	}

	if err := applyFieldValidationRules(fieldValue, *fieldValidator); err != nil {
		return err
	}

	return nil
}

// applyFieldValidationRules applies the validation rules to the field value.
func applyFieldValidationRules(fieldValue any, validator config.FieldValidator) error {
	fieldValueAsString, ok := fieldValue.(string)
	if !ok {
		return fmt.Errorf("%w: %v", ErrFieldValueIsNotString, fieldValue)
	}
	for _, rule := range validator.Rules {
		if rule.Type == config.RuleTypeEnum {
			for _, allowedValue := range rule.AllowedValues {
				if fieldValueAsString == allowedValue {
					return nil
				}
			}

			return fmt.Errorf("%w: '%s' for field '%s', allowed values: %v",
				ErrInvalidFieldValue, fieldValue, validator.FieldName, rule.AllowedValues)
		}
	}

	return nil
}

// getFieldValidator returns the FieldValidator for a given field name.
func getFieldValidator(fieldName string, fieldValidators config.FieldValidators) *config.FieldValidator {
	for _, fieldValidator := range fieldValidators {
		if fieldValidator.FieldName == fieldName {
			return &fieldValidator
		}
	}

	return nil
}

func getFieldName(fieldType reflect.StructField) string {
	return fieldType.Name
}

// getValidatorsForType returns FieldValidators for a specific type.
func getValidatorsForType(typeName string) config.FieldValidators {
	// Get global validators and filter by type
	globalValidators := GetGlobalTypeValidators()
	if globalValidators == nil {
		return nil
	}

	for _, typeValidator := range *globalValidators {
		if typeValidator.TypeName == typeName {
			return typeValidator.Fields
		}
	}

	return nil
}
