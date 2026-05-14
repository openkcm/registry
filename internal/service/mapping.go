package service

import (
	"context"

	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

// Mapping implements the procedure calls defined as protobufs.
// See https://github.com/openkcm/api-sdk/blob/main/proto/kms/api/cmk/registry/mapping/v1/mapping.proto.
type Mapping struct {
	mappinggrpc.UnimplementedServiceServer

	repo       repository.Repository
	meters     *Meters
	validation *validation.Validation
}

// NewMapping creates and returns a new instance of Mapping.
func NewMapping(repo repository.Repository, meters *Meters, validation *validation.Validation) *Mapping {
	return &Mapping{
		repo:       repo,
		meters:     meters,
		validation: validation,
	}
}

// UnmapSystemFromTenant unlinks Systems from the Tenant.
func (m *Mapping) UnmapSystemFromTenant(ctx context.Context, in *mappinggrpc.UnmapSystemFromTenantRequest) (*mappinggrpc.UnmapSystemFromTenantResponse, error) {
	ctx = slogctx.With(ctx, "tenantId", in.GetTenantId(), "externalId", in.GetExternalId(), "type", in.GetType())
	slogctx.Debug(ctx, "UnmapSystemFromTenant called")

	if err := m.validateUnmapRequest(in); err != nil {
		slogctx.Error(ctx, "validation failed for UnmapSystemFromTenant request", "error", err)
		return nil, err
	}

	emptyTenantID := ""

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := m.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		system, validateErr := validateAndGetSystemForUnmap(ctx, r, in)
		if validateErr != nil {
			slogctx.Error(ctx, "validateAndGetSystemForUnmap failed", "error", validateErr)
			return validateErr
		}

		system.TenantID = &emptyTenantID
		ok, patchErr := r.Patch(ctx, system)
		if patchErr != nil {
			slogctx.Error(ctx, "failed to patch system during unmap", "error", patchErr)
			return ErrSystemUpdate
		}

		if !ok {
			return ErrorWithParams(ErrSystemNotFound, "externalID", in.GetExternalId(), "type", in.GetType())
		}

		slogctx.Debug(ctx, "system successfully unmapped in transaction")
		return nil
	})

	err = mapError(err)
	if err != nil {
		slogctx.Error(ctx, "failed to unmap system from tenant", "error", err)
		return nil, err
	}

	slogctx.Info(ctx, "system successfully unmapped from tenant")
	return &mappinggrpc.UnmapSystemFromTenantResponse{Success: true}, nil
}

// MapSystemToTenant links Systems to the Tenant.
func (m *Mapping) MapSystemToTenant(ctx context.Context, in *mappinggrpc.MapSystemToTenantRequest) (*mappinggrpc.MapSystemToTenantResponse, error) {
	ctx = slogctx.With(ctx, "tenantId", in.GetTenantId(), "externalId", in.GetExternalId(), "type", in.GetType())

	tenantID := in.GetTenantId()
	slogctx.Debug(ctx, "MapSystemToTenant called")

	if err := m.validateMapRequest(in); err != nil {
		slogctx.Error(ctx, "validation failed for MapSystemToTenant request", "error", err)
		return nil, err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := m.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		system, found, validateErr := isSystemTenantMapAllowed(ctx, r, in)
		if validateErr != nil {
			slogctx.Error(ctx, "isSystemTenantMapAllowed failed", "error", validateErr)
			return validateErr
		}

		if !found {
			_, createErr := createSystem(ctx, m.validation, r, in.GetExternalId(), in.GetType(), tenantID)
			if createErr != nil {
				slogctx.Error(ctx, "failed to create system during map", "error", createErr)
			}
			return createErr
		}

		system.TenantID = &tenantID
		_, patchErr := r.Patch(ctx, system)
		if patchErr != nil {
			slogctx.Error(ctx, "failed to patch system during map", "error", patchErr)
			return ErrSystemUpdate
		}

		slogctx.Debug(ctx, "system successfully mapped in transaction")
		return nil
	})

	err = mapError(err)
	if err != nil {
		slogctx.Error(ctx, "failed to map system to tenant", "error", err)
		return nil, err
	}

	slogctx.Info(ctx, "system successfully mapped to tenant")
	return &mappinggrpc.MapSystemToTenantResponse{Success: true}, nil
}

