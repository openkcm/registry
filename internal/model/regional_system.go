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
	RegionalSystemSystemIDValidationID validation.ID = "RegionalSystem.SystemID"
	RegionalSystemRegionValidationID   validation.ID = "RegionalSystem.Region"
	SystemStatusValidationID           validation.ID = "RegionalSystem.Status"
)

// RegionalSystem represents a customer-exposed "tenant" of any kind.
type RegionalSystem struct {
	SystemID      string    `gorm:"column:system_id;primaryKey" validationID:"RegionalSystem.SystemID"`
	Region        string    `gorm:"column:region;primaryKey" validationID:"RegionalSystem.Region"`
	Status        string    `gorm:"column:status" validationID:"RegionalSystem.Status"`
	L2KeyID       string    `gorm:"column:l2key_id" validationID:"RegionalSystem.L2KeyID"`
	HasL1KeyClaim *bool     `gorm:"column:has_l1_key_claim"` // claim status of related L1 key
	Labels        Labels    `gorm:"column:labels;type:jsonb"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`

	System *System `gorm:"foreignKey:SystemID;references:ID"`
}

// TableName returns the table name of the System entity.
func (s *RegionalSystem) TableName() string {
	return "regional_systems"
}

// HasActiveL1KeyClaim returns true if the System has active L1KeyClaim.
func (s *RegionalSystem) HasActiveL1KeyClaim() bool {
	return s.HasL1KeyClaim != nil && *s.HasL1KeyClaim
}

// PaginationKey returns the fields used for pagination.
func (s *RegionalSystem) PaginationKey() map[repository.QueryField]any {
	// The pagination key is a combination of ExternalID and Region.
	keys := make(map[repository.QueryField]any)
	keys[repository.SystemIDField] = s.SystemID
	keys[repository.RegionField] = s.Region

	return keys
}

// ToProto converts the System to its gRPC representation.
func (s *RegionalSystem) ToProto() (*systemgrpc.System, error) {
	if s.System == nil {
		return nil, ErrSystemNotLoaded
	}

	var hasL1KeyClaim bool
	if s.HasL1KeyClaim != nil {
		hasL1KeyClaim = *s.HasL1KeyClaim
	}

	var tenantID string
	if s.System.TenantID != nil {
		tenantID = *s.System.TenantID
	}

	return &systemgrpc.System{
		ExternalId:    s.System.ExternalID,
		TenantId:      tenantID,
		L2KeyId:       s.L2KeyID,
		HasL1KeyClaim: hasL1KeyClaim,
		Region:        s.Region,
		Status:        typespb.Status(typespb.Status_value[s.Status]),
		Type:          s.System.Type,
		Labels:        s.Labels,
		UpdatedAt:     formatTime(s.UpdatedAt),
		CreatedAt:     formatTime(s.CreatedAt),
	}, nil
}

// IsAvailable returns true if the System status is STATUS_AVAILABLE.
func (s *RegionalSystem) IsAvailable() bool {
	return s.Status == typespb.Status_STATUS_AVAILABLE.String()
}

// Validations returns the validation fields for the System model.
func (s *RegionalSystem) Validations() []validation.Field {
	fields := make([]validation.Field, 0, 4)

	fields = append(fields, validation.Field{
		ID: RegionalSystemSystemIDValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})

	fields = append(fields, validation.Field{
		ID: RegionalSystemRegionValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})

	fields = append(fields, validation.Field{
		ID: SystemStatusValidationID,
		Validators: []validation.Validator{
			RegionalSystemStatusConstraint{},
		},
	})

	fields = append(fields, validation.Field{
		ID: "RegionalSystem.L2KeyID",
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})

	return fields
}

// RegionalSystemStatusConstraint validates that the system status is one of the allowed statuses.
type RegionalSystemStatusConstraint struct{}

var validSystemStatuses map[string]struct{}

// Validate checks if the provided system status is valid.
func (c RegionalSystemStatusConstraint) Validate(value any) error {
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
