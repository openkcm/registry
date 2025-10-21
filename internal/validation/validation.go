package validation

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrEmptyID           = errors.New("id is empty")
	ErrValidatorsMissing = errors.New("no validators provided")
	ErrIDMustExist       = errors.New("id must exist")
)

type (
	// Validation represents a map of validation specifications by their IDs.
	Validation struct {
		byID map[ID]Spec
		mu   sync.RWMutex
	}

	// ID represents a validation identifier.
	ID string

	// Spec represents the validation specification for a given ID.
	Spec struct {
		skipIfNotExists bool
		validators      []Validator
	}
)

// New creates a new Validation instance with the provided configuration fields.
func New(fields ...ConfigField) (*Validation, error) {
	v := &Validation{
		byID: make(map[ID]Spec),
	}
	err := v.RegisterConfig(fields...)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// RegisterConfig registers configuration fields into the Validation instance.
func (v *Validation) RegisterConfig(fields ...ConfigField) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, field := range fields {
		if field.ID == "" {
			return ErrEmptyID
		}

		validators, err := getValidators(field.Constraints)
		if err != nil {
			return err
		}
		spec, ok := v.byID[field.ID]
		if !ok {
			v.byID[field.ID] = Spec{
				skipIfNotExists: field.SkipIfNotExists,
				validators:      validators,
			}
			continue
		}
		spec.skipIfNotExists = spec.skipIfNotExists && field.SkipIfNotExists
		spec.validators = append(spec.validators, validators...)
		v.byID[field.ID] = spec
	}

	return nil
}

// Register registers validation fields into the Validation instance.
func (v *Validation) Register(fields ...Field) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, field := range fields {
		if field.ID == "" {
			return ErrEmptyID
		}
		if len(field.Validators) == 0 {
			return ErrValidatorsMissing
		}

		spec, ok := v.byID[field.ID]
		if !ok {
			v.byID[field.ID] = Spec{
				validators: field.Validators,
			}
			continue
		}
		spec.skipIfNotExists = false
		spec.validators = append(spec.validators, field.Validators...)
		v.byID[field.ID] = spec
	}

	return nil
}

// CheckIDs checks if all registered IDs exist in the provided sources.
// A source can be created using GetIDs function.
// If an ID is marked with SkipIfNotExists, it will be skipped during the check.
func (v *Validation) CheckIDs(sources ...map[ID]struct{}) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	for id, spec := range v.byID {
		if spec.skipIfNotExists {
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
			return fmt.Errorf("%w: %s", ErrIDMustExist, id)
		}
	}
	return nil
}

// ValidateAll validates all provided values mapped by their IDs.
func (v *Validation) ValidateAll(valuesByID map[ID]any) error {
	for id, value := range valuesByID {
		err := v.Validate(id, value)
		if err != nil {
			return err
		}
	}
	return nil
}

// Validate validates a single value by its ID.
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
