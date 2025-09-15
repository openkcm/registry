package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SystemType represents the type of  system.
type SystemType string

const (
	SystemTypeSystem SystemType = "system"
)

func (s SystemType) Validate() error {
	if s == "" {
		return status.Error(codes.InvalidArgument, "Missing system type")
	}

	return nil
}

func (s SystemType) String() string {
	return string(s)
}
