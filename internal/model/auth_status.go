package model

import (
	"fmt"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"

	"github.com/openkcm/registry/internal/validation"
)

const AuthStatusValidationID validation.ID = "Auth.Status"

var validAuthStatuses = map[AuthStatus]struct{}{}

// init calculates valid tenant roles.
func init() {
	for _, v := range pb.AuthStatus_name {
		if v != pb.AuthStatus_AUTH_STATUS_UNSPECIFIED.String() {
			validAuthStatuses[AuthStatus(v)] = struct{}{}
		}
	}
}

// AuthStatus represents the status of auth.
type AuthStatus string

// String returns the string representation of the AuthStatus.
func (a AuthStatus) String() string {
	return string(a)
}

// Validate validates given status of the auth.
func (a AuthStatus) Validate(value any) error {
	statusValue, ok := value.(AuthStatus)
	if !ok {
		return fmt.Errorf("%w: %T", validation.ErrValueNotAllowed, value)
	}

	if _, ok := validAuthStatuses[statusValue]; !ok {
		return validation.ErrValueNotAllowed
	}

	return nil
}

// Field returns the validation field for the AuthStatus.
func (a AuthStatus) Field() validation.StructField {
	return validation.StructField{
		ID:         AuthStatusValidationID,
		Validators: []validation.Validator{a},
	}
}
