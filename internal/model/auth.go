package model

import (
	"fmt"
	"time"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"

	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

// Validation IDs for the Auth model fields that require validation.
const (
	AuthExternalIDValidationID validation.ID = "Auth.ExternalID"
	AuthTenantIDValidationID   validation.ID = "Auth.TenantID"
	AuthTypeValidationID       validation.ID = "Auth.Type"
	AuthPropertiesValidationID validation.ID = "Auth.Properties"
	AuthStatusValidationID     validation.ID = "Auth.Status"
)

// Auth represents an auth method associated with a tenant.
type Auth struct {
	ExternalID   string    `gorm:"column:id;primaryKey" validationID:"Auth.ExternalID"`
	TenantID     string    `gorm:"column:tenant_id;not null" validationID:"Auth.TenantID"`
	Type         string    `gorm:"column:type;not null" validationID:"Auth.Type"`
	Properties   Map       `gorm:"column:properties;type:jsonb" validationID:"Auth.Properties"`
	Status       string    `gorm:"column:status;not null" validationID:"Auth.Status"`
	ErrorMessage string    `gorm:"column:error_message"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName specifies the database table name for the Auth model.
func (a *Auth) TableName() string {
	return "auths"
}

// PaginationKey returns a map representing the pagination key for the Auth model.
func (a *Auth) PaginationKey() map[repository.QueryField]any {
	key := make(map[repository.QueryField]any)
	key[repository.IDField] = a.ExternalID
	return key
}

// ToProto converts the Auth model to its protobuf representation.
func (a *Auth) ToProto() *pb.Auth {
	return &pb.Auth{
		ExternalId:   a.ExternalID,
		TenantId:     a.TenantID,
		Type:         a.Type,
		Properties:   a.Properties,
		Status:       pb.AuthStatus(pb.AuthStatus_value[a.Status]),
		ErrorMessage: a.ErrorMessage,
		UpdatedAt:    formatTime(a.UpdatedAt),
		CreatedAt:    formatTime(a.CreatedAt),
	}
}

// Validations returns the validation fields for the Auth model.
func (a *Auth) Validations() []validation.Field {
	validations := make([]validation.Field, 0, 5)

	for _, id := range []validation.ID{
		AuthExternalIDValidationID,
		AuthTenantIDValidationID,
		AuthTypeValidationID,
	} {
		validations = append(validations, validation.Field{
			ID: id,
			Validators: []validation.Validator{
				validation.NonEmptyConstraint{},
			},
		})
	}

	validations = append(validations, validation.Field{
		ID: AuthStatusValidationID,
		Validators: []validation.Validator{
			AuthStatusConstraint{},
		},
	})

	validations = append(validations, validation.Field{
		ID: AuthPropertiesValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyKeysConstraint{},
		},
	})

	return validations
}

// AuthStatusConstraint validates the Auth.Status field.
type AuthStatusConstraint struct{}

var validAuthStatuses map[string]struct{}

// Validate checks if the provided value is a valid Auth status.
// Auth status must be one of the defined enum values in pb.AuthStatus.
func (c AuthStatusConstraint) Validate(value any) error {
	statusValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("%w: %T", validation.ErrWrongType, value)
	}
	// lazy initialization of validAuthStatuses
	if validAuthStatuses == nil {
		validAuthStatuses = make(map[string]struct{}, len(pb.AuthStatus_name)-1)
		for _, v := range pb.AuthStatus_name {
			if v != pb.AuthStatus_AUTH_STATUS_UNSPECIFIED.String() {
				validAuthStatuses[v] = struct{}{}
			}
		}
	}

	if _, ok := validAuthStatuses[statusValue]; !ok {
		return validation.ErrValueNotAllowed
	}
	return nil
}
