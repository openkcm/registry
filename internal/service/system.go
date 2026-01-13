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

	if err := validateRegionalSystem(s.validation, regionalSystem); err != nil {
		return nil, err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	if err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		system, found, err := getSystem(ctx, r, in.GetExternalId(), in.GetType())
		if err != nil {
			return ErrSystemSelect
		}

		if found && system.TenantID != nil && in.GetTenantId() != *system.TenantID {
			return ErrRegisterSystemNotAllowedWithTenantID
		}

		if !found {
			system, err = createSystem(ctx, s.validation, r, in.GetExternalId(), in.GetType(), in.GetTenantId())
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
//
//nolint:cyclop
func (s *System) DeleteSystem(ctx context.Context, in *systemgrpc.DeleteSystemRequest) (*systemgrpc.DeleteSystemResponse, error) {
	slogctx.Debug(ctx, "DeleteSystem called", "external_id", in.GetExternalId(), "type", in.GetType(), "region", in.GetRegion())

	if err := s.validateExternalIDTypeAndRegion(in.GetExternalId(), in.GetType(), in.GetRegion()); err != nil {
		return nil, err
	}

	var systemFound bool
	var region string

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()
	err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		regionalSystem, err := getRegionalSystem(ctx, r, in.GetExternalId(), in.GetType(), in.GetRegion())
		if err != nil && errors.Is(err, ErrSystemNotFound) {
			return nil
		}
		if err != nil {
			return err
		}

		if err := validateDeleteSystem(regionalSystem); err != nil {
			slog.Error(DeleteSystemErrMsg, "err", err.Error())
			return err
		}

		if systemFound, err = r.Delete(ctx, regionalSystem); err != nil {
			return ErrSystemDelete
		}

		region = regionalSystem.Region

		query := repository.NewQuery(&model.RegionalSystem{})
		cond := repository.NewCompositeKey()
		cond.Where(repository.SystemIDField, regionalSystem.SystemID.String())
		query.Where(cond)

		var regionalSystems []model.RegionalSystem
		if err = r.List(ctx, &regionalSystems, *query); err != nil {
			return err
		}

		if len(regionalSystems) > 0 {
			return nil
		}

		system := &model.System{
			ID: regionalSystem.SystemID,
		}
		_, err = r.Delete(ctx, system)

		return err
	})

	err = mapError(err)
	if err != nil {
		return nil, err
	}

	if systemFound {
		s.meters.handleSystemDeletion(ctx, region)
	}

	return &systemgrpc.DeleteSystemResponse{Success: true}, nil
}

