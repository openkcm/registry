package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/openkcm/orbital"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
)

// Tenant implements the procedure calls defined as protobufs.
// See https://github.com/openkcm/api-sdk/blob/main/proto/kms/api/cmk/registry/tenant/v1/tenant.proto.
type Tenant struct {
	tenantgrpc.UnimplementedServiceServer

	repo    repository.Repository
	orbital *Orbital
	meters  *Meters
}

type (
	tenantUpdateFunc   func(tenant *model.Tenant)
	tenantValidateFunc func(tenant *model.Tenant) error
	orbitalJobFunc     func(ctx context.Context, tenant *model.Tenant) error

	patchTenantParams struct {
		id           model.ID
		updateFunc   tenantUpdateFunc
		validateFunc tenantValidateFunc
		jobFunc      orbitalJobFunc
	}
)

// NewTenant creates and returns a new instance of Tenant.
func NewTenant(repo repository.Repository, orbital *Orbital, meters *Meters) *Tenant {
	t := &Tenant{
		repo:    repo,
		orbital: orbital,
		meters:  meters,
	}

	// Register tenant service as job handler for tenant-related actions
	for _, jobType := range []string{
		tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(),
		tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String(),
		tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String(),
		tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String(),
	} {
		orbital.RegisterJobHandler(jobType, t)
	}

	return t
}

// RegisterTenant handles the creation of a new Tenant. The response contains the created Tenant's ID.
func (t *Tenant) RegisterTenant(ctx context.Context, in *tenantgrpc.RegisterTenantRequest) (*tenantgrpc.RegisterTenantResponse, error) {
	slogctx.Debug(ctx, "RegisterTenant called", "tenantId", in.GetId(), "tenantName", in.GetName(), "tenantRegion", in.GetRegion())
	tenant := &model.Tenant{
		Name:            model.Name(in.GetName()),
		ID:              model.ID(in.GetId()),
		Region:          model.Region(in.GetRegion()),
		OwnerID:         model.OwnerID(in.GetOwnerId()),
		OwnerType:       model.OwnerType(in.GetOwnerType()),
		Status:          model.TenantStatus(tenantgrpc.Status_STATUS_PROVISIONING.String()),
		StatusUpdatedAt: time.Now(),
		Role:            model.Role(in.GetRole().String()),
		Labels:          in.GetLabels(),
	}

	if err := tenant.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	err := t.repo.Transaction(ctx, func(ctx context.Context, r repository.Repository) error {
		if err := r.Create(ctx, tenant); err != nil {
			var ucErr *repository.UniqueConstraintError
			if errors.As(err, &ucErr) {
				return status.Error(codes.InvalidArgument, ucErr.Error())
			}

			return err
		}

		data, err := proto.Marshal(tenant.ToProto())
		if err != nil {
			slogctx.Error(ctx, "failed to encode tenant data", "error", err)
			return ErrTenantEncoding
		}

		err = t.orbital.PrepareJob(ctx, data, tenant.ID.String(), tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String())
		if err != nil {
			return status.Error(codes.Internal, "failed to start tenant provisioning job")
		}

		return nil
	})

	err = mapError(err)
	if err != nil {
		return nil, err
	}

	t.meters.handleTenantRegistration(ctx, string(tenant.Region))

	return &tenantgrpc.RegisterTenantResponse{
		Id: tenant.ID.String(),
	}, nil
}

