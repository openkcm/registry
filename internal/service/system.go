package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
)

// System implements the procedure calls defined as protobufs.
// See https://github.com/openkcm/api-sdk/blob/main/proto/kms/api/cmk/registry/system/v1/system.proto.
type System struct {
	systemgrpc.UnimplementedServiceServer

	repo   repository.Repository
	meters *Meters
}

type SystemUpdateFunc func(system *model.System) error

// NewSystem creates and return a new instance of System.
func NewSystem(repo repository.Repository, meters *Meters) *System {
	return &System{
		repo:   repo,
		meters: meters,
	}
}

// RegisterSystem handles the creation of a new System. The response contains the created System's ID.
func (s *System) RegisterSystem(ctx context.Context, in *systemgrpc.RegisterSystemRequest) (*systemgrpc.RegisterSystemResponse, error) {
	slogctx.Debug(ctx, "RegisterSystem called", "external_id", in.GetExternalId(), "region", in.GetRegion(), "tenant_id", in.GetTenantId(), "system_type", in.GetType(), "status", in.GetStatus().String())
	claim := in.GetHasL1KeyClaim()

	tenantID := in.GetTenantId()
	if tenantID != "" {
		err := assertTenantExist(ctx, s.repo, tenantID)
		if err != nil {
			return nil, err
		}
	}

	systemType := model.SystemTypeSystem
	if in.GetType() != "" {
		systemType = model.SystemType(in.GetType())
	}

	system := &model.System{
		ExternalID:    model.ExternalID(in.GetExternalId()),
		TenantID:      &tenantID,
		L2KeyID:       model.L2KeyID(in.GetL2KeyId()),
		HasL1KeyClaim: &claim,
		Status:        model.Status(in.GetStatus().String()),
		Region:        model.Region(in.GetRegion()),
		Type:          systemType,
		Labels:        in.GetLabels(),
	}

	err := system.Validate()
	if err != nil {
		return nil, err
	}

	err = s.repo.Create(ctx, system)
	if err != nil {
		return nil, err
	}

	s.meters.handleSystemRegistration(ctx, system.Region.String())

	return &systemgrpc.RegisterSystemResponse{
		Success: true,
	}, nil
}

// ListSystems retrieves a list of Systems based on optional query parameters such as tenant_id. region and external_id
// To retrieve sSystems one of tenant_id or a combination of region and external_id must be provided.
//
//nolint:cyclop
func (s *System) ListSystems(ctx context.Context, in *systemgrpc.ListSystemsRequest) (*systemgrpc.ListSystemsResponse, error) {
	slogctx.Debug(ctx, "ListSystems called", "external_id", in.GetExternalId(), "region", in.GetRegion(), "tenant_id", in.GetTenantId())

	if in.GetExternalId() == "" && in.GetTenantId() == "" {
		return nil, ErrSystemListNotAllowed
	}

	query := repository.NewQuery(&model.System{})

	err := query.ApplyPagination(in.GetLimit(), in.GetPageToken())
	if err != nil {
		return nil, err
	}

	cond := repository.NewCompositeKey()
	if in.GetExternalId() != "" {
		cond.Where(repository.ExternalIDField, in.GetExternalId())
	}

	if in.GetTenantId() != "" {
		cond.Where(repository.TenantIDField, in.GetTenantId())
	}

	if in.GetRegion() != "" {
		cond.Where(repository.RegionField, in.GetRegion())
	}

	if in.GetType() != "" {
		cond.Where(repository.TypeField, in.GetType())
	}

	query.Where(cond)

	var systems []model.System
	if err := s.repo.List(ctx, &systems, *query); err != nil {
		return nil, err
	}

	pbSystems := make([]*systemgrpc.System, 0, len(systems))
	for _, system := range systems {
		pbSystems = append(pbSystems, system.ToProto())
	}

	if len(pbSystems) == 0 {
		return nil, ErrSystemNotFound
	}

	if len(systems) < query.Limit {
		return &systemgrpc.ListSystemsResponse{
			Systems: pbSystems,
		}, nil
	}

	lastItem := systems[len(systems)-1]

	nextToken, err := repository.PageInfo{
		LastCreatedAt: lastItem.CreatedAt,
		LastKey:       lastItem.PaginationKey(),
	}.Encode()
	if err != nil {
		return nil, err
	}

	return &systemgrpc.ListSystemsResponse{
		Systems:       pbSystems,
		NextPageToken: nextToken,
	}, nil
}

