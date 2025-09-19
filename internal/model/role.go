package model

import (
	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
)

// Role represents the role of the tenant.
type Role string

// validRoles contains all valid tenant roles. Calculated in the init().
var (
	validRoles = map[Role]struct{}{}
)

// init calculates valid tenant roles.
func init() {
	for _, v := range pb.Role_name {
		if v != pb.Role_ROLE_UNSPECIFIED.String() {
			validRoles[Role(v)] = struct{}{}
		}
	}
}

// ValidateCustomRule implements CustomValidationRule interface.
func (r Role) ValidateCustomRule() error {
	if r == "" {
		return ErrFieldValueMustNotBeEmpty
	}
	if _, ok := validRoles[r]; !ok {
		return ErrInvalidFieldValue
	}

	return nil
}
