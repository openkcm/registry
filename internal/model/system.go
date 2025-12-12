package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

var ErrSystemNotLoaded = errors.New("system for regional system is not loaded")

const (
	SystemExternalIDValidationID validation.ID = "System.ExternalID"
	SystemIDValidationID         validation.ID = "System.ID"
	SystemTypeValidationID       validation.ID = "System.Type"
)

type System struct {
	ID         string    `gorm:"column:id;primaryKey" validationID:"System.ID"`
	ExternalID string    `gorm:"column:external_id;uniqueIndex:ext_type" validationID:"System.ExternalID"`
	TenantID   *string   `gorm:"column:tenant_id"` // related tenant id; optional
	Type       string    `gorm:"column:type;uniqueIndex:ext_type" validationID:"System.Type"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

func NewSystem(externalID, systemType string) *System {
	s := &System{
		ExternalID: externalID,
		Type:       systemType,
	}
	s.ID = s.GetID()

	return s
}

func (s *System) GetID() string {
	return fmt.Sprintf("%s-%s", s.ExternalID, s.Type)
}

func (s *System) LinkTenant(tenantID string) {
	s.TenantID = &tenantID
}

func (s *System) IsLinkedToTenant() bool {
	return s.TenantID != nil && *s.TenantID != ""
}

// TableName returns the table name of the GlobalSystem entity.
func (s *System) TableName() string {
	return "systems"
}

// PaginationKey returns the fields used for pagination.
func (s *System) PaginationKey() map[repository.QueryField]any {
	key := make(map[repository.QueryField]any)
	key[repository.IDField] = s.ID

	return key
}

func (s *System) Validations() []validation.Field {
	fields := make([]validation.Field, 0)

	fields = append(fields, validation.Field{
		ID: SystemExternalIDValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})

	fields = append(fields, validation.Field{
		ID: SystemTypeValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})

	fields = append(fields, validation.Field{
		ID: SystemIDValidationID,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})

	return fields
}