// ListTenants retrieves a list of Tenants based on optional query parameters such as id, name, region,
// owner_id, and owner_type.
// Retrieves all Tenants if all query parameters are empty.
func (t *Tenant) ListTenants(ctx context.Context, in *tenantgrpc.ListTenantsRequest) (*tenantgrpc.ListTenantsResponse, error) {
	slogctx.Debug(ctx, "ListTenants called", "id", in.GetId(), "name", in.GetName(), "region", in.GetRegion(), "ownerId", in.GetOwnerId(), "ownerType", in.GetOwnerType())

	query, err := t.buildListTenantsQuery(in)
	if err != nil {
		return nil, err
	}

	var tenants []model.Tenant
	if err := t.repo.List(ctx, &tenants, *query); err != nil {
		return nil, err
	}

	pbTenants := t.mapTenantsToGRPCResponse(tenants)
	if len(pbTenants) == 0 {
		return nil, ErrTenantNotFound
	}

	if len(tenants) < query.Limit {
		return &tenantgrpc.ListTenantsResponse{
			Tenants: pbTenants,
		}, nil
	}

	lastItem := tenants[len(tenants)-1]

	nextPageToken, err := repository.PageInfo{
		LastKey:       lastItem.PaginationKey(),
		LastCreatedAt: lastItem.CreatedAt,
	}.Encode()
	if err != nil {
		return nil, err
	}

	return &tenantgrpc.ListTenantsResponse{
		Tenants:       pbTenants,
		NextPageToken: nextPageToken,
	}, nil
}

// BlockTenant updates the status of a Tenant to BLOCKED.
// If the update is successful, a success message will be returned, otherwise an error will be returned.
//
//nolint:dupl
func (t *Tenant) BlockTenant(ctx context.Context, in *tenantgrpc.BlockTenantRequest) (*tenantgrpc.BlockTenantResponse, error) {
	slogctx.Debug(ctx, "BlockTenant called", "tenantId", in.GetId())

	id := model.ID(in.GetId())
	if err := t.validateTenantId(id); err != nil {
		return nil, err
	}

	err := t.patchTenant(ctx, patchTenantParams{
		id: id,
		updateFunc: func(tenant *model.Tenant) {
			tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKING.String()))
		},
		validateFunc: validateTransition(tenantgrpc.Status_STATUS_BLOCKING),
		jobFunc: func(ctx context.Context, tenant *model.Tenant) error {
			data, err := proto.Marshal(tenant.ToProto())
			if err != nil {
				slogctx.Error(ctx, "failed to encode tenant data", "error", err)
				return ErrTenantEncoding
			}
			return t.orbital.PrepareJob(ctx, data, tenant.ID.String(), tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String())
		},
	})
	if err != nil {
		return nil, err
	}

	return &tenantgrpc.BlockTenantResponse{Success: true}, nil
}

// UnblockTenant updates the status of a Tenant to ACTIVE.
// If the update is successful, a success message will be returned, otherwise an error will be returned.
//
//nolint:dupl
func (t *Tenant) UnblockTenant(ctx context.Context, in *tenantgrpc.UnblockTenantRequest) (*tenantgrpc.UnblockTenantResponse, error) {
	slogctx.Debug(ctx, "UnblockTenant called", "tenantId", in.GetId())

	id := model.ID(in.GetId())
	if err := t.validateTenantId(id); err != nil {
		return nil, err
	}

	err := t.patchTenant(ctx, patchTenantParams{
		id: id,
		updateFunc: func(tenant *model.Tenant) {
			tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_UNBLOCKING.String()))
		},
		validateFunc: validateTransition(tenantgrpc.Status_STATUS_UNBLOCKING),
		jobFunc: func(ctx context.Context, tenant *model.Tenant) error {
			data, err := proto.Marshal(tenant.ToProto())
			if err != nil {
				slogctx.Error(ctx, "failed to encode tenant data", "error", err)
				return ErrTenantEncoding
			}
			return t.orbital.PrepareJob(ctx, data, tenant.ID.String(), tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String())
		},
	})
	if err != nil {
		return nil, err
	}

	return &tenantgrpc.UnblockTenantResponse{Success: true}, nil
}

