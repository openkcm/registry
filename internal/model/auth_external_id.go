package model

import (
	"github.com/openkcm/registry/internal/validation"
)

const AuthExternalIDValidationID validation.ID = "Auth.ExternalID"

// AuthExternalID represents the external ID of the auth.
type AuthExternalID string

// String returns the string representation of the AuthExternalID.
func (a AuthExternalID) String() string {
	return string(a)
}

// Field returns the validation field for the AuthExternalID.
func (a AuthExternalID) Field() validation.StructField {
	return validation.StructField{
		ID: AuthExternalIDValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	}
}
