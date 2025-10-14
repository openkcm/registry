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

type Validator interface {
	Validate(value any) error
}

type String interface {
	String() string
}

type ListConstraint struct {
	AllowList []string `yaml:"allowList"`
}

func (l ListConstraint) Validate(value any) error {
	strValue, err := getString(value)
	if err != nil {
		return err
	}

	if !slices.Contains(l.AllowList, strValue) {
		return fmt.Errorf("%w: %s", ErrValueNotAllowed, strValue)
	}

	return nil
}

type NonEmptyConstraint struct{}

func (n NonEmptyConstraint) Validate(value any) error {
	strValue, err := getString(value)
	if err != nil {
		return err
	}

	if strValue == "" {
		return ErrValueEmpty
	}

	return nil
}

type NonEmptyKeysConstraint struct{}

func (n NonEmptyKeysConstraint) Validate(value any) error {
	mapValue, ok := value.(Map)
	if !ok {
		return fmt.Errorf("%w: %T", ErrWrongType, value)
	}

	for k := range mapValue.Map() {
		if k == "" {
			return ErrKeyEmpty
		}
	}
	return nil
}

func getString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case String:
		return v.String(), nil
	default:
		return "", fmt.Errorf("%w: %T", ErrWrongType, value)
	}
}
