package model

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/openkcm/registry/internal/config"
)

const (
	ValidatorsTagName = "validators"
)

const (
	InvalidFieldValueMsg         = "invalid field value"
	FieldValueMustNotBeEmptyMsg  = "field value must not be empty"
	NoPointerToStructMsg         = "expected a pointer to struct"
	NoPointerToFieldMsg          = "expected a pointer to field"
	FieldValueIsNotStringMsg     = "field value is not a string"
	FieldIsNotMapMsg             = "field is not a map"
	FieldIsNotArrayMsg           = "field is not an array or slice"
	FieldIsNotStructMemberMsg    = "field is not a struct member"
	FieldContainsEmptyKeysMsg    = "field contains empty keys"
	FieldContainsEmptyValuesMsg  = "field contains empty values"
	InvalidValidationRuleTypeMsg = "invalid validation rule type"
)

var (
	ErrInvalidFieldValue         = errors.New(InvalidFieldValueMsg)
	ErrFieldValueMustNotBeEmpty  = errors.New(FieldValueMustNotBeEmptyMsg)
	ErrNoPointerToStruct         = errors.New(NoPointerToStructMsg)
	ErrNoPointerToField          = errors.New(NoPointerToFieldMsg)
	ErrFieldValueIsNotString     = errors.New(FieldValueIsNotStringMsg)
	ErrFieldIsNotMap             = errors.New(FieldIsNotMapMsg)
	ErrFieldIsNotArray           = errors.New(FieldIsNotArrayMsg)
	ErrFieldIsNotStructMember    = errors.New(FieldIsNotStructMemberMsg)
	ErrFieldContainsEmptyKeys    = errors.New(FieldContainsEmptyKeysMsg)
	ErrFieldContainsEmptyValues  = errors.New(FieldContainsEmptyValuesMsg)
	ErrInvalidValidationRuleType = errors.New(InvalidValidationRuleTypeMsg)
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

// ClearGlobalTypeValidators clears the global validators.
func ClearGlobalTypeValidators() {
	SetGlobalTypeValidators(&config.TypeValidators{})
}

// CustomValidationRule is an interface that types can implement to provide custom validation logic.
type CustomValidationRule interface {
	ValidateCustomRule() error
}

// ValidateStruct validates a struct using reflection to automatically match field names
// with the configured validators.
// It will call the Validate method on all fields that implement the Validator interface.
// structPtr must be a pointer to a struct.
func ValidateStruct(structPtr any) error {
	structPtrValue := reflect.ValueOf(structPtr)
	if !isPointerToStruct(structPtrValue) {
		return fmt.Errorf("%w, got %T", ErrNoPointerToStruct, structPtr)
	}

	structValue := structPtrValue.Elem()
	structType := structValue.Type()
	typeName := structType.String()

	fieldValidators := getValidatorsForTypeName(typeName)

	return validateAllStructFields(structValue, structType, fieldValidators)
}

// ValidateField validates a specific field of a struct using reflection to match the field
// with the configured validators for the struct type.
// structPtr must be a pointer to a struct, and fieldPtr must be a pointer to a field within that struct.
// The function will apply validation rules configured for the field based on the struct type.
func ValidateField(structPtr any, fieldPtr any) error {
	structPtrValue := reflect.ValueOf(structPtr)
	if !isPointerToStruct(structPtrValue) {
		return fmt.Errorf("%w, got %T", ErrNoPointerToStruct, structPtr)
	}

	// Check if fieldPtr is a pointer
	fieldPtrValue := reflect.ValueOf(fieldPtr)
	if fieldPtrValue.Kind() != reflect.Ptr {
		return fmt.Errorf("%w, got %T", ErrNoPointerToField, fieldPtr)
	}

	structValue := structPtrValue.Elem()
	structType := structValue.Type()
	typeName := structType.String()

	var fieldName string
	for i := range structValue.NumField() {
		field := structValue.Field(i)
		if field.CanAddr() && field.Addr().Pointer() == fieldPtrValue.Pointer() {
			fieldName = getFieldName(structType.Field(i))
			break
		}
	}

	if fieldName == "" {
		return fmt.Errorf("%w struct: %s, field: %s", ErrFieldIsNotStructMember, structValue.String(), fieldPtrValue.String())
	}

	fieldValidators := getValidatorsForTypeName(typeName)

	fieldValidator := getFieldValidator(fieldValidators, fieldName)
	if fieldValidator == nil {
		// No validators configured for this field, nothing to do
		return nil
	}

	fieldValue := fieldPtrValue.Elem()

	if err := validateField(fieldValidator, fieldValue); err != nil {
		return err
	}
	return nil
}

// RegisterValidatorsForTypes registers validators for known model types that have 'validator' tags.
func RegisterValidatorsForTypes(types ...any) {
	validators := GetGlobalTypeValidators()
	if validators == nil {
		validators = &config.TypeValidators{}
	}

	for _, typ := range types {
		t := reflect.ValueOf(typ).Type()
		ruleMap := parseValidatorTags(t)

		// Add validators for model.Tenant based on tags
		addValidatorsForTypeAndRules(validators, t.String(), ruleMap)
	}
	SetGlobalTypeValidators(validators)
}

func isPointerToStruct(v reflect.Value) bool {
	return v.Kind() == reflect.Ptr && v.Elem().Kind() == reflect.Struct
}

func validateAllStructFields(structValue reflect.Value, structType reflect.Type, fieldValidators config.FieldValidators) error {
	for i := range structValue.NumField() {
		fieldValue := structValue.Field(i)
		fieldName := getFieldName(structType.Field(i))
		fieldValidator := getFieldValidator(fieldValidators, fieldName)

		if err := validateField(fieldValidator, fieldValue); err != nil {
			return err
		}
	}
	return nil
}

// ValidateField validates a value against the configured field validation rules.
func validateField(fieldValidator *config.FieldValidator, fieldValue reflect.Value) error {
	if fieldValidator == nil {
		return nil
	}

	if err := applyFieldValidationRules(fieldValue, *fieldValidator); err != nil {
		return err
	}

	return nil
}

// applyFieldValidationRules applies the validation rules to the field value.
//
//nolint:cyclop
func applyFieldValidationRules(fieldValue reflect.Value, validator config.FieldValidator) error {
	for _, rule := range validator.Rules {
		switch rule.Type {
		case config.RuleTypeEnum:
			err := applyRuleTypeEnum(fieldValue, validator, rule)
			if err != nil {
				return err
			}
		case config.RuleTypeNonEmpty:
			err := applyRuleTypeEmpty(fieldValue, validator)
			if err != nil {
				return err
			}
		case config.RuleTypeStringMap:
			err := applyRuleTypeStringMap(fieldValue, validator)
			if err != nil {
				return err
			}
		case config.RuleTypeStringArray:
			err := applyRuleTypeStringArray(fieldValue, validator)
			if err != nil {
				return err
			}
		case config.RuleTypeCustom:
			err := applyRuleTypeCustom(fieldValue, validator)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %s", ErrInvalidValidationRuleType, rule.Type)
		}
	}

	return nil
}

func applyRuleTypeStringArray(fieldValue reflect.Value, validator config.FieldValidator) error {
	if err := checkStringArrayHasNoEmptyValues(fieldValue, validator); err != nil {
		return err
	}
	return nil
}

func applyRuleTypeStringMap(fieldValue reflect.Value, validator config.FieldValidator) error {
	if err := checkStringMapHasNoEmptyKeysOrValues(fieldValue, validator); err != nil {
		return err
	}
	return nil
}

func applyRuleTypeEmpty(fieldValue reflect.Value, validator config.FieldValidator) error {
	if isEmpty(fieldValue) {
		return fmt.Errorf("%w: %s", ErrFieldValueMustNotBeEmpty, validator.FieldName)
	}
	return nil
}

func applyRuleTypeEnum(fieldValue reflect.Value, validator config.FieldValidator, rule config.ValidationRule) error {
	fieldValueAsString, err := getFieldValueAsString(fieldValue, validator.FieldName)
	if err != nil {
		return err
	}

	for _, allowedValue := range rule.AllowedValues {
		if fieldValueAsString == allowedValue {
			return nil
		}
	}

	return fmt.Errorf("%w: '%v' for field '%s', allowed values: %v",
		ErrInvalidFieldValue, fieldValue, validator.FieldName, rule.AllowedValues)
}

//nolint:cyclop
func getFieldValueAsString(fieldValue reflect.Value, fieldName string) (string, error) {
	switch fieldValue.Kind() { //nolint:exhaustive
	case reflect.String:
		return fieldValue.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(fieldValue.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(fieldValue.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", fieldValue.Float()), nil
	case reflect.Bool:
		return strconv.FormatBool(fieldValue.Bool()), nil
	case reflect.Ptr:
		if fieldValue.IsNil() {
			return "", nil
		}
		return getFieldValueAsString(fieldValue.Elem(), fieldName)
	case reflect.Interface:
		if fieldValue.IsNil() {
			return "", nil
		}
		return getFieldValueAsString(fieldValue.Elem(), fieldName)
	default:
		// Try to convert to string if possible
		if fieldValue.Type().ConvertibleTo(reflect.TypeOf("")) {
			return fieldValue.Convert(reflect.TypeOf("")).String(), nil
		}
		return "", fmt.Errorf("%w: %v for field %s", ErrFieldValueIsNotString, fieldValue, fieldName)
	}
}

func applyRuleTypeCustom(fieldValue reflect.Value, validator config.FieldValidator) error {
	customValidationRuleInterface := getCustomValidationRuleInterface(fieldValue)
	if customValidationRuleInterface != nil {
		if err := customValidationRuleInterface.ValidateCustomRule(); err != nil {
			return fmt.Errorf("%w: %s", err, validator.FieldName)
		}
	}
	return nil
}

func getCustomValidationRuleInterface(fieldValue reflect.Value) CustomValidationRule {
	// Skip unexported fields
	if !fieldValue.CanInterface() {
		return nil
	}

	// Handle pointer fields
	var customValidationRuleInterface CustomValidationRule
	if fieldValue.Kind() == reflect.Ptr {
		if !fieldValue.IsNil() {
			if v, ok := fieldValue.Interface().(CustomValidationRule); ok {
				customValidationRuleInterface = v
			}
		}
	}

	if customValidationRuleInterface == nil {
		// Handle non-pointer fields
		if v, ok := fieldValue.Interface().(CustomValidationRule); ok {
			customValidationRuleInterface = v
		}
	}

	if customValidationRuleInterface == nil {
		// Get the address of the field so we can validate it as a pointer
		if fieldValue.CanAddr() {
			addrValue := fieldValue.Addr()
			if v, ok := addrValue.Interface().(CustomValidationRule); ok {
				customValidationRuleInterface = v
			}
		}
	}

	return customValidationRuleInterface
}

func checkStringArrayHasNoEmptyValues(fieldValue reflect.Value, validator config.FieldValidator) error {
	var v reflect.Value
	if fieldValue.Kind() == reflect.Slice || fieldValue.Kind() == reflect.Array {
		v = fieldValue
	} else {
		return fmt.Errorf("%w: %s", ErrFieldIsNotArray, validator.FieldName)
	}

	for i := range v.Len() {
		str, err := getFieldValueAsString(v.Index(i), validator.FieldName)
		if err != nil {
			return err
		}
		if strings.ReplaceAll(str, " ", "") == "" {
			return fmt.Errorf("%w: %s", ErrFieldContainsEmptyValues, validator.FieldName)
		}
	}

	return nil
}

func checkStringMapHasNoEmptyKeysOrValues(fieldValue reflect.Value, validator config.FieldValidator) error {
	var v reflect.Value

	if fieldValue.Kind() == reflect.Map {
		v = fieldValue
	} else {
		return fmt.Errorf("%w: %s", ErrFieldIsNotMap, validator.FieldName)
	}

	for _, key := range v.MapKeys() {
		// Check for empty key
		keyStr, err := getFieldValueAsString(key, validator.FieldName)
		if err != nil {
			return err
		}

		if strings.ReplaceAll(keyStr, " ", "") == "" {
			return fmt.Errorf("%w: %s", ErrFieldContainsEmptyKeys, validator.FieldName)
		}

		// Check for empty value
		val := v.MapIndex(key)
		valStr, err := getFieldValueAsString(val, validator.FieldName)
		if err != nil {
			return err
		}

		if strings.ReplaceAll(valStr, " ", "") == "" {
			return fmt.Errorf("%w: %s", ErrFieldContainsEmptyValues, validator.FieldName)
		}
	}

	return nil
}

func getFieldValidator(fieldValidators config.FieldValidators, fieldName string) *config.FieldValidator {
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

// getValidatorsForTypeName returns FieldValidators for a specific type.
func getValidatorsForTypeName(typeName string) config.FieldValidators {
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

// addValidatorsForTypeAndRules adds validators for a specific type based on field tags.
func addValidatorsForTypeAndRules(validators *config.TypeValidators, typeName string, fieldRules map[string][]string) {
	typeValidator := findOrCreateTypeValidator(validators, typeName)

	// Add field validators based on tags
	for fieldName, rules := range fieldRules {
		// Find or create field validator
		var fieldValidator *config.FieldValidator
		for i := range typeValidator.Fields {
			if typeValidator.Fields[i].FieldName == fieldName {
				fieldValidator = &typeValidator.Fields[i]
				break
			}
		}
		if fieldValidator == nil {
			typeValidator.Fields = append(typeValidator.Fields, config.FieldValidator{
				FieldName: fieldName,
				Rules:     []config.ValidationRule{},
			})
			fieldValidator = &typeValidator.Fields[len(typeValidator.Fields)-1]
		}

		// Add rules that don't already exist
		for _, ruleName := range rules {
			hasRule := false
			for _, existingRule := range fieldValidator.Rules {
				if existingRule.Type == ruleName {
					hasRule = true
					break
				}
			}
			if !hasRule {
				fieldValidator.Rules = append([]config.ValidationRule{
					{Type: ruleName},
				}, fieldValidator.Rules...)
			}
		}
	}
}

func findOrCreateTypeValidator(validators *config.TypeValidators, typeName string) *config.TypeValidator {
	var typeValidator *config.TypeValidator
	for i := range *validators {
		if (*validators)[i].TypeName == typeName {
			typeValidator = &(*validators)[i]
			break
		}
	}
	if typeValidator == nil {
		*validators = append(*validators, config.TypeValidator{
			TypeName: typeName,
			Fields:   config.FieldValidators{},
		})
		typeValidator = &(*validators)[len(*validators)-1]
	}
	return typeValidator
}

// parseValidatorTags parses validator tags from a struct type using reflection
// This function can be used to automatically discover validator tags on struct fields.
func parseValidatorTags(structType reflect.Type) map[string][]string {
	fieldRules := make(map[string][]string)

	for i := range structType.NumField() {
		field := structType.Field(i)
		validatorTag := field.Tag.Get(ValidatorsTagName)
		if validatorTag != "" {
			// Simple parsing - split by comma for multiple rules
			var rules []string
			for _, rule := range splitAndTrim(validatorTag, ",") {
				if rule != "" {
					rules = append(rules, rule)
				}
			}
			if len(rules) > 0 {
				fieldRules[field.Name] = rules
			}
		}
	}

	return fieldRules
}
