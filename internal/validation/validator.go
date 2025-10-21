package validation

import (
	"errors"
	"fmt"
	"slices"
)

var (
	ErrWrongType       = errors.New("value has wrong type")
	ErrValueNotAllowed = errors.New("value is not allowed")
	ErrValueEmpty      = errors.New("value is empty")
	ErrKeyEmpty        = errors.New("key is empty")
)

// Validator defines the interface for constraints.
type Validator interface {
	Validate(value any) error
}

// ListConstraint validates that a value is within an allowed list.
type ListConstraint struct {
	AllowList []string `yaml:"allowList"`
}

// Validate checks if the provided value is in the AllowList.
func (l ListConstraint) Validate(value any) error {
	strValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("%w: %T", ErrWrongType, value)
	}

	if !slices.Contains(l.AllowList, strValue) {
		return fmt.Errorf("%w: %s", ErrValueNotAllowed, strValue)
	}

	return nil
}

// NonEmptyConstraint validates that a string value is not empty.
type NonEmptyConstraint struct{}

// Validate checks if the provided value is a non-empty string.
func (n NonEmptyConstraint) Validate(value any) error {
	strValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("%w: %T", ErrWrongType, value)
	}

	if strValue == "" {
		return ErrValueEmpty
	}

	return nil
}

// NonEmptyKeysConstraint validates that all keys in a map are non-empty strings.
type NonEmptyKeysConstraint struct{}

// Validate checks if the provided value is a map with non-empty keys.
func (n NonEmptyKeysConstraint) Validate(value any) error {
	mapValue, ok := value.(Map)
	if !ok {
		return fmt.Errorf("%w: %T", ErrWrongType, value)
	}

	for k, v := range mapValue.Map() {
		if k == "" {
			return fmt.Errorf("%w in key-value pair: '%s':'%v'", ErrKeyEmpty, k, v)
		}
	}
	return nil
}
