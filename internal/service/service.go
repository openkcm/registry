package service

import (
	"context"
	"time"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

const defaultTranTimeout = time.Second * 10

// assertTenantExist checks if a tenant exists in the database by tenant_id.
// It returns an error if the tenant does not exist.
func assertTenantExist(ctx context.Context, r repository.Repository, tenantID string) error {
	tenant := &model.Tenant{ID: tenantID}

	found, err := r.Find(ctx, tenant)
	if err != nil {
		return ErrTenantSelect
	}

	if !found {
		return ErrTenantNotFound
	}

	return nil
}

// getSystem fetches a system from the database by it's externalID and type.
// It returns the system, a boolean if the system is found and an error if an error occurs.
func getSystem(ctx context.Context, repo repository.Repository, externalID, systemType string) (*model.System, bool, error) {
	system := &model.System{
		ExternalID: externalID,
		Type:       systemType,
	}

	found, err := repo.Find(ctx, system)
	if err != nil {
		return nil, found, err
	}

	return system, found, nil
}

// getRegionalSystemsFormSystemID retrieves a list of model.RegionalSystem that have the given systemID.
func getRegionalSystemsFromSystemID(ctx context.Context, r repository.Repository, systemID string) ([]model.RegionalSystem, error) {
	query := repository.NewQuery(&model.RegionalSystem{})
	query.Where(repository.NewCompositeKey().Where(repository.SystemIDField, systemID))

	var regionalSystems []model.RegionalSystem

	if err := r.List(ctx, &regionalSystems, *query); err != nil {
		return nil, ErrSystemSelect
	}

	return regionalSystems, nil
}

// checkRegionalSystemAvailable returns nil if System has status Available.
func checkRegionalSystemAvailable(regionalSystem *model.RegionalSystem) error {
	if !regionalSystem.IsAvailable() {
		return ErrSystemUnavailable
	}
	return nil
}

// validateExternalIDAndType validates the externalID and type against the system's validator.
func validateExternalIDAndType(v *validation.Validation, externalID, systemType string) error {
	err := v.ValidateAll(map[validation.ID]any{
		model.SystemExternalIDValidationID: externalID,
		model.SystemTypeValidationID:       systemType,
	})
	if err != nil {
		return ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}

	return nil
}

// validateRegionalSystem uses the validator to validate the fields of a model.RegionalSystem.
func validateRegionalSystem(v *validation.Validation, regionalSystem *model.RegionalSystem) error {
	values, err := validation.GetValues(regionalSystem)
	if err != nil {
		return ErrorWithParams(ErrValidationConversion, "err", err.Error())
	}

	err = v.ValidateAll(values)
	if err != nil {
		return ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}

	return nil
}

// validateSystem uses the validator to validate the fields of a model.System.
func validateSystem(v *validation.Validation, system *model.System) error {
	values, err := validation.GetValues(system)
	if err != nil {
		return ErrorWithParams(ErrValidationConversion, "err", err.Error())
	}

	err = v.ValidateAll(values)
	if err != nil {
		return ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}

	return nil
}

// createSystem takes an externalID and a type to create a system in the databasse.
func createSystem(ctx context.Context, v *validation.Validation, repo repository.Repository, externalID, systemType, tenantID string) (*model.System, error) {
	system := &model.System{
		ExternalID: externalID,
		Type:       systemType,
	}

	if tenantID != "" {
		if err := assertTenantExist(ctx, repo, tenantID); err != nil {
			return nil, err
		}

		system.LinkTenant(tenantID)
	}

	if err := validateSystem(v, system); err != nil {
		return nil, err
	}

	if err := repo.Create(ctx, system); err != nil {
		return nil, err
	}

	return system, nil
}