// TerminateTenant updates the status of a Tenant to TERMINATED.
// If the update is successful, a success message will be returned, otherwise an error will be returned.
func (t *Tenant) TerminateTenant(ctx context.Context, in *tenantgrpc.TerminateTenantRequest) (*tenantgrpc.TerminateTenantResponse, error) {
	slogctx.Debug(ctx, "TerminateTenant called", "tenantId", in.GetId())

	id := model.ID(in.GetId())
	if err := t.validateTenantId(id); err != nil {
		return nil, err
	}

	if err := assertNoSystemLinks(ctx, t.repo, id.String()); err != nil {
		return nil, err
	}

	err := t.patchTenant(ctx, patchTenantParams{
		id: id,
		updateFunc: func(tenant *model.Tenant) {
			tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATING.String()))
		},
		validateFunc: validateTransition(tenantgrpc.Status_STATUS_TERMINATING),
		jobFunc: func(ctx context.Context, tenant *model.Tenant) error {
			data, err := proto.Marshal(tenant.ToProto())
			if err != nil {
				slogctx.Error(ctx, "failed to encode tenant data", "error", err)
				return ErrTenantEncoding
			}
			return t.orbital.PrepareJob(ctx, data, tenant.ID.String(), tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String())
		},
	})
	if err != nil {
		return nil, err
	}

	return &tenantgrpc.TerminateTenantResponse{Success: true}, nil
}

// SetTenantLabels sets the labels for the Tenant identified by its ID.
// Existing labels with the same keys will be overwritten.
// If the update is successful, a success message will be returned, otherwise an error will be returned.
func (t *Tenant) SetTenantLabels(ctx context.Context, in *tenantgrpc.SetTenantLabelsRequest) (*tenantgrpc.SetTenantLabelsResponse, error) {
	slogctx.Debug(ctx, "SetTenantLabels called", "tenantId", in.GetId())

	if err := t.validateSetTenantLabelsRequest(in); err != nil {
		return nil, err
	}

	err := t.patchTenant(ctx, patchTenantParams{
		id: model.ID(in.GetId()),
		updateFunc: func(tenant *model.Tenant) {
			if tenant.Labels == nil {
				tenant.Labels = make(model.Labels)
			}
			maps.Copy(tenant.Labels, in.GetLabels())
		},
		validateFunc: checkTenantActive,
	})
	if err != nil {
		return nil, err
	}

	return &tenantgrpc.SetTenantLabelsResponse{
		Success: true,
	}, nil
}

// RemoveTenantLabels removes the specified labels from the Tenant identified by its external ID and region.
// If one or more label keys are not found, they will be ignored.
// If the update is successful, a success message will be returned, otherwise an error will be returned.
func (t *Tenant) RemoveTenantLabels(ctx context.Context, in *tenantgrpc.RemoveTenantLabelsRequest) (*tenantgrpc.RemoveTenantLabelsResponse, error) {
	slogctx.Debug(ctx, "RemoveTenantLabels called", "tenantId", in.GetId())

	if err := t.validateRemoveTenantLabelsRequest(in); err != nil {
		return nil, err
	}

	err := t.patchTenant(ctx, patchTenantParams{
		id: model.ID(in.GetId()),
		updateFunc: func(tenant *model.Tenant) {
			if tenant.Labels == nil {
				return
			}
			for _, k := range in.GetLabelKeys() {
				delete(tenant.Labels, k)
			}
		},
		validateFunc: checkTenantActive,
	})
	if err != nil {
		return nil, err
	}

	return &tenantgrpc.RemoveTenantLabelsResponse{
		Success: true,
	}, nil
}

// GetTenant retrieves the details of a Tenant by its ID.
// If the Tenant is found, its details will be returned, otherwise an error will be returned.
func (t *Tenant) GetTenant(ctx context.Context, in *tenantgrpc.GetTenantRequest) (*tenantgrpc.GetTenantResponse, error) {
	slogctx.Debug(ctx, "GetTenant called", "tenantId", in.GetId())

	id := model.ID(in.GetId())
	if err := t.validateTenantId(id); err != nil {
		return nil, err
	}

	tenant, err := getTenant(ctx, t.repo, id)
	if err != nil {
		return nil, err
	}

	return &tenantgrpc.GetTenantResponse{
		Tenant: tenant.ToProto(),
	}, nil
}

