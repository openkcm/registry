package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"
)

// Status represents the status of the tenant.
type Status string

// validStatuses contains all valid tenant statuses. Calculated in the init().
var (
	validStatuses = map[Status]struct{}{}
)

// init calculates valid tenant status.
func init() {
	for _, v := range pb.Status_name {
		if v != pb.Status_STATUS_UNSPECIFIED.String() {
			validStatuses[Status(v)] = struct{}{}
		}
	}
}

// ValidateCustomRule implements CustomValidationRule interface.
func (s Status) ValidateCustomRule() error {
	if s == "" {
		return ErrFieldValueMustNotBeEmpty
	}
	if _, ok := validStatuses[s]; !ok {
		return ErrInvalidFieldValue
	}

	return nil
}

// IsAvailable checks if Status is available for processing.
func (s Status) IsAvailable() (bool, error) {
	if string(s) == pb.Status_STATUS_UNSPECIFIED.String() {
		return false, status.Error(codes.InvalidArgument, "Status is unspecified")
	}

	return string(s) == pb.Status_STATUS_AVAILABLE.String(), nil
}
