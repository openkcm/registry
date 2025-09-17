package model

import (
	"time"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/repository"
)

// Tenant represents the customer-managed key (CMK) tenant entity.
type Tenant struct {
	ID              ID           `gorm:"column:id;primaryKey"`
	Name            Name         `gorm:"column:name"`
	Region          Region       `gorm:"column:region"`
	OwnerID         OwnerID      `gorm:"column:owner_id"`
	OwnerType       OwnerType    `gorm:"column:owner_type"`
	Status          TenantStatus `gorm:"column:status"`
	StatusUpdatedAt time.Time    `gorm:"column:status_updated_at"`
	Role            Role         `gorm:"column:role"`
	Labels          Labels       `gorm:"column:labels;type:jsonb"`
	UpdatedAt       time.Time    `gorm:"column:updated_at;autoUpdateTime"`
	CreatedAt       time.Time    `gorm:"column:created_at;autoCreateTime"`
}

// TableName returns the table name of the tenant entity.
func (t *Tenant) TableName() string {
	return "tenants"
}

// Validate validates given tenant data.
func (t *Tenant) Validate(_ ValidationContext) error {
	return ValidateStruct(t)
}

// PaginationKey returns the fields used for pagination.
func (t *Tenant) PaginationKey() map[repository.QueryField]any {
	key := make(map[repository.QueryField]any)
	key[repository.IDField] = t.ID

	return key
}

func (t *Tenant) ToProto() *tenantgrpc.Tenant {
	return &tenantgrpc.Tenant{
		Id:              t.ID.String(),
		Name:            t.Name.String(),
		Region:          t.Region.String(),
		OwnerType:       t.OwnerType.String(),
		OwnerId:         t.OwnerID.String(),
		Status:          tenantgrpc.Status(tenantgrpc.Status_value[string(t.Status)]),
		StatusUpdatedAt: formatTime(t.StatusUpdatedAt),
		Role:            tenantgrpc.Role(tenantgrpc.Role_value[string(t.Role)]),
		Labels:          t.Labels,
		UpdatedAt:       formatTime(t.UpdatedAt),
		CreatedAt:       formatTime(t.CreatedAt),
	}
}

func (t *Tenant) SetStatus(status TenantStatus) {
	t.Status = status
	t.StatusUpdatedAt = time.Now()
}
