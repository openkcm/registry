package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/openkcm/registry/internal/validation"
)

const AuthPropertiesValidationID validation.ID = "Auth.Properties"

// AuthProperties represents the properties of auth.
type AuthProperties map[string]string //nolint:recvcheck

var _ validation.Map = AuthProperties{}

// Value implements the driver.Valuer interface.
func (a AuthProperties) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil //nolint:nilnil
	}
	return json.Marshal(a)
}

// Scan implements the sql.Scanner interface.
func (a *AuthProperties) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}

	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("%w: %v", ErrMarshalUserGroupValue, src)
	}

	return json.Unmarshal(bytes, a)
}

// Field returns the validation field for the AuthProperties.
func (a AuthProperties) Field() validation.StructField {
	return validation.StructField{
		ID: AuthPropertiesValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyKeysConstraint{},
		},
	}
}

// Map converts AuthProperties to a map[string]any.
// This is used by the validation package.
func (a AuthProperties) Map() map[string]any {
	res := make(map[string]any, len(a))
	for k, v := range a {
		res[k] = v
	}
	return res
}
