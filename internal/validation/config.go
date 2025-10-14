package validation

import (
	"errors"
	"fmt"
)

const (
	ConstraintTypeList         = "list"
	ConstraintTypeNonEmpty     = "non-empty"
	ConstraintTypeNonEmptyKeys = "non-empty-keys"
)

var (
	ErrUnkownConstraintType       = errors.New("unknown constraint type")
	ErrConstraintSpecMissing      = errors.New("constraint spec is missing")
	ErrConstraintAllowListMissing = errors.New("constraint allow list is missing")
)

type (
	ConfigField struct {
		ID          ID           `yaml:"id"`
		OmitIDCheck bool         `yaml:"omitIdCheck,omitempty"`
		Constraints []Constraint `yaml:"constraints"`
	}

	Constraint struct {
		Type string          `yaml:"type"`
		Spec *ConstraintSpec `yaml:"spec,omitempty"`
	}

	ConstraintSpec struct {
		AllowList []string `yaml:"allowList,omitempty"`
	}
)

func (c Constraint) getValidator() (Validator, error) {
	switch c.Type {
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
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnkownConstraintType, c.Type)
	}
}
