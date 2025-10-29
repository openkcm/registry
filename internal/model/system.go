package model

import (
	"time"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	typespb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

// Validation IDs for the System model fields that are validated individually.
const (
	SystemExternalIDValidationID validation.ID = "System.ExternalID"
	SystemRegionValidationID     validation.ID = "System.Region"
	SystemStatusValidationID     validation.ID = "System.Status"
)

// System represents a customer-exposed "tenant" of any kind.
type System struct {
	ExternalID    string    `gorm:"column:external_id;primaryKey" validationID:"System.ExternalID"`
	TenantID      *string   `gorm:"column:tenant_id"` // related tenant id; optional
	Region        string    `gorm:"column:region;primaryKey" validationID:"System.Region"`
	Status        string    `gorm:"column:status" validationID:"System.Status"`
	L2KeyID       string    `gorm:"column:l2key_id" validationID:"System.L2KeyID"`
	HasL1KeyClaim *bool     `gorm:"column:has_l1_key_claim"` // claim status of related L1 key
	Type          string    `gorm:"column:type" validationID:"System.Type"`
	Labels        Labels    `gorm:"column:labels;type:jsonb"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName returns the table name of the System entity.
func (s *System) TableName() string {
	return "systems"
}

// IsLinkedToTenant returns true if the System is linked to the Tenant.
func (s *System) IsLinkedToTenant() bool {
	return s.TenantID != nil && *s.TenantID != ""
}

// HasActiveL1KeyClaim returns true if the System has active L1KeyClaim.
func (s *System) HasActiveL1KeyClaim() bool {
	return s.HasL1KeyClaim != nil && *s.HasL1KeyClaim
}

// PaginationKey returns the fields used for pagination.
func (s *System) PaginationKey() map[repository.QueryField]any {
	// The pagination key is a combination of ExternalID and Region.
	keys := make(map[repository.QueryField]any)
	keys[repository.ExternalIDField] = s.ExternalID
	keys[repository.RegionField] = s.Region

	return keys
}

// ToProto converts the System to its gRPC representation.
func (s *System) ToProto() *systemgrpc.System {
	var hasL1KeyClaim bool
	if s.HasL1KeyClaim != nil {
		hasL1KeyClaim = *s.HasL1KeyClaim
	}

	return &systemgrpc.System{
		ExternalId:    s.ExternalID,
		TenantId:      *s.TenantID,
		L2KeyId:       s.L2KeyID,
		HasL1KeyClaim: hasL1KeyClaim,
		Region:        s.Region,
		Status:        typespb.Status(typespb.Status_value[s.Status]),
		Type:          s.Type,
		Labels:        s.Labels,
		UpdatedAt:     formatTime(s.UpdatedAt),
		CreatedAt:     formatTime(s.CreatedAt),
	}
}

// IsAvailable returns true if the System status is STATUS_AVAILABLE.
func (s *System) IsAvailable() bool {
	return s.Status == typespb.Status_STATUS_AVAILABLE.String()
}

// Validations returns the validation fields for the System model.
func (s *System) Validations() []validation.Field {
	fields := make([]validation.Field, 0, 7)

	fields = append(fields, validation.Field{
		ID: SystemExternalIDValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})

	fields = append(fields, validation.Field{
		ID: SystemRegionValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})

	fields = append(fields, validation.Field{
		ID: SystemStatusValidationID,
		Validators: []validation.Validator{
			SystemStatusConstraint{},
		},
	})

	fields = append(fields, validation.Field{
		ID: "System.L2KeyID",
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})

	fields = append(fields, validation.Field{
		ID: "System.Type",
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})

	return fields
}

// SystemStatusConstraint validates that the system status is one of the allowed statuses.
type SystemStatusConstraint struct{}

var validSystemStatuses map[string]struct{}

// Validate checks if the provided system status is valid.
func (c SystemStatusConstraint) Validate(value any) error {
	status, ok := value.(string)
	if !ok {
		return validation.ErrWrongType
	}

	// lazy initialization of valid system statuses
	if validSystemStatuses == nil {
		validSystemStatuses = make(map[string]struct{})
		for _, v := range typespb.Status_name {
			if v != typespb.Status_STATUS_UNSPECIFIED.String() {
				validSystemStatuses[v] = struct{}{}
			}
		}
	}

	if _, exists := validSystemStatuses[status]; !exists {
		return validation.ErrValueNotAllowed
	}

	return nil
}
