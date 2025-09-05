package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SystemType represents the type of  system.
type SystemType string

// Validate checks if the SystemType is valid based on the field validation configuration.
func (s SystemType) Validate() error {
	if s == "" {
		return status.Error(codes.InvalidArgument, "Missing system type")
	}

	return nil
}

func (s SystemType) String() string {
	return string(s)
}
