package model

import (
	"errors"
	"fmt"
	"slices"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
)

// TenantStatus represents the status of the tenant.
type TenantStatus string

var ErrInvalidTransition = errors.New("invalid tenant status transition")

var (
	// validTenantStatusTransitions defines the valid transitions between tenant statuses.
	validTenantStatusTransitions = map[pb.Status][]pb.Status{
		pb.Status_STATUS_REQUESTED: {
			pb.Status_STATUS_PROVISIONING,
		},
		pb.Status_STATUS_PROVISIONING: {
			pb.Status_STATUS_ACTIVE,
			pb.Status_STATUS_PROVISIONING_ERROR,
		},
		pb.Status_STATUS_PROVISIONING_ERROR: {
			pb.Status_STATUS_PROVISIONING,
		},
		pb.Status_STATUS_ACTIVE: {
			pb.Status_STATUS_BLOCKING,
		},
		pb.Status_STATUS_BLOCKING: {
			pb.Status_STATUS_BLOCKED,
			pb.Status_STATUS_BLOCKING_ERROR,
		},
		pb.Status_STATUS_BLOCKING_ERROR: {
			pb.Status_STATUS_BLOCKING,
		},
		pb.Status_STATUS_BLOCKED: {
			pb.Status_STATUS_TERMINATING,
			pb.Status_STATUS_UNBLOCKING,
		},
		pb.Status_STATUS_UNBLOCKING: {
			pb.Status_STATUS_ACTIVE,
			pb.Status_STATUS_UNBLOCKING_ERROR,
		},
		pb.Status_STATUS_UNBLOCKING_ERROR: {
			pb.Status_STATUS_UNBLOCKING,
		},
		pb.Status_STATUS_TERMINATING: {
			pb.Status_STATUS_TERMINATED,
			pb.Status_STATUS_TERMINATION_ERROR,
		},
		pb.Status_STATUS_TERMINATION_ERROR: {
			pb.Status_STATUS_TERMINATING,
		},
		pb.Status_STATUS_TERMINATED: {},
	}
)

// ValidateTransition checks if the transition from the current status to the target status is valid.
func (ts TenantStatus) ValidateTransition(to pb.Status) error {
	from := pb.Status_STATUS_UNSPECIFIED
	if ts != "" {
		from = pb.Status(pb.Status_value[string(ts)])
	}

	if validTransitions, ok := validTenantStatusTransitions[from]; ok {
		if slices.Contains(validTransitions, to) {
			return nil
		}
	}

	return fmt.Errorf("%w from %s to %s", ErrInvalidTransition, from, to)
}

// IsActive checks if Status is active.
func (ts TenantStatus) IsActive() bool {
	return string(ts) == pb.Status_STATUS_ACTIVE.String()
}
