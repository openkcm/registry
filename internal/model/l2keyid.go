package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// L2KeyID represents the L2KeyID of the system.
type L2KeyID string

// Validate validates given L2KeyID of the system.
func (l L2KeyID) Validate(_ ValidationContext) error {
	if l == "" {
		return status.Error(codes.InvalidArgument, "L2KeyID is empty")
	}

	return nil
}
