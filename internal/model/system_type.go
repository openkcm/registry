package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/registry/internal/config"
)

// SystemType represents the type of  system.
type SystemType string

// Validate checks if the SystemType is valid based on the field validation configuration.
func (s SystemType) Validate() error {
	// Use the field validation configuration
	err := config.ValidateField("system.type", string(s), true)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	return nil
}

func (s SystemType) String() string {
	return string(s)
}
