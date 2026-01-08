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
	ErrKeyMissing      = errors.New("required key is missing")
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

// Validate checks if the provided value is a map with non-empyy key value pairs.
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

// NonEmptyValConstraint validates that all keys in a map have non-empty values.
type NonEmptyValConstraint struct{}

// Validate checks if the provided value is a map with non-empyy key value pairs.
func (n NonEmptyValConstraint) Validate(value any) error {
	mapValue, ok := value.(Map)
	if !ok {
		return fmt.Errorf("%w: %T", ErrWrongType, value)
	}

	for k, v := range mapValue.Map() {
		if v == "" {
			return fmt.Errorf("%w in key-value pair: '%s':'%v'", ErrValueEmpty, k, v)
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

// MapKeyConstraintSpec holds the specification for validating a single map key.
type MapKeyConstraintSpec struct {
	Name       string
	Required   bool
	Validators []Validator
}

// MapKeysConstraint validates map keys according to the provided specifications.
type MapKeysConstraint struct {
	Keys []MapKeyConstraintSpec
}

// NewMapKeysConstraint creates a new MapKeysConstraint from the provided key specifications.
func NewMapKeysConstraint(keys []MapKeySpec) (*MapKeysConstraint, error) {
	keySpecs := make([]MapKeyConstraintSpec, 0, len(keys))

	for _, key := range keys {
		if key.Name == "" {
			return nil, ErrConstraintKeyNameMissing
		}

		var validators []Validator
		if len(key.Constraints) > 0 {
			var err error
			validators, err = getValidators(key.Constraints)
			if err != nil {
				return nil, fmt.Errorf("invalid constraints for key %q: %w", key.Name, err)
			}
		}

		keySpecs = append(keySpecs, MapKeyConstraintSpec{
			Name:       key.Name,
			Required:   key.Required,
			Validators: validators,
		})
	}

	return &MapKeysConstraint{
		Keys: keySpecs,
	}, nil
}

// Validate checks if the provided map value satisfies all key constraints.
func (m *MapKeysConstraint) Validate(value any) error {
	mapValue, err := m.toStringMap(value)
	if err != nil {
		return err
	}

	for _, keySpec := range m.Keys {
		val, exists := mapValue[keySpec.Name]

		if !exists {
			if keySpec.Required {
				return fmt.Errorf("%w: %s", ErrKeyMissing, keySpec.Name)
			}
			continue
		}

		// Run nested validators if any
		for _, v := range keySpec.Validators {
			if err := v.Validate(val); err != nil {
				return fmt.Errorf("validation failed for key %q: %w", keySpec.Name, err)
			}
		}
	}

	return nil
}

// toStringMap converts the value to a map[string]string.
func (m *MapKeysConstraint) toStringMap(value any) (map[string]string, error) {
	switch v := value.(type) {
	case Map:
		result := make(map[string]string, len(v.Map()))
		for key, val := range v.Map() {
			strVal, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("%w: map value for key %q is not a string", ErrWrongType, key)
			}
			result[key] = strVal
		}
		return result, nil
	case map[string]string:
		return v, nil
	case map[string]any:
		result := make(map[string]string, len(v))
		for key, val := range v {
			strVal, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("%w: map value for key %q is not a string", ErrWrongType, key)
			}
			result[key] = strVal
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%w: expected map, got %T", ErrWrongType, value)
	}
}
