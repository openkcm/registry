package model

import (
	"github.com/openkcm/registry/internal/validation"
)

const AuthTypeValidationID validation.ID = "Auth.Type"

// AuthType represents the type of auth.
type AuthType string

// String returns the string representation of the AuthType.
func (a AuthType) String() string {
	return string(a)
}

// Field returns the validation field for the AuthType.
func (a AuthType) Field() validation.StructField {
	return validation.StructField{
		ID: AuthTypeValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	}
}
