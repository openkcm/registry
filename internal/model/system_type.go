package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SystemType represents the type of  system.
type SystemType string

// Validate checks if the SystemType is valid based on the field validation configuration.
func (s SystemType) Validate(ctx ValidationContext) error {
	if s == "" {
		return status.Error(codes.InvalidArgument, "System type is empty")
	}

	if ctx == nil {
		return nil
	}

	if err := ctx.ValidateField(string(s)); err != nil {
		return err
	}

	return nil
}

func (s SystemType) String() string {
	return string(s)
}