// DeleteSystem handles the deletion of a new System. The response contains deletion status and error if failed.
func (s *System) DeleteSystem(ctx context.Context, in *systemgrpc.DeleteSystemRequest) (*systemgrpc.DeleteSystemResponse, error) {
	slogctx.Debug(ctx, "DeleteSystem called", "external_id", in.GetExternalId(), "region", in.GetRegion())

	if in.GetExternalId() == "" {
		return nil, ErrExternalIDIsEmpty
	} else if in.GetRegion() == "" {
		return nil, ErrRegionIsEmpty
	}

	system := &model.System{
		ExternalID: model.ExternalID(in.GetExternalId()),
		Region:     model.Region(in.GetRegion()),
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	var systemFound bool

	err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		err := validateDeleteSystem(ctx, r, system)
		if err != nil {
			slog.Error(DeleteSystemErrMsg, "err", err.Error())
			return err
		}

		if systemFound, err = r.Delete(ctx, system); err != nil {
			return ErrSystemDelete
		}

		return nil
	})

	err = mapError(err)
	if err != nil {
		return nil, err
	}

	if systemFound {
		s.meters.handleSystemDeletion(ctx, system.Region.String())
	}

	return &systemgrpc.DeleteSystemResponse{Success: true}, nil
}

// UpdateSystemL1KeyClaim updates the l1_key_claim parameter of the System identified by its system_id.
func (s *System) UpdateSystemL1KeyClaim(ctx context.Context, in *systemgrpc.UpdateSystemL1KeyClaimRequest) (*systemgrpc.UpdateSystemL1KeyClaimResponse, error) {
	slogctx.Debug(ctx, "UpdateSystemL1KeyClaim called", "external_id", in.GetExternalId(), "region", in.GetRegion(), "key_claim", in.GetL1KeyClaim(), "tenant_id", in.GetTenantId())

	if in.GetExternalId() == "" {
		return nil, ErrExternalIDIsEmpty
	} else if in.GetRegion() == "" {
		return nil, ErrRegionIsEmpty
	}

	desiredClaim := in.GetL1KeyClaim()

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		system := &model.System{
			ExternalID: model.ExternalID(in.GetExternalId()),
			Region:     model.Region(in.GetRegion()),
		}

		found, err := r.Find(ctx, system)
		if err != nil || !found {
			return ErrSystemNotFound
		}

		if err = s.isUpdateKeyClaimAllowed(system, desiredClaim, in.GetTenantId()); err != nil {
			return err
		}

		isPatched, err := r.Patch(ctx, &model.System{
			ExternalID:    model.ExternalID(in.GetExternalId()),
			Region:        model.Region(in.GetRegion()),
			HasL1KeyClaim: &desiredClaim,
		})
		if err != nil || !isPatched {
			return ErrSystemUpdate
		}

		return nil
	})

	err = mapError(err)
	if err != nil {
		return nil, err
	}

	return &systemgrpc.UpdateSystemL1KeyClaimResponse{Success: true}, nil
}

// UnlinkSystemsFromTenant unlinks Systems from the Tenant.
func (s *System) UnlinkSystemsFromTenant(ctx context.Context, in *systemgrpc.UnlinkSystemsFromTenantRequest) (*systemgrpc.UnlinkSystemsFromTenantResponse, error) {
	slogctx.Debug(ctx, "UnlinkSystemsFromTenant called")

	emptyTenantID := ""

	systems, uniqMap, err := validateAndGetSystems(in.GetSystemIdentifiers())
	if err != nil {
		return nil, err
	}

	query := generateQuery(systems...)

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	updatedSystems := make([]model.System, 0, len(systems))
	err = s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		if err := isSystemTenantUnlinkAllowed(ctx, r, uniqMap, query); err != nil {
			return err
		}

		_, err := r.PatchAll(ctx, &model.System{
			TenantID: &emptyTenantID,
		}, &updatedSystems, *query)
		if err != nil {
			return ErrSystemUpdate
		}

		return checkForMissingSystems(uniqMap, updatedSystems)
	})

	err = mapError(err)
	if err != nil {
		return nil, err
	}

	return &systemgrpc.UnlinkSystemsFromTenantResponse{Success: true}, nil
}