// ConfirmJob checks if a job can be confirmed based on tenant existence and tenant status.
func (t *Tenant) ConfirmJob(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error) {
	tenant, err := getTenant(ctx, t.repo, model.ID(job.ExternalID))
	if err != nil {
		slogctx.Error(ctx, "failed to load tenant for job", "error", err, "jobID", job.ID.String())
		return orbital.JobConfirmResult{}, err
	}

	switch job.Type {
	case tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String():
		return orbital.JobConfirmResult{Done: true}, nil
	case tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String(), tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String(), tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String():
		status, err := jobTypeToStatus(job.Type)
		if err != nil { //nolint:nilerr // if we return an error here, the job will be retried indefinitely
			return orbital.JobConfirmResult{
				IsCanceled:           true,
				CanceledErrorMessage: fmt.Sprintf("%s: %s", ErrUnexpectedJobType, job.Type),
			}, nil
		}

		if tenant.Status != model.TenantStatus(status.String()) {
			return orbital.JobConfirmResult{}, ErrInvalidTenantStatus
		}

		return orbital.JobConfirmResult{Done: true}, nil
	default:
		return orbital.JobConfirmResult{
			IsCanceled:           true,
			CanceledErrorMessage: fmt.Sprintf("%s: %s", ErrUnexpectedJobType, job.Type),
		}, nil
	}
}

// ResolveTasks creates a task for the job based on the tenant's region.
func (t *Tenant) ResolveTasks(ctx context.Context, job orbital.Job, targetsByRegion map[string]orbital.Initiator) (orbital.TaskResolverResult, error) {
	tenant := &tenantgrpc.Tenant{}

	err := proto.Unmarshal(job.Data, tenant)
	if err != nil {
		return orbital.TaskResolverResult{
			IsCanceled:           true,
			CanceledErrorMessage: fmt.Sprintf("failed to unmarshal tenant data: %v", err),
		}, nil
	}

	_, ok := targetsByRegion[tenant.GetRegion()]
	if !ok {
		return orbital.TaskResolverResult{
			IsCanceled:           true,
			CanceledErrorMessage: "no orbital initiator found for region: " + tenant.GetRegion(),
		}, nil
	}

	return orbital.TaskResolverResult{
		TaskInfos: []orbital.TaskInfo{
			{
				Data:   job.Data,
				Type:   job.Type,
				Target: tenant.GetRegion(),
			},
		},
		Done: true,
	}, nil
}

// HandleJobFailed applies the changes to the tenant based on the job type when the job is failed.
func (t *Tenant) HandleJobFailed(ctx context.Context, job orbital.Job) error {
	return t.handleJobAborted(ctx, job)
}

// HandleJobCanceled applies the changes to the tenant based on the job type when the job is canceled.
func (t *Tenant) HandleJobCanceled(ctx context.Context, job orbital.Job) error {
	return t.handleJobAborted(ctx, job)
}

// HandleJobDone applies the changes to the tenant based on the job type when the job is done.
func (t *Tenant) HandleJobDone(ctx context.Context, job orbital.Job) error {
	return t.patchTenant(ctx, patchTenantParams{
		id: model.ID(job.ExternalID),
		updateFunc: func(tenant *model.Tenant) {
			switch job.Type {
			case tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(), tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String():
				tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()))
			case tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String():
				tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String()))
			case tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String():
				tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATED.String()))
			}
		},
	})
}

func (t *Tenant) SetTenantUserGroups(ctx context.Context, in *tenantgrpc.SetTenantUserGroupsRequest) (*tenantgrpc.SetTenantUserGroupsResponse, error) {
	slogctx.Debug(ctx, "SetTenantUserGroups called", "tenantId", in.GetId())

	id := model.ID(in.GetId())
	if err := t.validateTenantId(id); err != nil {
		return nil, err
	}

	userGroups := model.UserGroups(in.GetUserGroups())
	if err := t.validateUserGroups(userGroups); err != nil {
		return nil, err
	}

	err := t.patchTenant(ctx, patchTenantParams{
		id: id,
		updateFunc: func(tenant *model.Tenant) {
			tenant.UserGroups = in.GetUserGroups()
		},
	})
	if err != nil {
		return nil, err
	}

	return &tenantgrpc.SetTenantUserGroupsResponse{Success: true}, nil
}

