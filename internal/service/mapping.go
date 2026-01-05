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
	slogctx.Debug(ctx, "UnmapSystemFromTenant called")

	if err := m.validateUnmapRequest(in); err != nil {
		return nil, err
	}

	emptyTenantID := ""

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := m.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		system, err := validateAndGetSystemForUnmap(ctx, r, in)
		if err != nil {
			return err
		}

		system.TenantID = &emptyTenantID
		ok, err := r.Patch(ctx, system)
		if err != nil {
			return ErrSystemUpdate
		}

		if !ok {
			return ErrorWithParams(ErrSystemNotFound, "externalID", in.GetExternalId(), "type", in.GetType())
		}

		return nil
	})

	err = mapError(err)
	if err != nil {
		return nil, err
	}

	return &mappinggrpc.UnmapSystemFromTenantResponse{Success: true}, nil
}

// MapSystemToTenant links Systems to the Tenant.
func (m *Mapping) MapSystemToTenant(ctx context.Context, in *mappinggrpc.MapSystemToTenantRequest) (*mappinggrpc.MapSystemToTenantResponse, error) {
	tenantID := in.GetTenantId()
	slogctx.Debug(ctx, "MapSystemToTenant called", "tenant_id", tenantID)

	if err := m.validateMapRequest(in); err != nil {
		return nil, err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := m.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		system, found, err := isSystemTenantMapAllowed(ctx, r, in)
		if err != nil {
			return err
		}

		if !found {
			_, err = createSystem(ctx, m.validation, r, in.GetExternalId(), in.GetType(), tenantID)
			return err
		}

		system.TenantID = &tenantID
		_, err = r.Patch(ctx, system)
		if err != nil {
			return ErrSystemUpdate
		}

		return nil
	})

	err = mapError(err)
	if err != nil {
		return nil, err
	}

	return &mappinggrpc.MapSystemToTenantResponse{Success: true}, nil
}

// Get gets the mapped tenant from the system.
func (m *Mapping) Get(ctx context.Context, in *mappinggrpc.GetRequest) (*mappinggrpc.GetResponse, error) {
	slogctx.Debug(ctx, "Get called")

	if err := validateExternalIDAndType(m.validation, in.GetExternalId(), in.GetType()); err != nil {
		return nil, err
	}

	system, found, err := getSystem(ctx, m.repo, in.GetExternalId(), in.GetType())
	if err != nil {
		return nil, ErrSystemSelect
	}

	if !found {
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

// validateAndGetSystemForUnmap fetched and returns the system it also validates
// iIt checks if the tenantID matches and if the tenant is active and it checks for the regional systems validity.
func validateAndGetSystemForUnmap(ctx context.Context, r repository.Repository, in *mappinggrpc.UnmapSystemFromTenantRequest) (*model.System, error) {
	tenantID := in.GetTenantId()

	system, found, err := getSystem(ctx, r, in.GetExternalId(), in.GetType())
	if err != nil {
		return nil, ErrSystemSelect
	}
	if !found {
		return nil, ErrSystemNotFound
	}

	if !system.IsLinkedToTenant() {
		return nil, ErrorWithParams(ErrSystemIsNotLinkedToTenant, "externalID", system.ExternalID, "type", system.Type)
	}

	if *system.TenantID != tenantID {
		return nil, ErrorWithParams(ErrSystemIsNotLinkedToTenant, "externalID", system.ExternalID, "type", system.Type)
	}

	tenant, err := getTenant(ctx, r, *system.TenantID)
	if err != nil {
		return nil, err
	}

	err = checkTenantActive(tenant)
	if err != nil {
		return nil, err
	}

	if err := validateRegionalSystemsForUnmap(ctx, r, system); err != nil {
		return nil, err
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
// It returns nil if the provided Tenant exist, the System is found and no linked, and HasL1KeyClaim is false.
func isSystemTenantMapAllowed(ctx context.Context, r repository.Repository, in *mappinggrpc.MapSystemToTenantRequest) (*model.System, bool, error) {
	tenant, err := getTenant(ctx, r, in.GetTenantId())
	if err != nil {
		return nil, false, err
	}

	err = checkTenantActive(tenant)
	if err != nil {
		return nil, false, err
	}

	system, found, err := getSystem(ctx, r, in.GetExternalId(), in.GetType())
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	// For linking, each system must not be already linked and must not have an active L1 key claim.
	if system.IsLinkedToTenant() {
		return system, found, ErrorWithParams(ErrSystemIsLinkedToTenant, "externalID", system.ExternalID, "type", system.Type)
	}

	if err := validateRegionalSystemsForLink(ctx, r, system); err != nil {
		return system, found, err
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
			return err
		}

		if s.HasL1KeyClaim != nil && *s.HasL1KeyClaim {
			return ErrorWithParams(ErrSystemHasL1KeyClaim, "externalID", system.ExternalID, "type", system.Type, "region", s.Region)
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
