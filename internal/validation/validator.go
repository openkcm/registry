package validation

import (
	"errors"
	"fmt"
	"regexp"
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

// RegexConstraint validates that the string matches the configured regex patern.
type RegexConstraint struct {
	re *regexp.Regexp
}

// NewRegexConstraint takes a pattern and returns a RegexConstraint with the compiled regex patern.
func NewRegexConstraint(pattern string) (*RegexConstraint, error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	return &RegexConstraint{
		re: compiled,
	}, nil
}

// Validate checks if the provided value satisfies the regex constraint.
func (r *RegexConstraint) Validate(value any) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		if !r.re.MatchString(v) {
			return fmt.Errorf("%w: %s", ErrValueNotAllowed, v)
		}

	case []string:
		err := r.validateStringSlice(v)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("%w: %T", ErrWrongType, value)
	}

	return nil
}

// validateStringSlice validates the elements in the string slice against the regex validator.
func (r *RegexConstraint) validateStringSlice(v []string) error {
	if v == nil {
		return nil
	}
	if len(v) == 0 {
		return fmt.Errorf("%w: %v", ErrValueNotAllowed, v)
	}
	for _, s := range v {
		if !r.re.MatchString(s) {
			return fmt.Errorf("%w: %s", ErrValueNotAllowed, s)
		}
	}

	return nil
}
