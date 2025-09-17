package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

// Validate validates given role of the tenant.
func (r Role) Validate(_ ValidationContext) error {
	if _, ok := validRoles[r]; !ok {
		return status.Error(codes.InvalidArgument, "Role is not correct")
	}

	return nil
}