// Get gets the mapped tenant from the system.
func (m *Mapping) Get(ctx context.Context, in *mappinggrpc.GetRequest) (*mappinggrpc.GetResponse, error) {
	ctx = slogctx.With(ctx, "externalId", in.GetExternalId(), "type", in.GetType())
	slogctx.Debug(ctx, "Get called")

	if err := validateExternalIDAndType(m.validation, in.GetExternalId(), in.GetType()); err != nil {
		slogctx.Error(ctx, "validation failed for Get request", "error", err)
		return nil, err
	}

	system, found, err := getSystem(ctx, m.repo, in.GetExternalId(), in.GetType())
	if err != nil {
		slogctx.Error(ctx, "failed to get system for Get request", "error", err)
		return nil, ErrSystemSelect
	}

	if !found {
		slogctx.Debug(ctx, "system not found for Get request")
		return nil, ErrSystemNotFound
	}

	tenantID := ""
	if system.TenantID != nil {
		tenantID = *system.TenantID
	}

	return &mappinggrpc.GetResponse{
		TenantId: tenantID,
	}, nil
}

// validateAndGetSystemForUnmap fetches and returns the system. It also validates
// that the tenantID matches, that the tenant is active, and checks the regional systems validity.
func validateAndGetSystemForUnmap(ctx context.Context, r repository.Repository, in *mappinggrpc.UnmapSystemFromTenantRequest) (*model.System, error) {
	tenantID := in.GetTenantId()

	system, found, err := getSystem(ctx, r, in.GetExternalId(), in.GetType())
	if err != nil {
		slogctx.Error(ctx, "failed to get system for unmap", "error", err)
		return nil, ErrSystemSelect
	}
	if !found {
		slogctx.Warn(ctx, "system not found for unmap", "externalId", in.GetExternalId(), "type", in.GetType())
		return nil, ErrSystemNotFound
	}

	if !system.IsLinkedToTenant() {
		slogctx.Warn(ctx, "system is not linked to any tenant", "externalId", system.ExternalID, "type", system.Type)
		return nil, ErrorWithParams(ErrSystemIsNotLinkedToTenant, "externalID", system.ExternalID, "type", system.Type)
	}

	if *system.TenantID != tenantID {
		slogctx.Warn(ctx, "system is linked to a different tenant", "externalId", system.ExternalID, "type", system.Type, "linkedTenantId", *system.TenantID, "requestedTenantId", tenantID)
		return nil, ErrorWithParams(ErrSystemIsNotLinkedToTenant, "externalID", system.ExternalID, "type", system.Type)
	}

	tenant, getTenantErr := getTenant(ctx, r, *system.TenantID)
	if getTenantErr != nil {
		slogctx.Error(ctx, "failed to get tenant for unmap", "tenantId", *system.TenantID, "error", getTenantErr)
		return nil, getTenantErr
	}

	if activeErr := checkTenantActive(tenant); activeErr != nil {
		slogctx.Warn(ctx, "tenant is not active for unmap", "tenantId", tenant.ID, "status", tenant.Status, "error", activeErr)
		return nil, activeErr
	}

	if regionalErr := validateRegionalSystemsForUnmap(ctx, r, system); regionalErr != nil {
		slogctx.Warn(ctx, "regional systems validation failed for unmap", "externalId", system.ExternalID, "type", system.Type, "error", regionalErr)
		return nil, regionalErr
	}

	return system, nil
}

func validateRegionalSystemsForUnmap(ctx context.Context, r repository.Repository, system *model.System) error {
	regionalSystems, err := getRegionalSystemsFromSystemID(ctx, r, system.ID.String())
	if err != nil {
		return err
	}

	for _, s := range regionalSystems {
		if err := checkRegionalSystemAvailable(&s); err != nil {
			return err
		}

		if s.HasActiveL1KeyClaim() {
			return ErrorWithParams(ErrSystemHasL1KeyClaim, "externalID", system.ExternalID, "type", system.Type, "region", s.Region)
		}
	}

	return nil
}

