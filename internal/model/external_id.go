package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrExternalIDIsEmpty = status.Error(codes.InvalidArgument, "external id is empty")
)

// ExternalID represents the external ID of the system.
type ExternalID string

func (e ExternalID) Validate(_ ValidationContext) error {
	if len(e) == 0 {
		return ErrExternalIDIsEmpty
	}

	return nil
}

func (e ExternalID) String() string {
	return string(e)
}
