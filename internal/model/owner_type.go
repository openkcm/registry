package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OwnerType represents the type of owner for the tenant model.
type OwnerType string

func (o OwnerType) Validate() error {
	if o == "" {
		return status.Error(codes.InvalidArgument, "Invalid owner type")
	}

	return nil
}

func (o OwnerType) String() string {
	return string(o)
}
