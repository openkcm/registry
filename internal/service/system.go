package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/google/uuid"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

// System implements the procedure calls defined as protobufs.
// See https://github.com/openkcm/api-sdk/blob/main/proto/kms/api/cmk/registry/system/v1/system.proto.
type System struct {
	systemgrpc.UnimplementedServiceServer

	repo       repository.Repository
	meters     *Meters
	validation *validation.Validation
}

type SystemUpdateFunc func(system *model.RegionalSystem) error

// NewSystem creates and return a new instance of System.
func NewSystem(repo repository.Repository, meters *Meters, validation *validation.Validation) *System {
	return &System{
		repo:       repo,
		meters:     meters,
		validation: validation,
	}
}

// RegisterSystem handles the creation of a new System. The response contains the created System's ID.
func (s *System) RegisterSystem(ctx context.Context, in *systemgrpc.RegisterSystemRequest) (*systemgrpc.RegisterSystemResponse, error) {
	slogctx.Debug(ctx, "RegisterSystem called", "external_id", in.GetExternalId(), "region", in.GetRegion(), "tenant_id", in.GetTenantId(), "system_type", in.GetType(), "status", in.GetStatus().String())

	regionalSystem := &model.RegionalSystem{
		L2KeyID:       in.GetL2KeyId(),
		HasL1KeyClaim: &in.HasL1KeyClaim,
		Status:        in.GetStatus().String(),
		Region:        in.GetRegion(),
		Labels:        in.GetLabels(),
	}

	if err := s.validateRegionalSystem(regionalSystem); err != nil {
		return nil, err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	if err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		system, found, err := getSystemByIdentifier(ctx, r, in.GetExternalId(), in.GetType())
		if err != nil {
			return ErrSystemSelect
		}

		if found && system.TenantID != nil && in.GetTenantId() != *system.TenantID {
			return ErrRegisterSystemNotAllowedWithTenantID
		}

		if !found {
			system, err = s.createSystem(ctx, r, in.GetExternalId(), in.GetType(), in.GetTenantId())
			if err != nil {
				return err
			}
		}

		regionalSystem.SystemID = system.ID

		return r.Create(ctx, regionalSystem)
	}); err != nil {
		return nil, err
	}

	s.meters.handleSystemRegistration(ctx, regionalSystem.Region)

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

	query := repository.NewQuery(&model.RegionalSystem{})

	err := query.ApplyPagination(in.GetLimit(), in.GetPageToken())
	if err != nil {
		return nil, err
	}

	cond := repository.NewCompositeKey()

	system := &model.System{}
	regionalSystem := &model.RegionalSystem{}

	query.Joins = []repository.Join{
		{
			Resource: system,
			OnColumn: repository.IDField,
			Column:   repository.SystemIDField,
		},
	}

	if in.GetExternalId() != "" {
		fieldAfterJoin := fmt.Sprintf("%s.%s", system.TableName(), repository.ExternalIDField)
		cond.Where(fieldAfterJoin, in.GetExternalId())
	}

	if in.GetTenantId() != "" {
		fieldAfterJoin := fmt.Sprintf("%s.%s", system.TableName(), repository.TenantIDField)
		cond.Where(fieldAfterJoin, in.GetTenantId())
	}

	if in.GetRegion() != "" {
		fieldAfterJoin := fmt.Sprintf("%s.%s", regionalSystem.TableName(), repository.RegionField)
		cond.Where(fieldAfterJoin, in.GetRegion())
	}

	if in.GetType() != "" {
		fieldAfterJoin := fmt.Sprintf("%s.%s", system.TableName(), repository.TypeField)
		cond.Where(fieldAfterJoin, in.GetType())
	}

	query.Where(cond)
	query.Populate(repository.System)

	var systems []model.RegionalSystem
	if err := s.repo.List(ctx, &systems, *query); err != nil {
		return nil, err
	}

	pbSystems := make([]*systemgrpc.System, 0, len(systems))
	for _, system := range systems {
		systemProto, err := system.ToProto()
		if err != nil {
			return nil, ErrSystemProtoConversion
		}
		pbSystems = append(pbSystems, systemProto)
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
	slogctx.Debug(ctx, "DeleteSystem called", "system_identifier", in.GetSystemIdentifier(), "region", in.GetRegion())

	err := s.validateIdentifier(in.GetSystemIdentifier())
	if err != nil {
		return nil, err
	}

	system, found, err := getSystemByIdentifier(ctx, s.repo, in.SystemIdentifier.GetExternalId(), in.SystemIdentifier.GetType())
	if err != nil {
		return nil, err
	}
	if !found {
		return &systemgrpc.DeleteSystemResponse{
			Success: true,
		}, nil
	}

	regionalSystem := &model.RegionalSystem{
		SystemID: system.ID,
		Region:   in.GetRegion(),
	}

	query := repository.NewQuery(&model.RegionalSystem{})
	cond := repository.NewCompositeKey()
	cond.Where(repository.SystemIDField, system.ID.String())
	query.Where(cond)

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	var systemFound bool
	err = s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		err = validateDeleteSystem(ctx, r, regionalSystem)
		if err != nil {
			slog.Error(DeleteSystemErrMsg, "err", err.Error())
			return err
		}

		if systemFound, err = r.Delete(ctx, regionalSystem); err != nil {
			return ErrSystemDelete
		}

		var regionalSystems []model.RegionalSystem
		if err = r.List(ctx, &regionalSystems, *query); err != nil {
			return err
		}

		if len(regionalSystems) > 0 {
			return nil
		}

		_, err = r.Delete(ctx, system)

		return err
	})

	err = mapError(err)
	if err != nil {
		return nil, err
	}

	if systemFound {
		s.meters.handleSystemDeletion(ctx, regionalSystem.Region)
	}

	return &systemgrpc.DeleteSystemResponse{Success: true}, nil
}

