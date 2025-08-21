package model

import (
	"time"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	typesgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/registry/internal/repository"
)

// System represents a customer-exposed "tenant" of any kind.
// Systems are exposed to customer through SAP Formations.
type System struct {
	ExternalID    ExternalID `gorm:"column:external_id;primaryKey"`
	TenantID      *string    `gorm:"column:tenant_id"` // related tenant id; optional
	Region        Region     `gorm:"column:region;primaryKey"`
	Status        Status     `gorm:"column:status"`
	L2KeyID       L2KeyID    `gorm:"column:l2key_id"`
	HasL1KeyClaim *bool      `gorm:"column:has_l1_key_claim"` // claim status of related L1 key
	Type          SystemType `gorm:"column:type"`
	Labels        Labels     `gorm:"column:labels;type:jsonb"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;autoUpdateTime"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime"`
}

// TableName returns the table name of the System entity.
func (s *System) TableName() string {
	return "systems"
}

// Validate validates given System data.
func (s *System) Validate() error {
	return ValidateAll(s.ExternalID, s.Region, s.L2KeyID, s.Status, s.Type, &s.Labels)
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
		ExternalId:    s.ExternalID.String(),
		TenantId:      *s.TenantID,
		L2KeyId:       string(s.L2KeyID),
		HasL1KeyClaim: hasL1KeyClaim,
		Region:        s.Region.String(),
		Status:        typesgrpc.Status(typesgrpc.Status_value[string(s.Status)]),
		Type:          s.Type.String(),
		Labels:        s.Labels,
		UpdatedAt:     formatTime(s.UpdatedAt),
		CreatedAt:     formatTime(s.CreatedAt),
	}
}
