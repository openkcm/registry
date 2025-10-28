package model

import (
	"fmt"
	"time"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

const (
	TenantIDValidationID        = "Tenant.ID"
	TenantOwnerTypeValidationID = "Tenant.OwnerType"
)

// Tenant represents the customer-managed key (CMK) tenant entity.
type Tenant struct {
	ID              string       `gorm:"column:id;primaryKey" validationID:"Tenant.ID"`
	Name            string       `gorm:"column:name" validationID:"Tenant.Name"`
	Region          string       `gorm:"column:region" validationID:"Tenant.Region"`
	OwnerID         string       `gorm:"column:owner_id" validationID:"Tenant.OwnerID"`
	OwnerType       string       `gorm:"column:owner_type" validationID:"Tenant.OwnerType"`
	Status          TenantStatus `gorm:"column:status"`
	StatusUpdatedAt time.Time    `gorm:"column:status_updated_at"`
	Role            string       `gorm:"column:role" validationID:"Tenant.Role"`
	Labels          Labels       `gorm:"column:labels;type:jsonb"`
	UserGroups      UserGroups   `gorm:"column:user_groups;type:jsonb"`
	UpdatedAt       time.Time    `gorm:"column:updated_at;autoUpdateTime"`
	CreatedAt       time.Time    `gorm:"column:created_at;autoCreateTime"`
}

var _ validation.Model = &Tenant{}

// TableName returns the table name of the tenant entity.
func (t *Tenant) TableName() string {
	return "tenants"
}

// Validations returns the validation fields for the Tenant Model.
func (t *Tenant) Validations() []validation.Field {
	validations := make([]validation.Field, 0, 7)
	validations = append(validations, validation.Field{
		ID: TenantIDValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})
	validations = append(validations, validation.Field{
		ID: "Tenant.Name",
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})
	validations = append(validations, validation.Field{
		ID: "Tenant.Region",
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})
	validations = append(validations, validation.Field{
		ID: "Tenant.OwnerID",
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})
	validations = append(validations, validation.Field{
		ID: TenantOwnerTypeValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})
	validations = append(validations, validation.Field{
		ID: "Tenant.Role",
		Validators: []validation.Validator{
			TenantRoleConstraint{},
		},
	})
	return validations
}

// TenantRoleConstraint validates the Tenant.Role field.
type TenantRoleConstraint struct{}

var validTenantRoles map[string]struct{}

// Validate checks if the provided value is a valid Tenant role.
// Tenant role must be one of the defined enum values in tenant proto Role.
func (t TenantRoleConstraint) Validate(value any) error {
	roleValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("%w: %T", validation.ErrWrongType, value)
	}
	if validTenantRoles == nil {
		validTenantRoles = make(map[string]struct{}, len(tenantgrpc.Role_name)-1)
		for _, v := range tenantgrpc.Role_name {
			if v != tenantgrpc.Role_ROLE_UNSPECIFIED.String() {
				validTenantRoles[v] = struct{}{}
			}
		}
	}
	if _, ok := validTenantRoles[roleValue]; !ok {
		return validation.ErrValueNotAllowed
	}
	return nil
}

// PaginationKey returns the fields used for pagination.
func (t *Tenant) PaginationKey() map[repository.QueryField]any {
	key := make(map[repository.QueryField]any)
	key[repository.IDField] = t.ID

	return key
}

func (t *Tenant) ToProto() *tenantgrpc.Tenant {
	return &tenantgrpc.Tenant{
		Id:              t.ID,
		Name:            t.Name,
		Region:          t.Region,
		OwnerType:       t.OwnerType,
		OwnerId:         t.OwnerID,
		Status:          tenantgrpc.Status(tenantgrpc.Status_value[string(t.Status)]),
		StatusUpdatedAt: formatTime(t.StatusUpdatedAt),
		Role:            tenantgrpc.Role(tenantgrpc.Role_value[t.Role]),
		Labels:          t.Labels,
		UserGroups:      t.UserGroups,
		UpdatedAt:       formatTime(t.UpdatedAt),
		CreatedAt:       formatTime(t.CreatedAt),
	}
}

func (t *Tenant) SetStatus(status TenantStatus) {
	t.Status = status
	t.StatusUpdatedAt = time.Now()
}