func (t *Tenant) handleJobAborted(ctx context.Context, job orbital.Job) error {
	return t.patchTenant(ctx, patchTenantParams{
		id: model.ID(job.ExternalID),
		updateFunc: func(tenant *model.Tenant) {
			switch job.Type {
			case tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String():
				tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_PROVISIONING_ERROR.String()))
			case tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String():
				tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_UNBLOCKING_ERROR.String()))
			case tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String():
				tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKING_ERROR.String()))
			case tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String():
				tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATION_ERROR.String()))
			}
		},
	})
}

func (t *Tenant) validateTenantId(tenantID model.ID) error {
	tenantPtr := &model.Tenant{
		ID: tenantID,
	}
	if err := model.ValidateField(tenantPtr, &tenantPtr.ID); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return nil
}

func (t *Tenant) validateLabels(labels model.Labels) error {
	tenantPtr := &model.Tenant{
		Labels: labels,
	}
	if err := model.ValidateField(tenantPtr, &tenantPtr.Labels); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	return nil
}

func (t *Tenant) validateUserGroups(userGroups model.UserGroups) error {
	tenantPtr := &model.Tenant{
		UserGroups: userGroups,
	}
	if err := model.ValidateField(tenantPtr, &tenantPtr.UserGroups); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	return nil
}

// validateSetTenantLabelsRequest validates the SetTenantLabelsRequest.
// If the request is valid, it returns nil, otherwise it returns an error.
func (t *Tenant) validateSetTenantLabelsRequest(in *tenantgrpc.SetTenantLabelsRequest) error {
	id := model.ID(in.GetId())
	if err := t.validateTenantId(id); err != nil {
		return err
	}

	if len(in.GetLabels()) == 0 {
		return ErrMissingLabels
	}

	labels := model.Labels(in.GetLabels())
	if err := t.validateLabels(labels); err != nil {
		return err
	}

	return nil
}