// UpdateSystemL1KeyClaim updates the l1_key_claim parameter of the System identified by its system_id.
func (s *System) UpdateSystemL1KeyClaim(ctx context.Context, in *systemgrpc.UpdateSystemL1KeyClaimRequest) (*systemgrpc.UpdateSystemL1KeyClaimResponse, error) {
	slogctx.Debug(ctx, "UpdateSystemL1KeyClaim called", "system_identifier", in.GetSystemIdentifier(), "region", in.GetRegion(), "key_claim", in.GetL1KeyClaim(), "tenant_id", in.GetTenantId())

	err := s.validateIdentifier(in.GetSystemIdentifier())
	if err != nil {
		return nil, err
	}

	system, found, err := getSystemByIdentifier(ctx, s.repo, in.SystemIdentifier.GetExternalId(), in.SystemIdentifier.GetType())
	if err != nil {
		return nil, ErrSystemSelect
	}
	if !found {
		return nil, ErrSystemNotFound
	}

	regionalSystem := &model.RegionalSystem{
		SystemID: system.ID,
		Region:   in.GetRegion(),
	}

	desiredClaim := in.GetL1KeyClaim()

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err = s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		err := getRegionalSystem(ctx, r, regionalSystem, true)
		if err != nil {
			return err
		}

		if err = s.isUpdateKeyClaimAllowed(regionalSystem, desiredClaim, in.GetTenantId()); err != nil {
			return err
		}

		regionalSystem.HasL1KeyClaim = &desiredClaim
		isPatched, err := r.Patch(ctx, regionalSystem)
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

	systems, uniqMap, err := s.validateAndGetSystems(in.GetSystemIdentifiers())
	if err != nil {
		return nil, err
	}

	query := generateQuery(systems...)

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	updatedSystems := make([]model.System, 0, len(systems))
	err = s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		if err = isSystemTenantUnlinkAllowed(ctx, r, uniqMap, query); err != nil {
			return err
		}

		_, err = r.PatchAll(ctx, &model.System{
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

	systems, uniqMap, err := s.validateAndGetSystems(in.GetSystemIdentifiers())
	if err != nil {
		return nil, err
	}

	query := generateQuery(systems...)

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	updatedSystems := make([]model.System, 0, len(systems))
	err = s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		if err = assertTenantExist(ctx, r, tenantID); err != nil {
			return err
		}

		if err = isSystemTenantLinkAllowed(ctx, r, tenantID, uniqMap, query); err != nil {
			return err
		}

		_, err = r.PatchAll(ctx, &model.System{
			TenantID: &tenantID,
		}, &updatedSystems, *query)
		if err != nil {
			return ErrSystemUpdate
		}

		missingSystems := getMissingSystems(uniqMap, updatedSystems)
		for _, sys := range missingSystems {
			sys.TenantID = &tenantID
			if err = r.Create(ctx, sys); err != nil {
				return err
			}
		}

		return nil
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
	slogctx.Debug(ctx, "UpdateSystemStatus called", "system_identifier", in.GetSystemIdentifier(), "region", in.GetRegion(), "status", in.GetStatus())

	err := s.validateIdentifier(in.GetSystemIdentifier())
	if err != nil {
		return nil, err
	}
	err = s.validation.Validate(model.SystemStatusValidationID, in.GetStatus().String())
	if err != nil {
		return nil, ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}

	system, found, err := getSystemByIdentifier(ctx, s.repo, in.SystemIdentifier.GetExternalId(), in.SystemIdentifier.GetType())
	if err != nil {
		return nil, ErrSystemSelect
	}
	if !found {
		return nil, ErrSystemNotFound
	}

	isPatched, err := s.repo.Patch(ctx, &model.RegionalSystem{
		SystemID: system.ID,
		Region:   in.GetRegion(),
		Status:   in.GetStatus().String(),
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
	slogctx.Debug(ctx, "SetSystemLabels called", "system_identifier", in.GetSystemIdentifier(), "region", in.GetRegion())

	if err := s.validateSetSystemLabelsRequest(in); err != nil {
		return nil, err
	}

	system, found, err := getSystemByIdentifier(ctx, s.repo, in.SystemIdentifier.GetExternalId(), in.SystemIdentifier.GetType())
	if err != nil {
		return nil, ErrSystemSelect
	}
	if !found {
		return nil, ErrSystemNotFound
	}

	if err = s.patchSystem(ctx, system.ID, in.GetRegion(), func(regionalSystem *model.RegionalSystem) error {
		if regionalSystem.Labels == nil {
			regionalSystem.Labels = make(model.Labels)
		}

		maps.Copy(regionalSystem.Labels, in.GetLabels())

		return nil
	}); err != nil {
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
	slogctx.Debug(ctx, "RemoveSystemLabels called", "system_identifier", in.GetSystemIdentifier(), "region", in.GetRegion())

	if err := s.validateRemoveSystemLabelsRequest(in); err != nil {
		return nil, err
	}

	system, found, err := getSystemByIdentifier(ctx, s.repo, in.SystemIdentifier.GetExternalId(), in.SystemIdentifier.GetType())
	if err != nil {
		return nil, ErrSystemSelect
	}
	if !found {
		return nil, ErrSystemNotFound
	}

	if err = s.patchSystem(ctx, system.ID, in.GetRegion(), func(regionalSystem *model.RegionalSystem) error {
		for _, k := range in.GetLabelKeys() {
			delete(regionalSystem.Labels, k)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &systemgrpc.RemoveSystemLabelsResponse{
		Success: true,
	}, nil
}

func (s *System) validateRegionalSystem(system *model.RegionalSystem) error {
	values, err := validation.GetValues(system)
	if err != nil {
		return ErrorWithParams(ErrValidationConversion, "err", err.Error())
	}

	err = s.validation.ValidateAll(values)
	if err != nil {
		return ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}

	return nil
}

func (s *System) validateSystem(system *model.System) error {
	values, err := validation.GetValues(system)
	if err != nil {
		return ErrorWithParams(ErrValidationConversion, "err", err.Error())
	}

	err = s.validation.ValidateAll(values)
	if err != nil {
		return ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}

	return nil
}

func (s *System) validateIdentifier(identifier *systemgrpc.SystemIdentifier) error {
	if identifier == nil {
		return ErrNoSystemIdentifiers
	}

	err := s.validation.ValidateAll(map[validation.ID]any{
		model.SystemExternalIDValidationID: identifier.GetExternalId(),
		model.SystemTypeValidationID:       identifier.GetType(),
	})
	if err != nil {
		return ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}

	return nil
}

// validateSetSystemLabelsRequest validates the SetSystemLabelsRequest.
// If the request is valid, it returns nil, otherwise it returns an error.
func (s *System) validateSetSystemLabelsRequest(in *systemgrpc.SetSystemLabelsRequest) error {
	err := s.validateIdentifier(in.GetSystemIdentifier())
	if err != nil {
		return err
	}

	err = s.validation.ValidateAll(map[validation.ID]any{
		model.RegionalSystemRegionValidationID: in.GetRegion(),
	})
	if err != nil {
		return ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}

	if len(in.GetLabels()) == 0 {
		return ErrMissingLabels
	}

	labels := model.Labels(in.GetLabels())
	err = labels.Validate()
	if err != nil {
		return err
	}

	return nil
}

// validateRemoveSystemLabelsRequest validates the RemoveSystemLabelsRequest.
// If the request is valid, it returns nil, otherwise it returns an error.
func (s *System) validateRemoveSystemLabelsRequest(in *systemgrpc.RemoveSystemLabelsRequest) error {
	err := s.validateIdentifier(in.GetSystemIdentifier())
	if err != nil {
		return err
	}

	if err = s.validation.ValidateAll(map[validation.ID]any{
		model.RegionalSystemRegionValidationID: in.GetRegion(),
	}); err != nil {
		return ErrorWithParams(ErrValidationFailed, "err", err.Error())
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
func (s *System) patchSystem(ctx context.Context, id uuid.UUID, region string, updateFunc SystemUpdateFunc) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		system := &model.RegionalSystem{
			SystemID: id,
			Region:   region,
		}

		err := getRegionalSystem(ctx, r, system, false)
		if err != nil {
			return err
		}

		if err = checkRegionalSystemAvailable(system); err != nil {
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
func validateDeleteSystem(ctx context.Context, r repository.Repository, system *model.RegionalSystem) error {
	err := getRegionalSystem(ctx, r, system, true)
	if errors.Is(err, ErrSystemNotFound) {
		slog.Info(fmt.Sprintf("%s:%s", DeleteSystemErrMsg, SystemNotFoundMsg))
		return nil
	} else if err != nil {
		return err
	}

	err = checkRegionalSystemAvailable(system)
	if err != nil {
		return err
	}

	if system.System.IsLinkedToTenant() {
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
		if !system.IsLinkedToTenant() {
			return ErrorWithParams(ErrSystemIsNotLinkedToTenant, "externalID", system.ExternalID, "type", system.Type)
		}

		if err := validateRegionalSystemsForUnlink(ctx, r, &system); err != nil {
			return err
		}

		tenant, err := getTenant(ctx, r, *system.TenantID)
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

func validateRegionalSystemsForUnlink(ctx context.Context, r repository.Repository, system *model.System) error {
	regionalSystems, err := getRegionalSystemsForSystem(ctx, r, system.ID.String())
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

// isSystemTenantLinkAllowed checks whether all conditions are met to link the Tenant.
// It returns nil if the provided Tenant exist, the System is found and no linked, and HasL1KeyClaim is false.
// Here repository r is passed as a variable to address the scenarios where we will
// create a new repository from the existing repository for e.g. in the case of transaction.
func isSystemTenantLinkAllowed(ctx context.Context, r repository.Repository, tenantID string, uniqMap map[string]*model.System, query *repository.Query) error {
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

	// For linking, each system must not be already linked and must not have an active L1 key claim.
	for _, system := range systems {
		if system.IsLinkedToTenant() {
			return ErrorWithParams(ErrSystemIsLinkedToTenant, "externalID", system.ExternalID, "type", system.Type)
		}

		if err := validateRegionalSystemsForLink(ctx, r, &system); err != nil {
			return err
		}
	}

	return nil
}

func validateRegionalSystemsForLink(ctx context.Context, r repository.Repository, system *model.System) error {
	regionalSystems, err := getRegionalSystemsForSystem(ctx, r, system.ID.String())
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

func getRegionalSystemsForSystem(ctx context.Context, r repository.Repository, systemID string) ([]model.RegionalSystem, error) {
	query := repository.NewQuery(&model.RegionalSystem{})
	query.Where(repository.NewCompositeKey().Where(repository.SystemIDField, systemID))

	var regionalSystems []model.RegionalSystem

	if err := r.List(ctx, &regionalSystems, *query); err != nil {
		return nil, ErrSystemSelect
	}

	return regionalSystems, nil
}

// isUpdateKeyClaimAllowed checks whether all conditions are met to update the KeyClaim.
func (s *System) isUpdateKeyClaimAllowed(system *model.RegionalSystem, desiredClaim bool, tenantID string) error {
	err := checkRegionalSystemAvailable(system)
	if err != nil {
		return err
	}

	if desiredClaim == system.HasActiveL1KeyClaim() {
		if desiredClaim {
			return ErrKeyClaimAlreadyActive
		}

		return ErrKeyClaimAlreadyInactive
	}

	if !system.System.IsLinkedToTenant() || *system.System.TenantID != tenantID {
		return ErrSystemIsNotLinkedToTenant
	}

	return nil
}

// getRegionalSystem queries the System by its ID.
func getRegionalSystem(ctx context.Context, r repository.Repository, regionalSystem *model.RegionalSystem, withSystem bool) error {
	found, err := r.Find(ctx, regionalSystem)
	if err != nil {
		return ErrSystemSelect
	}

	if !found {
		return ErrSystemNotFound
	}

	if !withSystem {
		return nil
	}

	system := &model.System{
		ID: regionalSystem.SystemID,
	}

	found, err = r.Find(ctx, system)
	if err != nil {
		return ErrSystemSelect
	}

	if !found {
		return ErrSystemNotFound
	}

	regionalSystem.System = system

	return nil
}

// checkRegionalSystemAvailable returns nil if System has status Available.
func checkRegionalSystemAvailable(system *model.RegionalSystem) error {
	if !system.IsAvailable() {
		return ErrSystemUnavailable
	}
	return nil
}

// validateAndGetSystems validates the input slice of SystemId and returns a slice of model.System having only unique systems.
func (s *System) validateAndGetSystems(in []*systemgrpc.SystemIdentifier) ([]*model.System, map[string]*model.System, error) {
	if len(in) == 0 {
		return nil, nil, ErrNoSystemIdentifiers
	}

	uniqMap := make(map[string]*model.System)
	systems := make([]*model.System, 0, len(in))

	for _, system := range in {
		err := s.validateIdentifier(system)
		if err != nil {
			return nil, nil, err
		}

		if _, ok := uniqMap[fmt.Sprintf("%s-%s", system.GetExternalId(), system.GetType())]; !ok {
			sys := model.NewSystem(system.GetExternalId(), system.GetType())
			systems = append(systems, sys)
			uniqMap[fmt.Sprintf("%s-%s", sys.ExternalID, sys.Type)] = sys
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
			Where(repository.TypeField, s.Type),
		)
	}

	return query.SetLimit(len(systems))
}

// checkForMissingSystems checks if all systems in the uniqMap are present in the systems slice returned from the DB.
func checkForMissingSystems(uniqMap map[string]*model.System, systems []model.System) error {
	missingSystems := getMissingSystems(uniqMap, systems)
	for _, s := range missingSystems {
		return ErrorWithParams(ErrSystemNotFound, "externalID", s.ExternalID, "type", s.Type)
	}

	return nil
}

func getMissingSystems(uniqMap map[string]*model.System, systems []model.System) []*model.System {
	if len(systems) == len(uniqMap) {
		return nil
	}

	missingSystems := make([]*model.System, 0, len(uniqMap))

	systemsMap := make(map[string]model.System)
	for _, system := range systems {
		systemsMap[fmt.Sprintf("%s-%s", system.ExternalID, system.Type)] = system
	}

	for key, sys := range uniqMap {
		if _, ok := systemsMap[key]; !ok {
			missingSystems = append(missingSystems, sys)
		}
	}

	return missingSystems
}

func getSystemByIdentifier(ctx context.Context, repo repository.Repository, externalID, systemType string) (*model.System, bool, error) {
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

func (s *System) createSystem(ctx context.Context, repo repository.Repository, externalID, systemType, tenantID string) (*model.System, error) {
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

	if err := s.validateSystem(system); err != nil {
		return nil, err
	}

	if err := repo.Create(ctx, system); err != nil {
		return nil, err
	}

	return system, nil
}
