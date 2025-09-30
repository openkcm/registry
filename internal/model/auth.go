package model

import (
	"time"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"

	"github.com/openkcm/registry/internal/repository"
)

// Auth represents an auth method associated with a tenant.
type Auth struct {
	ExternalID   ExternalID     `gorm:"column:id;primaryKey" validators:"non-empty"`
	TenantID     ID             `gorm:"column:tenant_id;not null" validators:"non-empty"`
	Type         AuthType       `gorm:"column:type;not null" validators:"non-empty"`
	Properties   AuthProperties `gorm:"column:properties;type:jsonb" validators:"map"`
	Status       AuthStatus     `gorm:"column:status;not null" validators:"non-empty"`
	ErrorMessage string         `gorm:"column:error_message"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	CreatedAt    time.Time      `gorm:"column:created_at;autoCreateTime"`
}

// TableName specifies the database table name for the Auth model.
func (a *Auth) TableName() string {
	return "auths"
}

// Validate validates given tenant data.
func (a *Auth) Validate() error {
	return ValidateStruct(a)
}

// PaginationKey returns a map representing the pagination key for the Auth model.
func (a *Auth) PaginationKey() map[repository.QueryField]any {
	key := make(map[repository.QueryField]any)
	key[repository.IDField] = a.ExternalID
	return key
}

// ToProto converts the Auth model to its protobuf representation.
func (a *Auth) ToProto() *authgrpc.Auth {
	return &authgrpc.Auth{
		ExternalId:   a.ExternalID.String(),
		TenantId:     a.TenantID.String(),
		Type:         a.Type.String(),
		Properties:   a.Properties,
		Status:       authgrpc.AuthStatus(authgrpc.AuthStatus_value[a.Status.String()]),
		ErrorMessage: a.ErrorMessage,
		UpdatedAt:    formatTime(a.UpdatedAt),
		CreatedAt:    formatTime(a.CreatedAt),
	}
}
