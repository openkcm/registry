package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/openkcm/orbital"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	tenantconfiggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant_config/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

const systemLimitField = "system_limit"

// TenantConfig implements the tenant_config/v1 gRPC service.
// It owns the lifecycle of per-tenant configuration overrides and tracks whether
// each update has been propagated to the CMK layer (UPDATING → ACTIVE / UPDATING_ERROR).
type TenantConfig struct {
	tenantconfiggrpc.UnimplementedServiceServer

	repo       repository.Repository
	orbital    JobPreparer
	validation *validation.Validation
}

// NewTenantConfig creates a new TenantConfig service and registers its orbital job handler.
func NewTenantConfig(repo repository.Repository, orbital *Orbital, v *validation.Validation) *TenantConfig {
	tc := &TenantConfig{
		repo:       repo,
		orbital:    orbital,
		validation: v,
	}
	orbital.RegisterJobHandler(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), tc)
	return tc
}

// GetTenantConfig returns the configuration overrides for the given tenant.
// Returns NOT_FOUND if no config record has been created yet (i.e. UpdateTenantConfig was never called).
func (tc *TenantConfig) GetTenantConfig(ctx context.Context, in *tenantconfiggrpc.GetTenantConfigRequest) (*tenantconfiggrpc.GetTenantConfigResponse, error) {
	slogctx.Debug(ctx, "GetTenantConfig called", "tenantId", in.GetTenantId())

	if in.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id must not be empty")
	}

	cfg, err := getTenantConfig(ctx, tc.repo, in.GetTenantId())
	if err != nil {
		return nil, err
	}

	return cfg.ToProto(), nil
}

