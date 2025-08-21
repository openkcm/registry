package model

import (
	"slices"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SystemType represents the type of  system.
type SystemType string

const (
	SystemTypeSystem     SystemType = "system"
	SystemTypeSubaccount SystemType = "subaccount"
)

var allowedSystemTypes = []SystemType{SystemTypeSystem, SystemTypeSubaccount}

func (s SystemType) Validate() error {
	if !slices.Contains(allowedSystemTypes, s) {
		return status.Error(codes.InvalidArgument, "Invalid system type")
	}

	return nil
}

func (s SystemType) String() string {
	return string(s)
}
