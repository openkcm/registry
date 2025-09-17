package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrEmptyID = status.Error(codes.InvalidArgument, "ID is empty")
)

// ID represents the ID of a resource.
type ID string

// Validate validates given ID of the model.
func (i ID) Validate(_ ValidationContext) error {
	if i == "" {
		return ErrEmptyID
	}

	return nil
}

func (i ID) String() string {
	return string(i)
}
