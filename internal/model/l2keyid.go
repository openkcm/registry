package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// L2KeyID represents the L2KeyID of the system.
type L2KeyID string

// Validate validates given L2KeyID of the system.
func (l L2KeyID) Validate(ctx ValidationContext) error {
	if l == "" {
		return status.Error(codes.InvalidArgument, "L2KeyID is empty")
	}

	if ctx == nil {
		return nil
	}

	if err := ctx.ValidateField(string(l)); err != nil {
		return err
	}

	return nil
}