// UpdateSystemL1KeyClaim updates the l1_key_claim parameter of the System identified by its system_id.
func (s *System) UpdateSystemL1KeyClaim(ctx context.Context, in *systemgrpc.UpdateSystemL1KeyClaimRequest) (*systemgrpc.UpdateSystemL1KeyClaimResponse, error) {
	slogctx.Debug(ctx, "UpdateSystemL1KeyClaim called", "external_id", in.GetExternalId(), "region", in.GetRegion(), "key_claim", in.GetL1KeyClaim(), "tenant_id", in.GetTenantId())

	if err := s.validateExternalIDTypeAndRegion(in.GetExternalId(), in.GetType(), in.GetRegion()); err != nil {
		return nil, err
	}

	desiredClaim := in.GetL1KeyClaim()

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		regionalSystem, err := getRegionalSystem(ctx, r, in.GetExternalId(), in.GetType(), in.GetRegion())
		if err != nil {
			return err
		}

		if err := s.isUpdateKeyClaimAllowed(regionalSystem, desiredClaim, in.GetTenantId()); err != nil {
			return err
		}

		isPatched, err := r.Patch(ctx, &model.RegionalSystem{
			SystemID:      regionalSystem.SystemID,
			Region:        regionalSystem.Region,
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

// UpdateSystemStatus updates the status of the System identified by its ID.
// The status can be one of a predefined set of values.
// If the update is successful, a success message will be returned, otherwise an error will be returned.
func (s *System) UpdateSystemStatus(ctx context.Context, in *systemgrpc.UpdateSystemStatusRequest) (*systemgrpc.UpdateSystemStatusResponse, error) {
	slogctx.Debug(ctx, "UpdateSystemStatus called", "external_id", in.GetExternalId(), "type", in.GetType(), "region", in.GetRegion(), "status", in.GetStatus())
	if err := s.validateExternalIDTypeAndRegion(in.GetExternalId(), in.GetType(), in.GetRegion()); err != nil {
		return nil, err
	}
	if err := s.validation.Validate(model.SystemStatusValidationID, in.GetStatus().String()); err != nil {
		return nil, ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		regionalSystem, err := getRegionalSystem(ctx, r, in.GetExternalId(), in.GetType(), in.GetRegion())
		if err != nil {
			return err
		}

		isPatched, err := r.Patch(ctx, &model.RegionalSystem{
			SystemID: regionalSystem.SystemID,
			Region:   in.GetRegion(),
			Status:   in.GetStatus().String(),
		})
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
		return nil, err
	}

	return &systemgrpc.UpdateSystemStatusResponse{Success: true}, nil
}

// SetSystemLabels sets the labels for the System identified by its external ID and region.
// Existing labels with the same keys will be overwritten.
// If the update is successful, a success message will be returned, otherwise an error will be returned.
func (s *System) SetSystemLabels(ctx context.Context, in *systemgrpc.SetSystemLabelsRequest) (*systemgrpc.SetSystemLabelsResponse, error) {
	slogctx.Debug(ctx, "SetSystemLabels called", "external_id", in.GetExternalId(), "type", in.GetType(), "region", in.GetRegion())

	if err := s.validateSetSystemLabelsRequest(in); err != nil {
		return nil, err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		regionalSystem, err := getRegionalSystem(ctx, r, in.GetExternalId(), in.GetType(), in.GetRegion())
		if err != nil {
			return err
		}

		if err := checkRegionalSystemAvailable(regionalSystem); err != nil {
			return err
		}

		systemToPatch := &model.RegionalSystem{
			SystemID: regionalSystem.SystemID,
			Region:   in.GetRegion(),
			Labels:   regionalSystem.Labels,
		}

		if systemToPatch.Labels == nil {
			systemToPatch.Labels = make(map[string]string)
		}

		maps.Copy(systemToPatch.Labels, in.GetLabels())

		isPatched, err := r.Patch(ctx, systemToPatch)
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
	slogctx.Debug(ctx, "RemoveSystemLabels called", "external_id", in.GetExternalId(), "type", in.GetType(), "region", in.GetRegion())

	if err := s.validateRemoveSystemLabelsRequest(in); err != nil {
		return nil, err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := s.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		regionalSystem, err := getRegionalSystem(ctx, r, in.GetExternalId(), in.GetType(), in.GetRegion())
		if err != nil {
			return err
		}

		if err := checkRegionalSystemAvailable(regionalSystem); err != nil {
			return err
		}

		systemToPatch := &model.RegionalSystem{
			SystemID: regionalSystem.SystemID,
			Region:   in.GetRegion(),
			Labels:   regionalSystem.Labels,
		}

		for _, k := range in.GetLabelKeys() {
			delete(systemToPatch.Labels, k)
		}

		isPatched, err := r.Patch(ctx, systemToPatch)
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
		return nil, err
	}

	return &systemgrpc.RemoveSystemLabelsResponse{
		Success: true,
	}, nil
}

// validateExternalIDTypeAndRegion validates the externalID, type and region against the validator.
func (s *System) validateExternalIDTypeAndRegion(exteralID, systemType, region string) error {
	if systemType != "" {
		if err := validateExternalIDAndType(s.validation, exteralID, systemType); err != nil {
			return err
		}
	}

	if err := s.validation.ValidateAll(map[validation.ID]any{
		model.SystemExternalIDValidationID:     exteralID,
		model.RegionalSystemRegionValidationID: region,
	}); err != nil {
		return ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}

	return nil
}

// validateSetSystemLabelsRequest validates the SetSystemLabelsRequest.
// If the request is valid, it returns nil, otherwise it returns an error.
func (s *System) validateSetSystemLabelsRequest(in *systemgrpc.SetSystemLabelsRequest) error {
	if err := s.validateExternalIDTypeAndRegion(in.GetExternalId(), in.GetType(), in.GetRegion()); err != nil {
		return err
	}

	if len(in.GetLabels()) == 0 {
		return ErrMissingLabels
	}

	labels := in.GetLabels()
	err := s.validation.Validate(model.RegionalSystemLabelsValidationID, labels)
	if err != nil {
		return err
	}

	return nil
}

// validateRemoveSystemLabelsRequest validates the RemoveSystemLabelsRequest.
// If the request is valid, it returns nil, otherwise it returns an error.
func (s *System) validateRemoveSystemLabelsRequest(in *systemgrpc.RemoveSystemLabelsRequest) error {
	if err := s.validateExternalIDTypeAndRegion(in.GetExternalId(), in.GetType(), in.GetRegion()); err != nil {
		return err
	}

	if len(in.GetLabelKeys()) == 0 {
		return ErrMissingLabelKeys
	}

	if slices.Contains(in.GetLabelKeys(), "") {
		return ErrEmptyLabelKeys
	}

	return nil
}

// validateDeleteSystem makes sure that the System is allowed to be deleted.
// Here repository r is passed as a variable to address the scenarios where we will
// create a new repository from the existing repository for e.g. in the case of transaction.
func validateDeleteSystem(regionalSystem *model.RegionalSystem) error {
	err := checkRegionalSystemAvailable(regionalSystem)
	if err != nil {
		return err
	}

	if regionalSystem.System.IsLinkedToTenant() {
		return ErrSystemIsLinkedToTenant
	}

	return nil
}

// isUpdateKeyClaimAllowed checks whether all conditions are met to update the KeyClaim.
func (s *System) isUpdateKeyClaimAllowed(regionalSystem *model.RegionalSystem, desiredClaim bool, tenantID string) error {
	err := checkRegionalSystemAvailable(regionalSystem)
	if err != nil {
		return err
	}

	if desiredClaim == regionalSystem.HasActiveL1KeyClaim() {
		if desiredClaim {
			return ErrKeyClaimAlreadyActive
		}

		return ErrKeyClaimAlreadyInactive
	}

	if !regionalSystem.System.IsLinkedToTenant() || *regionalSystem.System.TenantID != tenantID {
		return ErrSystemIsNotLinkedToTenant
	}

	return nil
}

// getRegionalSystem fetches the regional system from the db based on the externalID, type and region.
func getRegionalSystem(ctx context.Context, repo repository.Repository, external_id, systemType, region string) (*model.RegionalSystem, error) {
	var regionalSystem *model.RegionalSystem
	var err error

	if len(systemType) == 0 {
		regionalSystem, err = getRegionalSystemWithoutType(ctx, repo, external_id, region)
	} else {
		regionalSystem, err = getRegionalSystemWithType(ctx, repo, external_id, systemType, region)
	}

	return regionalSystem, err
}

// getRegionalSystemWithType fetched the system and returns the regional system.
func getRegionalSystemWithType(ctx context.Context, repo repository.Repository, externalID, systemType, region string) (*model.RegionalSystem, error) {
	system := &model.System{
		ExternalID: externalID,
		Type:       systemType,
	}

	found, err := repo.Find(ctx, system)
	if err != nil {
		return nil, ErrSystemSelect
	}
	if !found {
		return nil, ErrSystemNotFound
	}

	regionalSystem := &model.RegionalSystem{
		SystemID: system.ID,
		Region:   region,
	}

	found, err = repo.Find(ctx, regionalSystem)
	if err != nil {
		return nil, ErrSystemSelect
	}
	if !found {
		return nil, ErrSystemNotFound
	}

	regionalSystem.System = system

	return regionalSystem, nil
}

// getRegionalSystemWithoutType fetches the regionalSystem if there are multiple systems returned then it returns an error.
func getRegionalSystemWithoutType(ctx context.Context, repo repository.Repository, externalID, region string) (*model.RegionalSystem, error) {
	var systems []model.System
	query := repository.NewQuery(&model.System{})
	query.Where(repository.NewCompositeKey().Where(repository.ExternalIDField, externalID))

	if err := repo.List(ctx, &systems, *query); err != nil {
		return nil, err
	}

	if len(systems) == 0 {
		return nil, ErrSystemNotFound
	}

	if len(systems) > 1 {
		return nil, ErrTooManyTypes
	}

	system := systems[0]

	regionalSystems, err := getRegionalSystemsFromSystemID(ctx, repo, system.ID.String())
	if err != nil {
		return nil, err
	}

	if len(regionalSystems) == 0 {
		return nil, ErrSystemNotFound
	}

	for _, rs := range regionalSystems {
		if rs.Region == region {
			rs.System = &system
			return &rs, nil
		}
	}

	return nil, ErrSystemNotFound
}
