package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Region represents the region of the model.
type Region string

// Validate validates given region of the model.
func (r Region) Validate() error {
	if len(r) == 0 {
		return status.Error(codes.InvalidArgument, "Region is empty")
	}

	return nil
}

func (r Region) String() string {
	return string(r)
}