// isSystemTenantMapAllowed checks whether all conditions are met to map the Tenant.
// It returns (nil, false, nil) when the Tenant is valid and the System does not exist yet,
// so the caller can create it. It returns (system, true, nil) when the Tenant is valid,
// the System is found, is not linked, and has no active L1 key claim. The bool indicates
// whether the System was found.
func isSystemTenantMapAllowed(ctx context.Context, r repository.Repository, in *mappinggrpc.MapSystemToTenantRequest) (*model.System, bool, error) {
	tenant, getTenantErr := getTenant(ctx, r, in.GetTenantId())
	if getTenantErr != nil {
		slogctx.Error(ctx, "failed to get tenant for map", "tenantId", in.GetTenantId(), "error", getTenantErr)
		return nil, false, getTenantErr
	}

	if activeErr := checkTenantActive(tenant); activeErr != nil {
		slogctx.Warn(ctx, "tenant is not active for map", "tenantId", tenant.ID, "status", tenant.Status, "error", activeErr)
		return nil, false, activeErr
	}

	system, found, getSystemErr := getSystem(ctx, r, in.GetExternalId(), in.GetType())
	if getSystemErr != nil {
		slogctx.Error(ctx, "failed to get system for map", "externalId", in.GetExternalId(), "type", in.GetType(), "error", getSystemErr)
		return nil, false, getSystemErr
	}

	if !found {
		slogctx.Debug(ctx, "system not found - will create new", "externalId", in.GetExternalId(), "type", in.GetType())
		return nil, false, nil
	}

	// For linking, each system must not be already linked and must not have an active L1 key claim.
	if system.IsLinkedToTenant() {
		slogctx.Warn(ctx, "system is already linked to a tenant", "externalId", system.ExternalID, "type", system.Type, "linkedTenantId", *system.TenantID)
		return system, found, ErrorWithParams(ErrSystemIsLinkedToTenant, "externalID", system.ExternalID, "type", system.Type)
	}

	if regionalErr := validateRegionalSystemsForLink(ctx, r, system); regionalErr != nil {
		slogctx.Warn(ctx, "regional systems validation failed for map", "externalId", system.ExternalID, "type", system.Type, "error", regionalErr)
		return system, found, regionalErr
	}

	return system, found, nil
}

// validateRegionalSystemsForLink checks the regional systems for Status and active L1KeyClaim.
func validateRegionalSystemsForLink(ctx context.Context, r repository.Repository, system *model.System) error {
	regionalSystems, err := getRegionalSystemsFromSystemID(ctx, r, system.ID.String())
	if err != nil {
		return err
	}

	for _, s := range regionalSystems {
		err := checkRegionalSystemAvailable(&s)
		if err != nil {
			slogctx.Warn(ctx, "validation failed for MapSystemToTenant request", "error", err)
			return err
		}

		if s.HasL1KeyClaim != nil && *s.HasL1KeyClaim {
			err = ErrorWithParams(ErrSystemHasL1KeyClaim, "externalID", system.ExternalID, "type", system.Type, "region", s.Region)
			slogctx.Warn(ctx, "validation failed for MapSystemToTenant request", "error", err)
			return err
		}
	}

	return nil
}

// validateAndGetSystems validates the input slice of SystemId and returns a slice of model.System having only unique systems.
func (m *Mapping) validateUnmapRequest(in *mappinggrpc.UnmapSystemFromTenantRequest) error {
	if in == nil || len(in.GetTenantId()) == 0 {
		return ErrNoTenantID
	}

	return validateExternalIDAndType(m.validation, in.GetExternalId(), in.GetType())
}

func (m *Mapping) validateMapRequest(in *mappinggrpc.MapSystemToTenantRequest) error {
	if in == nil || len(in.GetTenantId()) == 0 {
		return ErrNoTenantID
	}

	return validateExternalIDAndType(m.validation, in.GetExternalId(), in.GetType())
}