// LinkSystemsToTenant links Systems to the Tenant.
func (s *System) LinkSystemsToTenant(ctx context.Context, in *systemgrpc.LinkSystemsToTenantRequest) (*systemgrpc.LinkSystemsToTenantResponse, error) {
	slogctx.Debug(ctx, "LinkSystemsToTenant called", "tenant_id", in.GetTenantId())
	tenantID := in.GetTenantId()

	systems, uniqMap, err := validateAndGetSystems(in.GetSystemIdentifiers())
	if err != nil {
		return nil, err
	}

	query := generateQuery(systems...)

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	updatedSystems := make([]model.System, 0, len(systems))
	err = s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		err := assertTenantExist(ctx, r, tenantID)
		if err != nil {
			return err
		}

		err = isSystemTenantLinkAllowed(ctx, r, model.ID(tenantID), uniqMap, query)
		if err != nil {
			return err
		}

		_, err = r.PatchAll(ctx, &model.System{
			TenantID: &tenantID,
		}, &updatedSystems, *query)
		if err != nil {
			return ErrSystemUpdate
		}

		return checkForMissingSystems(uniqMap, updatedSystems)
	})

	err = mapError(err)
	if err != nil {
		return nil, err
	}

	return &systemgrpc.LinkSystemsToTenantResponse{Success: true}, nil
}

// UpdateSystemStatus updates the status of the System identified by its ID.
// The status can be one of a predefined set of values.
// If the update is successful, a success message will be returned, otherwise an error will be returned.
func (s *System) UpdateSystemStatus(ctx context.Context, in *systemgrpc.UpdateSystemStatusRequest) (*systemgrpc.UpdateSystemStatusResponse, error) {
	slogctx.Debug(ctx, "UpdateSystemStatus called", "external_id", in.GetExternalId(), "region", in.GetRegion(), "status", in.GetStatus())

	if in.GetExternalId() == "" {
		return nil, ErrExternalIDIsEmpty
	} else if in.GetRegion() == "" {
		return nil, ErrRegionIsEmpty
	}

	if err := model.Status(in.GetStatus().String()).Validate(); err != nil {
		return nil, err
	}

	isPatched, err := s.repo.Patch(ctx, &model.System{
		ExternalID: model.ExternalID(in.GetExternalId()),
		Region:     model.Region(in.GetRegion()),
		Status:     model.Status(in.GetStatus().String()),
	})
	if err != nil {
		return nil, ErrSystemUpdate
	}

	if !isPatched {
		return nil, ErrSystemNotFound
	}

	return &systemgrpc.UpdateSystemStatusResponse{Success: true}, nil
}