// UpdateTenantConfig applies the masked fields onto the tenant's configuration record,
// sets the status to UPDATING, and dispatches an orbital job to propagate the change to CMK.
// On job completion the status transitions to ACTIVE; on failure to UPDATING_ERROR.
func (tc *TenantConfig) UpdateTenantConfig(ctx context.Context, in *tenantconfiggrpc.UpdateTenantConfigRequest) (*tenantconfiggrpc.UpdateTenantConfigResponse, error) {
	slogctx.Debug(ctx, "UpdateTenantConfig called", "tenantId", in.GetTenantId())

	if err := tc.validation.Validate(model.TenantConfigTenantIDValidationID, in.GetTenantId()); err != nil {
		return nil, ErrorWithParams(ErrValidationFailed, "err", err.Error())
	}
	if len(in.GetUpdateMask().GetPaths()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "update_mask must not be empty")
	}
	if err := validateTenantConfigMaskPaths(in.GetValues(), in.GetUpdateMask().GetPaths()); err != nil {
		return nil, err
	}

	err := transact(ctx, tc.repo, func(ctx context.Context, r repository.Repository) error {
		tenant, err := getTenant(ctx, r, in.GetTenantId())
		if err != nil {
			return err
		}

		if err := checkTenantActive(tenant); err != nil {
			return err
		}

		cfg, err := findOrInitTenantConfig(ctx, r, in.GetTenantId())
		if err != nil {
			return err
		}

		if cfg.Status == tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING.String() {
			return ErrTenantConfigUpdateInProgress
		}

		applyTenantConfigMask(cfg, in.GetValues(), in.GetUpdateMask())
		cfg.Status = tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING.String()
		cfg.ErrorMessage = ""

		if err := createOrPatchTenantConfig(ctx, r, cfg); err != nil {
			return err
		}

		data, err := proto.Marshal(tenant.ToProto())
		if err != nil {
			slogctx.Error(ctx, "failed to encode tenant data for config job", "error", err)
			return fmt.Errorf("%w: %w", ErrTenantConfigEncoding, err)
		}

		if err := tc.orbital.PrepareJob(ctx, data, in.GetTenantId(), tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String()); err != nil {
			return status.Errorf(codes.Internal, "failed to start config update job: %v", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return tenantconfiggrpc.UpdateTenantConfigResponse_builder{}.Build(), nil
}

// ConfirmJob verifies the config record exists and is still in UPDATING state before the job runs.
func (tc *TenantConfig) ConfirmJob(ctx context.Context, job orbital.Job) (orbital.JobConfirmerResult, error) {
	cfg, err := getTenantConfig(ctx, tc.repo, job.ExternalID)
	if err != nil {
		if errors.Is(err, ErrTenantConfigNotFound) {
			return orbital.CancelJobConfirmer("tenant config not found"), nil
		}
		slogctx.Error(ctx, "failed to load tenant config for job confirmation", "error", err, "jobId", job.ID.String())
		return nil, err
	}

	if cfg.Status != tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING.String() {
		return orbital.CancelJobConfirmer("tenant config not in UPDATING status: " + cfg.Status), nil
	}

	return orbital.CompleteJobConfirmer(), nil
}

// ResolveTasks routes the job to the tenant's region using the Tenant proto stored in the job payload.
func (tc *TenantConfig) ResolveTasks(ctx context.Context, job orbital.Job, targetsByRegion map[string]orbital.TargetManager) (orbital.TaskResolverResult, error) {
	tenant := &tenantgrpc.Tenant{}
	if err := proto.Unmarshal(job.Data, tenant); err != nil {
		msg := "failed to unmarshal tenant data for config job"
		slogctx.Error(ctx, msg, "error", err)
		return orbital.CancelTaskResolver(fmt.Sprintf("%s: %v", msg, err)), nil
	}

	if _, ok := targetsByRegion[tenant.GetRegion()]; !ok {
		msg := "no matching orbital target for region"
		slogctx.Error(ctx, msg, "region", tenant.GetRegion())
		return orbital.CancelTaskResolver(msg + ": " + tenant.GetRegion()), nil
	}

	return orbital.CompleteTaskResolver().WithTaskInfo([]orbital.TaskInfo{
		{
			Data:   job.Data,
			Type:   job.Type,
			Target: tenant.GetRegion(),
		},
	}), nil
}

// HandleJobDone transitions the config status to ACTIVE when the orbital job completes.
func (tc *TenantConfig) HandleJobDone(ctx context.Context, job orbital.Job) error {
	err := patchTenantConfig(ctx, tc.repo, job.ExternalID, func(cfg *model.TenantConfig) {
		cfg.Status = tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_ACTIVE.String()
		cfg.ErrorMessage = ""
	})
	if errors.Is(err, ErrTenantConfigNotFound) {
		slogctx.Warn(ctx, "tenant config not found on job done", "tenantId", job.ExternalID)
		return nil
	}
	return err
}

// HandleJobFailed transitions the config status to UPDATING_ERROR when the orbital job fails.
func (tc *TenantConfig) HandleJobFailed(ctx context.Context, job orbital.Job) error {
	return tc.handleJobAborted(ctx, job)
}

// HandleJobCanceled transitions the config status to UPDATING_ERROR when the orbital job is canceled.
func (tc *TenantConfig) HandleJobCanceled(ctx context.Context, job orbital.Job) error {
	return tc.handleJobAborted(ctx, job)
}

func (tc *TenantConfig) handleJobAborted(ctx context.Context, job orbital.Job) error {
	err := patchTenantConfig(ctx, tc.repo, job.ExternalID, func(cfg *model.TenantConfig) {
		cfg.Status = tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING_ERROR.String()
		cfg.ErrorMessage = job.ErrorMessage
	})
	if errors.Is(err, ErrTenantConfigNotFound) {
		slogctx.Warn(ctx, "tenant config not found on job aborted", "tenantId", job.ExternalID)
		return nil
	}
	return err
}

// validateTenantConfigMaskPaths checks that all paths in the update mask are known,
// and that any supplied values are valid.
func validateTenantConfigMaskPaths(values *tenantconfiggrpc.TenantConfigurationValues, paths []string) error {
	for _, path := range paths {
		if path != systemLimitField {
			return status.Errorf(codes.InvalidArgument, "unknown field mask path: %s", path)
		}
		if values != nil && values.GetSystemLimit() <= 0 {
			return status.Error(codes.InvalidArgument, "system_limit must be greater than 0")
		}
	}
	return nil
}

// applyTenantConfigMask writes the fields listed in mask from values onto cfg.
func applyTenantConfigMask(cfg *model.TenantConfig, values *tenantconfiggrpc.TenantConfigurationValues, mask *fieldmaskpb.FieldMask) {
	for _, path := range mask.GetPaths() {
		if path == systemLimitField && values != nil && values.GetSystemLimit() > 0 {
			sl := values.GetSystemLimit()
			cfg.SystemLimit = &sl
		}
	}
}

// findOrInitTenantConfig returns the existing config record for tenantID, or a zero-value struct if none exists yet.
func findOrInitTenantConfig(ctx context.Context, r repository.Repository, tenantID string) (*model.TenantConfig, error) {
	cfg := &model.TenantConfig{TenantID: tenantID}
	found, err := r.Find(ctx, cfg)
	if err != nil {
		slogctx.Error(ctx, "failed to find tenant config", "tenantId", tenantID, "error", err)
		return nil, ErrTenantConfigSelect
	}
	if !found {
		return &model.TenantConfig{TenantID: tenantID}, nil
	}
	return cfg, nil
}

// createOrPatchTenantConfig creates the config record if it does not exist yet, otherwise patches it.
func createOrPatchTenantConfig(ctx context.Context, r repository.Repository, cfg *model.TenantConfig) error {
	probe := &model.TenantConfig{TenantID: cfg.TenantID}
	found, err := r.Find(ctx, probe)
	if err != nil {
		return ErrTenantConfigSelect
	}
	if !found {
		if err := r.Create(ctx, cfg); err != nil {
			return ErrTenantConfigUpdate
		}
		return nil
	}
	patched, err := r.Patch(ctx, cfg)
	if err != nil {
		return ErrTenantConfigUpdate
	}
	if !patched {
		return ErrTenantConfigNotFound
	}
	return nil
}

// getTenantConfig loads the TenantConfig record for tenantID, returning ErrTenantConfigNotFound
// if no record exists.
func getTenantConfig(ctx context.Context, r repository.Repository, tenantID string) (*model.TenantConfig, error) {
	cfg := &model.TenantConfig{TenantID: tenantID}
	found, err := r.Find(ctx, cfg)
	if err != nil {
		slogctx.Error(ctx, "failed to select tenant config", "tenantId", tenantID, "error", err)
		return nil, ErrTenantConfigSelect
	}
	if !found {
		return nil, ErrTenantConfigNotFound
	}
	return cfg, nil
}

// patchTenantConfig applies updateFunc to a TenantConfig stub and persists it.
func patchTenantConfig(ctx context.Context, r repository.Repository, tenantID string, updateFunc func(*model.TenantConfig)) error {
	cfg := &model.TenantConfig{TenantID: tenantID}
	updateFunc(cfg)

	found, err := r.Patch(ctx, cfg)
	if err != nil {
		slogctx.Error(ctx, "failed to patch tenant config", "tenantId", tenantID, "error", err)
		return ErrTenantConfigUpdate
	}
	if !found {
		return ErrTenantConfigNotFound
	}
	return nil
}
