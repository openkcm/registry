package validation

import (
	"errors"
	"fmt"
	"sync"
)

var ErrMissingID = errors.New("id does not exist")

type (
	Validation struct {
		byID map[ID]Spec
		mu   sync.RWMutex
	}

	ID   string
	Spec struct {
		omitIDCheck bool
		validators  []Validator
	}
)

func New(fields ...ConfigField) (*Validation, error) {
	v := &Validation{
		byID: make(map[ID]Spec),
	}
	err := v.AddConfigFields(fields...)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (v *Validation) AddConfigFields(fields ...ConfigField) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, field := range fields {
		validators, err := getValidators(field.Constraints)
		if err != nil {
			return err
		}
		spec, ok := v.byID[field.ID]
		if !ok {
			v.byID[field.ID] = Spec{
				omitIDCheck: field.OmitIDCheck,
				validators:  validators,
			}
			continue
		}
		spec.omitIDCheck = spec.omitIDCheck || field.OmitIDCheck
		spec.validators = append(spec.validators, validators...)
		v.byID[field.ID] = spec
	}

	return nil
}

func (v *Validation) AddStructFields(fields ...StructField) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, field := range fields {
		spec, ok := v.byID[field.ID]
		if !ok {
			v.byID[field.ID] = Spec{
				validators: field.Validators,
			}
			continue
		}
		spec.omitIDCheck = false
		spec.validators = append(spec.validators, field.Validators...)
		v.byID[field.ID] = spec
	}
}

func (v *Validation) CheckIDs(sources ...map[ID]struct{}) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	for id, spec := range v.byID {
		if spec.omitIDCheck {
			continue
		}

		exists := false
		for _, source := range sources {
			_, ok := source[id]
			if ok {
				exists = true
				break
			}
		}
		if !exists {
			return fmt.Errorf("%w: %s", ErrMissingID, id)
		}
	}
	return nil
}

func (v *Validation) ValidateAll(valuesByID map[ID]any) error {
	for id, value := range valuesByID {
		err := v.Validate(id, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Validation) Validate(id ID, value any) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	spec, ok := v.byID[id]
	if !ok {
		return nil
	}

	for _, v := range spec.validators {
		err := v.Validate(value)
		if err != nil {
			return fmt.Errorf("validation failed for %s: %w", id, err)
		}
	}

	return nil
}

func getValidators(constraints []Constraint) ([]Validator, error) {
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