// SetSystemLabels sets the labels for the System identified by its external ID and region.
// Existing labels with the same keys will be overwritten.
// If the update is successful, a success message will be returned, otherwise an error will be returned.
func (s *System) SetSystemLabels(ctx context.Context, in *systemgrpc.SetSystemLabelsRequest) (*systemgrpc.SetSystemLabelsResponse, error) {
	slogctx.Debug(ctx, "SetSystemLabels called", "external_id", in.GetExternalId(), "region", in.GetRegion())

	if err := s.validateSetSystemLabelsRequest(in); err != nil {
		return nil, err
	}

	err := s.patchSystem(ctx, model.ExternalID(in.GetExternalId()), model.Region(in.GetRegion()), func(system *model.System) error {
		if system.Labels == nil {
			system.Labels = make(model.Labels)
		}

		maps.Copy(system.Labels, in.GetLabels())

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &systemgrpc.SetSystemLabelsResponse{
		Success: true,
	}, nil
}

// RemoveSystemLabels removes the specified labels from the System identified by its external ID and region.
// If one or more label keys are not found, they will be ignored.
// If the update is successful, a success message will be returned, otherwise an error will be returned.
func (s *System) RemoveSystemLabels(ctx context.Context, in *systemgrpc.RemoveSystemLabelsRequest) (*systemgrpc.RemoveSystemLabelsResponse, error) {
	slogctx.Debug(ctx, "RemoveSystemLabels called", "external_id", in.GetExternalId(), "region", in.GetRegion())

	if err := s.validateRemoveSystemLabelsRequest(in); err != nil {
		return nil, err
	}

	err := s.patchSystem(ctx, model.ExternalID(in.GetExternalId()), model.Region(in.GetRegion()), func(system *model.System) error {
		for _, k := range in.GetLabelKeys() {
			delete(system.Labels, k)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &systemgrpc.RemoveSystemLabelsResponse{
		Success: true,
	}, nil
}

// validateSetSystemLabelsRequest validates the SetSystemLabelsRequest.
// If the request is valid, it returns nil, otherwise it returns an error.
func (s *System) validateSetSystemLabelsRequest(in *systemgrpc.SetSystemLabelsRequest) error {
	if in.GetExternalId() == "" {
		return ErrExternalIDIsEmpty
	}

	if in.GetRegion() == "" {
		return ErrRegionIsEmpty
	}

	if len(in.GetLabels()) == 0 {
		return ErrMissingLabels
	}

	labels := model.Labels(in.GetLabels())

	err := labels.Validate()
	if err != nil {
		return err
	}

	return nil
}

// validateRemoveSystemLabelsRequest validates the RemoveSystemLabelsRequest.
// If the request is valid, it returns nil, otherwise it returns an error.
func (s *System) validateRemoveSystemLabelsRequest(in *systemgrpc.RemoveSystemLabelsRequest) error {
	if in.GetExternalId() == "" {
		return ErrExternalIDIsEmpty
	}

	if in.GetRegion() == "" {
		return ErrRegionIsEmpty
	}

	if len(in.GetLabelKeys()) == 0 {
		return ErrMissingLabelKeys
	}

	if slices.Contains(in.GetLabelKeys(), "") {
		return ErrEmptyLabelKeys
	}

	return nil
}

// patchSystem retrieves the System by its external ID and region, applies the update function to it,
// and then updates the System in the repository.
// It returns an error if the System is not found, if it is unavailable, or if the update fails.
func (s *System) patchSystem(ctx context.Context, externalID model.ExternalID, region model.Region, updateFunc SystemUpdateFunc) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		system := &model.System{
			ExternalID: externalID,
			Region:     region,
		}

		system, err := getSystem(ctx, r, system)
		if err != nil {
			return err
		}

		if err = checkSystemAvailable(system); err != nil {
			return err
		}

		if err = updateFunc(system); err != nil {
			return err
		}

		isPatched, err := r.Patch(ctx, system)
		if err != nil {
			return ErrSystemUpdate
		}

		if !isPatched {
			return ErrSystemNotFound
		}

		return nil
	})

	err = mapError(err)
	if err != nil {
		return err
	}

	return nil
}

// validateDeleteSystem makes sure that the System is allowed to be deleted.
// Here repository r is passed as a variable to address the scenarios where we will
// create a new repository from the existing repository for e.g. in the case of transaction.
func validateDeleteSystem(ctx context.Context, r repository.Repository, system *model.System) error {
	system, err := getSystem(ctx, r, system)
	if errors.Is(err, ErrSystemNotFound) {
		slog.Info(fmt.Sprintf("%s:%s", DeleteSystemErrMsg, SystemNotFoundMsg))
		return nil
	} else if err != nil {
		return err
	}

	err = checkSystemAvailable(system)
	if err != nil {
		return err
	}

	if system.IsLinkedToTenant() {
		return ErrSystemIsLinkedToTenant
	}

	return nil
}

// isSystemTenantUnlinkAllowed checks whether all conditions are met to unlink the Tenant from the System.
// Here repository r is passed as a variable to address the scenarios where we will
// create a new repository from the existing repository for e.g. in the case of transaction.
func isSystemTenantUnlinkAllowed(ctx context.Context, r repository.Repository, uniqMap map[string]*model.System, query *repository.Query) error {
	systems := make([]model.System, 0, len(uniqMap))

	err := r.List(ctx, &systems, *query)
	if err != nil {
		return ErrSystemSelect
	}

	// since we are searching based on primary key the number of returned elements should be equal to the number of ids used
	err = checkForMissingSystems(uniqMap, systems)
	if err != nil {
		return err
	}

	// For unlinking, every system must be linked and must not have an active L1 key claim.
	for _, system := range systems {
		if err := checkSystemAvailable(&system); err != nil {
			return err
		}

		if !system.IsLinkedToTenant() {
			return ErrorWithParams(ErrSystemIsNotLinkedToTenant, "externalID", system.ExternalID, "region", system.Region)
		}

		if system.HasActiveL1KeyClaim() {
			return ErrorWithParams(ErrSystemHasL1KeyClaim, "externalID", system.ExternalID, "region", system.Region)
		}

		tenant, err := getTenant(ctx, r, model.ID(*system.TenantID))
		if err != nil {
			return err
		}

		err = checkTenantActive(tenant)
		if err != nil {
			return err
		}
	}

	return nil
}

// isSystemTenantLinkAllowed checks whether all conditions are met to link the Tenant.
// It returns nil if the provided Tenant exist, the System is found and no linked, and HasL1KeyClaim is false.
// Here repository r is passed as a variable to address the scenarios where we will
// create a new repository from the existing repository for e.g. in the case of transaction.
func isSystemTenantLinkAllowed(ctx context.Context, r repository.Repository, tenantID model.ID, uniqMap map[string]*model.System, query *repository.Query) error {
	tenant, err := getTenant(ctx, r, tenantID)
	if err != nil {
		return err
	}

	err = checkTenantActive(tenant)
	if err != nil {
		return err
	}

	systems := make([]model.System, 0, len(uniqMap))
	if err := r.List(ctx, &systems, *query); err != nil {
		return ErrSystemSelect
	}

	// since we are searching based on primary key the number of returned elements should be equal to the number of ids used
	if err := checkForMissingSystems(uniqMap, systems); err != nil {
		return err
	}

	// For linking, each system must not be already linked and must not have an active L1 key claim.
	for _, system := range systems {
		err := checkSystemAvailable(&system)
		if err != nil {
			return err
		}

		if system.IsLinkedToTenant() {
			return ErrorWithParams(ErrSystemIsLinkedToTenant, "externalID", system.ExternalID, "region", system.Region)
		}

		if system.HasL1KeyClaim != nil && *system.HasL1KeyClaim {
			return ErrorWithParams(ErrSystemHasL1KeyClaim, "externalID", system.ExternalID, "region", system.Region)
		}
	}

	return nil
}

// isUpdateKeyClaimAllowed checks whether all conditions are met to update the KeyClaim.
func (s *System) isUpdateKeyClaimAllowed(system *model.System, desiredClaim bool, tenantID string) error {
	err := checkSystemAvailable(system)
	if err != nil {
		return err
	}

	if desiredClaim == system.HasActiveL1KeyClaim() {
		if desiredClaim {
			return ErrKeyClaimAlreadyActive
		}

		return ErrKeyClaimAlreadyInactive
	}

	if !system.IsLinkedToTenant() || *system.TenantID != tenantID {
		return ErrSystemIsNotLinkedToTenant
	}

	return nil
}

// getSystem queries the System by its ID.
func getSystem(ctx context.Context, r repository.Repository, system *model.System) (*model.System, error) {
	found, err := r.Find(ctx, system)
	if err != nil {
		return nil, ErrSystemSelect
	}

	if !found {
		return nil, ErrSystemNotFound
	}

	return system, nil
}

// checkSystemAvailable returns nil if System has status Available.
func checkSystemAvailable(system *model.System) error {
	if available, err := system.Status.IsAvailable(); err != nil {
		return err
	} else if !available {
		return ErrSystemUnavailable
	}

	return nil
}

// validateAndGetSystems validates the input slice of SystemId and returns a slice of model.System having only unique systems.
func validateAndGetSystems(in []*systemgrpc.SystemIdentifier) ([]*model.System, map[string]*model.System, error) {
	if len(in) == 0 {
		return nil, nil, ErrNoSystemIdentifiers
	}

	uniqMap := make(map[string]*model.System)
	systems := make([]*model.System, 0, len(in))

	for _, system := range in {
		if system.GetExternalId() == "" {
			return nil, nil, ErrExternalIDIsEmpty
		} else if system.GetRegion() == "" {
			return nil, nil, ErrRegionIsEmpty
		}

		if _, ok := uniqMap[fmt.Sprintf("%s.%s", system.GetExternalId(), system.GetRegion())]; !ok {
			sys := &model.System{
				ExternalID: model.ExternalID(system.GetExternalId()),
				Region:     model.Region(system.GetRegion()),
			}
			systems = append(systems, sys)
			uniqMap[fmt.Sprintf("%s.%s", system.GetExternalId(), system.GetRegion())] = sys
		}
	}

	return systems, uniqMap, nil
}

// generateQuery generates a query based on the provided systems by adding their composite keys to the query.
func generateQuery(systems ...*model.System) *repository.Query {
	query := repository.NewQuery(&model.System{})

	for _, s := range systems {
		query.Where(repository.NewCompositeKey().
			Where(repository.ExternalIDField, s.ExternalID).
			Where(repository.RegionField, s.Region),
		)
	}

	return query.SetLimit(len(systems))
}

// checkForMissingSystems checks if all systems in the uniqMap are present in the systems slice returned from the DB.
func checkForMissingSystems(uniqMap map[string]*model.System, systems []model.System) error {
	if len(systems) == len(uniqMap) {
		return nil
	}

	systemsMap := make(map[string]model.System)
	for _, system := range systems {
		systemsMap[fmt.Sprintf("%s.%s", system.ExternalID, system.Region)] = system
	}

	for key, val := range uniqMap {
		if _, ok := systemsMap[key]; !ok {
			return ErrorWithParams(ErrSystemNotFound, "externalID", val.ExternalID, "region", val.Region)
		}
	}

	return nil
}
