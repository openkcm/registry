package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Name represents the name of the tenant.
type Name string

// Validate validates given name of the tenant.
func (n Name) Validate(ctx ValidationContext) error {
	if n == "" {
		return status.Error(codes.InvalidArgument, "Name is empty")
	}

	if ctx == nil {
		return nil
	}

	if err := ctx.ValidateField(string(n)); err != nil {
		return err
	}

	return nil
}

func (n Name) String() string {
	return string(n)
}