// validateRemoveTenantLabelsRequest validates the RemoveTenantLabelsRequest.
// If the request is valid, it returns nil, otherwise it returns an error.
func (t *Tenant) validateRemoveTenantLabelsRequest(in *tenantgrpc.RemoveTenantLabelsRequest) error {
	id := model.ID(in.GetId())
	if err := t.validateTenantId(id); err != nil {
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

// patchTenant retrieves the Tenant by its ID, applies the update function to it,
// and then updates the Tenant in the repository.
// It returns an error if the Tenant is not found, if the validation fails, or if the repository update fails.
func (t *Tenant) patchTenant(ctx context.Context, params patchTenantParams) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := t.repo.Transaction(ctxTimeout, func(ctx context.Context, r repository.Repository) error {
		tenant, err := getTenant(ctx, r, params.id)
		if err != nil {
			return err
		}

		if params.validateFunc != nil {
			err = params.validateFunc(tenant)
			if err != nil {
				return err
			}
		}

		if params.updateFunc != nil {
			params.updateFunc(tenant)

			isPatched, err := r.Patch(ctx, tenant)
			if err != nil {
				return ErrTenantUpdate
			}

			if !isPatched {
				return ErrTenantNotFound
			}
		}

		if params.jobFunc != nil {
			err = params.jobFunc(ctx, tenant)
			if err != nil {
				return status.Errorf(codes.Internal, "failed to start orbital job: %v", err)
			}
		}

		return nil
	})

	return mapError(err)
}

// getTenant queries the Tenant by its ID.
func getTenant(ctx context.Context, r repository.Repository, id model.ID) (*model.Tenant, error) {
	tenant := &model.Tenant{
		ID: id,
	}

	found, err := r.Find(ctx, tenant)
	if err != nil {
		return nil, ErrTenantSelect
	}

	if !found {
		return nil, ErrTenantNotFound
	}

	return tenant, nil
}

func (t *Tenant) buildListTenantsQuery(in *tenantgrpc.ListTenantsRequest) (*repository.Query, error) {
	query := repository.NewQuery(&model.Tenant{})

	err := query.ApplyPagination(in.GetLimit(), in.GetPageToken())
	if err != nil {
		return nil, err
	}

	cond := repository.NewCompositeKey()
	if in.GetId() != "" {
		cond.Where(repository.IDField, in.GetId())
	}

	if in.GetName() != "" {
		cond.Where(repository.NameField, in.GetName())
	}

	if in.GetRegion() != "" {
		cond.Where(repository.RegionField, in.GetRegion())
	}

	if in.GetOwnerId() != "" {
		cond.Where(repository.OwnerIDField, in.GetOwnerId())
	}

	if in.GetOwnerType() != "" {
		ownerType := model.OwnerType(in.GetOwnerType())
		tenantPtr := &model.Tenant{
			OwnerType: ownerType,
		}
		if err := model.ValidateField(tenantPtr, &tenantPtr.OwnerType); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		cond.Where(repository.OwnerTypeField, in.GetOwnerType())
	}

	return query.Where(cond), nil
}

// mapTenantsToGRPCResponse maps model Tenants to GRPC Tenants to be compatible for response.
func (t *Tenant) mapTenantsToGRPCResponse(tenants []model.Tenant) []*tenantgrpc.Tenant {
	pbTenants := make([]*tenantgrpc.Tenant, 0, len(tenants))
	for _, tenant := range tenants {
		pbTenants = append(pbTenants, tenant.ToProto())
	}

	return pbTenants
}

// assertNoSystemLinks checks if there are any Systems linked with the Tenant.
// If records are found for the provided tenantID, it returns an error.
// Here repository r is passed as a variable to address the scenarios where we will
// create a new repository from the existing repository for e.g. in the case of transaction.
func assertNoSystemLinks(ctx context.Context, r repository.Repository, tenantID string) error {
	query := repository.NewQuery(&model.Tenant{}).Where(
		repository.NewCompositeKey().Where(repository.TenantIDField, tenantID),
	)

	var systems []model.System

	err := r.List(ctx, &systems, *query)
	if err != nil {
		return ErrSystemSelect
	}

	if len(systems) > 0 {
		return ErrSystemIsLinkedToTenant
	}

	return nil
}

// validateTransition checks if a tenant can transition to the given status.
func validateTransition(targetStatus tenantgrpc.Status) tenantValidateFunc {
	return func(tenant *model.Tenant) error {
		err := tenant.Status.ValidateTransition(targetStatus)
		if err != nil {
			return status.Error(codes.FailedPrecondition, err.Error())
		}

		return nil
	}
}

// checkTenantActive returns nil if Tenant has status Available.
func checkTenantActive(tenant *model.Tenant) error {
	if tenant.Status.IsActive() {
		return nil
	}

	return ErrTenantUnavailable
}

// jobTypeToStatus maps the job type to the corresponding tenant status.
func jobTypeToStatus(jobType string) (tenantgrpc.Status, error) {
	switch jobType {
	case tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String():
		return tenantgrpc.Status_STATUS_PROVISIONING, nil
	case tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String():
		return tenantgrpc.Status_STATUS_BLOCKING, nil
	case tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String():
		return tenantgrpc.Status_STATUS_UNBLOCKING, nil
	case tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String():
		return tenantgrpc.Status_STATUS_TERMINATING, nil
	default:
		return tenantgrpc.Status_STATUS_UNSPECIFIED, fmt.Errorf("%w: %s", ErrUnexpectedJobType, jobType)
	}
}
