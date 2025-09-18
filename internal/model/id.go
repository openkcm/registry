package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrEmptyID = status.Error(codes.InvalidArgument, "id is empty")
)

// ID represents the ID of a resource.
type ID string

// Validate validates given ID of the model.
func (i ID) Validate(ctx ValidationContext) error {
	if i == "" {
		return ErrEmptyID
	}

	if ctx == nil {
		return nil
	}

	if err := ctx.ValidateField(string(i)); err != nil {
		return err
	}

	return nil
}

func (i ID) String() string {
	return string(i)
}
