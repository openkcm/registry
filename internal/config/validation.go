package config

import (
	"fmt"
)

// ValidateField validates a value against field validation configuration.
func ValidateField(fieldName, value string, required bool) error {
	config := GetGlobalConfig()
	if config == nil {
		// No config set - fallback to basic validation
		if value == "" && required {
			return fmt.Errorf("%w: '%s'", ErrFieldIsRequired, fieldName)
		}

		return nil
	}

	return config.ValidateField(fieldName, value, required)
}
