package model

import (
	"time"

	tenantconfiggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant_config/v1"

	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

const TenantConfigTenantIDValidationID validation.ID = "TenantConfig.TenantID"

// TenantConfig stores per-tenant configuration overrides and their reconciliation status.
// The status tracks whether the configuration has been propagated to the CMK layer via
// an orbital job (UPDATING → ACTIVE on success, UPDATING_ERROR on failure).
type TenantConfig struct {
	TenantID     string    `gorm:"column:tenant_id;primaryKey" validationID:"TenantConfig.TenantID"`
	SystemLimit  *int32    `gorm:"column:system_limit"`
	Status       string    `gorm:"column:status;not null"`
	ErrorMessage string    `gorm:"column:error_message"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// Validations returns the default field validators for TenantConfig.
func (tc *TenantConfig) Validations() []validation.Field {
	return []validation.Field{
		{
			ID: TenantConfigTenantIDValidationID,
			Validators: []validation.Validator{
				validation.NonEmptyConstraint{},
			},
		},
	}
}

// TableName returns the table name for the TenantConfig model.
func (tc *TenantConfig) TableName() string {
	return "tenant_configs"
}

// PaginationKey returns the fields used for pagination.
func (tc *TenantConfig) PaginationKey() map[repository.QueryField]any {
	return map[repository.QueryField]any{
		repository.TenantIDField: tc.TenantID,
	}
}

// ToProto converts the TenantConfig model to its GetTenantConfigResponse protobuf representation.
func (tc *TenantConfig) ToProto() *tenantconfiggrpc.GetTenantConfigResponse {
	tenantID := tc.TenantID
	st := tenantconfiggrpc.TenantConfigStatus(tenantconfiggrpc.TenantConfigStatus_value[tc.Status])
	errMsg := tc.ErrorMessage
	createdAt := formatTime(tc.CreatedAt)
	updatedAt := formatTime(tc.UpdatedAt)

	b := tenantconfiggrpc.GetTenantConfigResponse_builder{
		TenantId:     &tenantID,
		Status:       &st,
		ErrorMessage: &errMsg,
		CreatedAt:    &createdAt,
		UpdatedAt:    &updatedAt,
	}
	if tc.SystemLimit != nil {
		sl := *tc.SystemLimit
		b.Values = tenantconfiggrpc.TenantConfigurationValues_builder{SystemLimit: &sl}.Build()
	}
	return b.Build()
}
