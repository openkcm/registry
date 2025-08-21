package model

// Validator defines the methods for validation.
type Validator interface {
	Validate() error
}

// ValidateAll goes through the given validators and calls their Validate method.
// It stops and returns at the first error encountered, if any. If all validate successfully, it returns nil.
func ValidateAll(v ...Validator) error {
	for i := range v {
		err := v[i].Validate()
		if err != nil {
			return err
		}
	}

	return nil
}
