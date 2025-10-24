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
	// Config represents the validation configuration.
	Config struct {
		// Fields represents configuration fields.
		Fields []ConfigField
		// Models represents models to extract validations from and check for ID existence.
		Models []Model
	}

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
func New(cfg Config) (*Validation, error) {
	v := &Validation{
		byID: make(map[ID]Spec),
	}
	err := v.registerConfig(cfg.Fields...)
	if err != nil {
		return nil, err
	}
	for _, model := range cfg.Models {
		err := v.register(model.Validations()...)
		if err != nil {
			return nil, err
		}
	}
	err = v.checkIDExists(cfg.Models...)
	if err != nil {
		return nil, err
	}

	return v, nil
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

// registerConfig registers configuration fields into the Validation instance.
func (v *Validation) registerConfig(fields ...ConfigField) error {
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

// register registers validation fields into the Validation instance.
func (v *Validation) register(fields ...Field) error {
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

// checkIDExists checks if all registered IDs exist in the provided models.
func (v *Validation) checkIDExists(models ...Model) error {
	sources := make([]map[ID]struct{}, 0, len(models))
	for _, input := range models {
		ids, err := getIDs(input)
		if err != nil {
			return err
		}
		sources = append(sources, ids)
	}
	return v.checkIDs(sources...)
}

// checkIDs checks if all registered IDs exist in the provided sources.
// If an ID is marked with SkipIfNotExists, it will be skipped during the check.
func (v *Validation) checkIDs(sources ...map[ID]struct{}) error {
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
