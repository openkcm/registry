package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExternalID represents the external ID of the system.
type ExternalID string

func (e ExternalID) Validate() error {
	if len(e) == 0 {
		return status.Error(codes.InvalidArgument, "ExternalID is empty")
	}

	return nil
}

func (e ExternalID) String() string {
	return string(e)
}
