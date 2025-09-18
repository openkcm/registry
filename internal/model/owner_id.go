package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OwnerID represents the owner ID of the tenant model.
type OwnerID string

// Validate validates given OwnerID of the model.
func (o OwnerID) Validate(ctx ValidationContext) error {
	if len(o) == 0 {
		return status.Error(codes.InvalidArgument, "owner id is empty")
	}

	if ctx == nil {
		return nil
	}

	if err := ctx.ValidateField(string(o)); err != nil {
		return err
	}

	return nil
}

func (o OwnerID) String() string {
	return string(o)
}
