package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OwnerType represents the type of owner for the tenant model.
type OwnerType string

func (o OwnerType) Validate(ctx ValidationContext) error {
	if o == "" {
		return status.Error(codes.InvalidArgument, "owner type is empty")
	}

	if ctx == nil {
		return nil
	}

	if err := ctx.ValidateField(string(o)); err != nil {
		return err
	}

	return nil
}

func (o OwnerType) String() string {
	return string(o)
}
