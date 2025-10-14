package model

import (
	"time"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"

	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

// Auth represents an auth method associated with a tenant.
type Auth struct {
	ExternalID   AuthExternalID `gorm:"column:id;primaryKey" validationID:"Auth.ExternalID"`
	TenantID     ID             `gorm:"column:tenant_id;not null" validationID:"Auth.TenantID"`
	Type         AuthType       `gorm:"column:type;not null" validationID:"Auth.Type"`
	Properties   AuthProperties `gorm:"column:properties;type:jsonb" validationID:"Auth.Properties"`
	Status       AuthStatus     `gorm:"column:status;not null" validationID:"Auth.Status"`
	ErrorMessage string         `gorm:"column:error_message" validationID:"Auth.ErrorMessage"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;autoUpdateTime" validationID:"Auth.UpdatedAt"`
	CreatedAt    time.Time      `gorm:"column:created_at;autoCreateTime" validationID:"Auth.CreatedAt"`
}

// TableName specifies the database table name for the Auth model.
func (a *Auth) TableName() string {
	return "auths"
}

func (a *Auth) Validate() error {
	// Will be replaced with different validation approach in the future.
	return nil
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

// Fields returns the validation fields for the Auth struct.
// This is used by the validation package.
func (a *Auth) Fields() []validation.StructField {
	return validation.GetFields(a.ExternalID, a.Type, a.Properties, a.Status)
}
