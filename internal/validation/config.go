package validation

import (
	"errors"
	"fmt"
)

const (
	ConstraintTypeList         = "list"
	ConstraintTypeNonEmpty     = "non-empty"
	ConstraintTypeNonEmptyKeys = "non-empty-keys"
	ConstraintTypeNonEmptyVals = "non-empty-vals"
	ConstraintTypeRegex        = "regex"
	ConstraintTypeMapKeys      = "map-keys"
)

var (
	ErrConstraintsMissing         = errors.New("no constraints provided")
	ErrEmptyConstraintType        = errors.New("constraint type is empty")
	ErrUnknownConstraintType      = errors.New("unknown constraint type")
	ErrConstraintSpecMissing      = errors.New("constraint spec is missing")
	ErrConstraintAllowListMissing = errors.New("constraint allow list is missing")
	ErrConstraintPatternMissing   = errors.New("constraint pattern is missing")
	ErrConstraintKeysMissing      = errors.New("constraint keys are missing")
	ErrConstraintKeyNameMissing   = errors.New("constraint key name is missing")
)

type (
	// ConfigField represents a configuration field with its validation constraints.
	// If the ID is not defined via `TagName`,
	// SkipIfNotExists needs to be set to true.
	ConfigField struct {
		ID              ID           `yaml:"id"`
		SkipIfNotExists bool         `yaml:"skipIfNotExists,omitempty"`
		Constraints     []Constraint `yaml:"constraints"`
	}

	// Constraint represents a validation constraint for a configuration field.
	Constraint struct {
		Type string          `yaml:"type"`
		Spec *ConstraintSpec `yaml:"spec,omitempty"`
	}

	// ConstraintSpec holds the specification for a constraint.
	ConstraintSpec struct {
		AllowList []string     `yaml:"allowList,omitempty"`
		Pattern   string       `yaml:"pattern,omitempty"`
		Keys      []MapKeySpec `yaml:"keys,omitempty"`
	}

	// MapKeySpec holds the specification for a map key constraint.
	MapKeySpec struct {
		Name        string       `yaml:"name"`
		Required    bool         `yaml:"required,omitempty"`
		Constraints []Constraint `yaml:"constraints,omitempty"`
	}
)

//nolint:cyclop
func (c Constraint) getValidator() (Validator, error) {
	switch c.Type {
	case "":
		return nil, ErrEmptyConstraintType
	case ConstraintTypeList:
		if c.Spec == nil {
			return nil, ErrConstraintSpecMissing
		}
		if len(c.Spec.AllowList) == 0 {
			return nil, ErrConstraintAllowListMissing
		}
		return ListConstraint{
			AllowList: c.Spec.AllowList,
		}, nil
	case ConstraintTypeNonEmpty:
		return NonEmptyConstraint{}, nil
	case ConstraintTypeNonEmptyKeys:
		return NonEmptyKeysConstraint{}, nil
	case ConstraintTypeNonEmptyVals:
		return NonEmptyValConstraint{}, nil
	case ConstraintTypeRegex:
		if c.Spec == nil {
			return nil, ErrConstraintSpecMissing
		}
		if len(c.Spec.Pattern) == 0 {
			return nil, ErrConstraintPatternMissing
		}
		return NewRegexConstraint(c.Spec.Pattern)
	case ConstraintTypeMapKeys:
		if c.Spec == nil {
			return nil, ErrConstraintSpecMissing
		}
		if len(c.Spec.Keys) == 0 {
			return nil, ErrConstraintKeysMissing
		}
		return NewMapKeysConstraint(c.Spec.Keys)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownConstraintType, c.Type)
	}
}

func getValidators(constraints []Constraint) ([]Validator, error) {
	if len(constraints) == 0 {
		return nil, ErrConstraintsMissing
	}

	v := make([]Validator, 0, len(constraints))
	for _, c := range constraints {
		cv, err := c.getValidator()
		if err != nil {
			return nil, err
		}
		v = append(v, cv)
	}
	return v, nil
}
