package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/openkcm/orbital"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

// Auth implements the procedure calls defined as protobufs.
// See https://github.com/openkcm/api-sdk/blob/main/proto/kms/api/cmk/registry/auth/v1/auth.proto.
type Auth struct {
	authgrpc.UnimplementedServiceServer

	repo       repository.Repository
	orbital    *Orbital
	validation *validation.Validation
}

type (
	authValidateFunc   func(*model.Auth) error
	authUpdateFunc     func(*model.Auth)
	authSkipUpdateFunc func(*model.Auth) bool

	patchAuthOpts struct {
		validateFn   authValidateFunc
		skipUpdateFn authSkipUpdateFunc
		updateFn     authUpdateFunc
	}
)

var AuthTransientStates = map[string]struct{}{
	authgrpc.AuthStatus_AUTH_STATUS_APPLYING.String():   {},
	authgrpc.AuthStatus_AUTH_STATUS_REMOVING.String():   {},
	authgrpc.AuthStatus_AUTH_STATUS_BLOCKING.String():   {},
	authgrpc.AuthStatus_AUTH_STATUS_UNBLOCKING.String(): {},
}

var AuthNonUpdatableState = map[string]struct{}{
	authgrpc.AuthStatus_AUTH_STATUS_REMOVED.String():        {},
	authgrpc.AuthStatus_AUTH_STATUS_APPLYING_ERROR.String(): {},
}

// NewAuth creates and return a new instance of Auth.
// It also registers the job handlers to the Orbital instance.
func NewAuth(repo repository.Repository, orbital *Orbital, validation *validation.Validation) *Auth {
	a := &Auth{
		repo:       repo,
		orbital:    orbital,
		validation: validation,
	}

	for _, jobType := range []string{
		authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String(),
		authgrpc.AuthAction_AUTH_ACTION_REMOVE_AUTH.String(),
	} {
		orbital.RegisterJobHandler(jobType, a)
	}
	return a
}

// ApplyAuth creates a new auth and starts a job to apply it to the linked tenant.
// If an auth with the same external ID already exists, it returns success to make the action idempotent.
func (a *Auth) ApplyAuth(ctx context.Context, req *authgrpc.ApplyAuthRequest) (*authgrpc.ApplyAuthResponse, error) {
	ctx = slogctx.With(ctx, "externalID", req.ExternalId, "tenantID", req.TenantId, "type", req.Type, "properties", req.Properties)
	slogctx.Debug(ctx, "applying auth")

	auth := &model.Auth{
		ExternalID: req.ExternalId,
		TenantID:   req.TenantId,
		Type:       req.Type,
		Properties: req.Properties,
		Status:     authgrpc.AuthStatus_AUTH_STATUS_APPLYING.String(),
	}

	err := a.validateAuth(auth)
	if err != nil {
		return nil, err
	}

	err = a.repo.Transaction(ctx, func(ctx context.Context, r repository.Repository) error {
		err := a.validateActiveTenant(ctx, r, auth.TenantID)
		if err != nil {
			slogctx.Error(ctx, "tenant is invalid or not active", "error", err)
			return err
		}

		err = r.Create(ctx, auth)
		if err != nil {
			slogctx.Error(ctx, "failed to create auth", "error", err)
			var ucErr *repository.UniqueConstraintError
			if errors.As(err, &ucErr) {
				slogctx.Info(ctx, AuthAlreadyExistsMsg, "detail", ucErr.Detail)
				return ErrAuthAlreadyExists
			}

			return status.Error(codes.Internal, "failed to create auth")
		}

		err = a.prepareJob(ctx, auth, authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String())
		if err != nil {
			slogctx.Error(ctx, "failed to prepare job", "error", err)
			return err
		}

		return nil
	})
	err = mapError(err)
	if err != nil && !errors.Is(err, ErrAuthAlreadyExists) {
		return nil, err
	}

	return &authgrpc.ApplyAuthResponse{
		Success: true,
	}, nil
}

// GetAuth retrieves an auth by its external ID.
func (a *Auth) GetAuth(ctx context.Context, req *authgrpc.GetAuthRequest) (*authgrpc.GetAuthResponse, error) {
	ctx = slogctx.With(ctx, "externalID", req.ExternalId)
	slogctx.Debug(ctx, "getting auth")

	err := a.validation.Validate(model.AuthExternalIDValidationID, req.ExternalId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid external ID: %v", err)
	}

	auth, err := getAuth(ctx, a.repo, req.ExternalId)
	if errors.Is(err, ErrAuthNotFound) {
		return nil, status.Error(codes.NotFound, "auth not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get auth")
	}

	return &authgrpc.GetAuthResponse{
		Auth: auth.ToProto(),
	}, nil
}

// RemoveAuth marks an auth for removal by its external ID and starts a job to remove it from the linked tenant.
// If the auth does not exist or is not in APPLIED status, it returns an error.
// If the linked tenant does not exist or is not active, it returns an error.
func (a *Auth) RemoveAuth(ctx context.Context, req *authgrpc.RemoveAuthRequest) (*authgrpc.RemoveAuthResponse, error) {
	ctx = slogctx.With(ctx, "externalID", req.ExternalId)
	slogctx.Debug(ctx, "removing auth")

	err := a.validation.Validate(model.AuthExternalIDValidationID, req.ExternalId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid external ID: %v", err)
	}

	err = a.repo.Transaction(ctx, func(ctx context.Context, r repository.Repository) error {
		auth, err := getAuth(ctx, r, req.ExternalId)
		if err != nil {
			return err
		}

		if auth.Status != authgrpc.AuthStatus_AUTH_STATUS_APPLIED.String() {
			slogctx.Error(ctx, AuthInvalidStatusMsg, "status", auth.Status)
			return ErrorWithParams(ErrAuthInvalidStatus, "status", auth.Status)
		}

		err = a.validateActiveTenant(ctx, r, auth.TenantID)
		if err != nil {
			slogctx.Error(ctx, "tenant is invalid or not active", "error", err)
			return err
		}

		err = patchAuth(ctx, r,
			req.ExternalId,
			func(auth *model.Auth) {
				auth.Status = authgrpc.AuthStatus_AUTH_STATUS_REMOVING.String()
			},
		)
		if err != nil {
			return err
		}

		err = a.prepareJob(ctx, auth, authgrpc.AuthAction_AUTH_ACTION_REMOVE_AUTH.String())
		if err != nil {
			slogctx.Error(ctx, "failed to prepare job", "error", err)
			return err
		}

		return nil
	})
	err = mapError(err)
	if err != nil {
		return nil, err
	}

	return &authgrpc.RemoveAuthResponse{
		Success: true,
	}, nil
}

// ConfirmJob confirms that the auth associated with the job exists.
func (a *Auth) ConfirmJob(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error) {
	auth, err := getAuth(ctx, a.repo, job.ExternalID)
	if err != nil {
		slogctx.Error(ctx, "failed to get auth for job confirmation", "error", err)
		return orbital.JobConfirmResult{}, err
	}

	switch job.Type {
	case authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String():
		return orbital.JobConfirmResult{Done: true}, nil
	case authgrpc.AuthAction_AUTH_ACTION_REMOVE_AUTH.String():
		if auth.Status != authgrpc.AuthStatus_AUTH_STATUS_REMOVING.String() {
			slogctx.Error(ctx, AuthInvalidStatusMsg, "status", auth.Status)
			return orbital.JobConfirmResult{
				IsCanceled:           true,
				CanceledErrorMessage: fmt.Sprintf("%s: %s", ErrAuthInvalidStatus, auth.Status),
			}, nil
		}
		return orbital.JobConfirmResult{Done: true}, nil
	default:
		slogctx.Error(ctx, "unexpected job type for auth")
		return orbital.JobConfirmResult{
			IsCanceled:           true,
			CanceledErrorMessage: fmt.Sprintf("%s: %s", ErrUnexpectedJobType, job.Type),
		}, nil
	}
}

// ResolveTasks determines the tasks to be executed for a given job.
func (a *Auth) ResolveTasks(ctx context.Context, job orbital.Job, targetsByRegion map[string]orbital.ManagerTarget) (orbital.TaskResolverResult, error) {
	auth := &authgrpc.Auth{}
	err := proto.Unmarshal(job.Data, auth)
	if err != nil {
		slogctx.Error(ctx, "failed to decode auth proto", "error", err)
		return orbital.TaskResolverResult{
			IsCanceled:           true,
			CanceledErrorMessage: fmt.Sprintf("failed to decode auth proto: %v", err),
		}, nil
	}
	ctx = slogctx.With(ctx, "tenantID", auth.TenantId)

	tenant, err := getTenant(ctx, a.repo, auth.TenantId)
	if err != nil {
		slogctx.Error(ctx, "failed to get tenant for resolving tasks for auth", "error", err)
		return orbital.TaskResolverResult{}, err
	}

	_, ok := targetsByRegion[tenant.Region]
	if !ok {
		slogctx.Error(ctx, "no target for region", "region", tenant.Region)
		return orbital.TaskResolverResult{
			IsCanceled:           true,
			CanceledErrorMessage: "no target for region: " + tenant.Region,
		}, nil
	}

	return orbital.TaskResolverResult{
		TaskInfos: []orbital.TaskInfo{
			{
				Data:   job.Data,
				Type:   job.Type,
				Target: tenant.Region,
			},
		},
		Done: true,
	}, nil
}

// HandleJobDone updates auth when the job is done.
func (a *Auth) HandleJobDone(ctx context.Context, job orbital.Job) error {
	var status authgrpc.AuthStatus
	switch job.Type {
	case authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String():
		status = authgrpc.AuthStatus_AUTH_STATUS_APPLIED
	case authgrpc.AuthAction_AUTH_ACTION_REMOVE_AUTH.String():
		status = authgrpc.AuthStatus_AUTH_STATUS_REMOVED
	default:
		slogctx.Error(ctx, ErrUnexpectedJobType.Error())
		return nil
	}

	err := patchAuth(ctx, a.repo,
		job.ExternalID,
		func(auth *model.Auth) {
			auth.Status = status.String()
		},
	)
	if errors.Is(err, ErrAuthNotFound) {
		slogctx.Warn(ctx, "auth not found for job done")
		return nil
	}
	return err
}

// HandleJobCanceled updates auth when the job is canceled.
func (a *Auth) HandleJobCanceled(ctx context.Context, job orbital.Job) error {
	return a.handleJobAborted(ctx, job)
}

// HandleJobFailed updates auth when the job is failed.
func (a *Auth) HandleJobFailed(ctx context.Context, job orbital.Job) error {
	return a.handleJobAborted(ctx, job)
}

func (a *Auth) validateActiveTenant(ctx context.Context, r repository.Repository, tenantID string) error {
	tenant, err := getTenant(ctx, r, tenantID)
	if err != nil {
		return err
	}
	return checkTenantActive(tenant)
}

func (a *Auth) validateAuth(auth *model.Auth) error {
	valuesByID, err := validation.GetValues(auth)
	if err != nil {
		return status.Error(codes.Internal, "failed to get auth values by validation ID")
	}

	err = a.validation.ValidateAll(valuesByID)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid auth: %v", err)
	}

	return nil
}

func (a *Auth) prepareJob(ctx context.Context, auth *model.Auth, jobType string) error {
	authData, err := proto.Marshal(auth.ToProto())
	if err != nil {
		return status.Error(codes.Internal, "failed to marshal auth proto")
	}

	err = a.orbital.PrepareJob(ctx,
		authData,
		auth.ExternalID,
		jobType,
	)
	if err != nil {
		return status.Error(codes.Internal, "failed to start auth job")
	}

	return nil
}

func (a *Auth) handleJobAborted(ctx context.Context, job orbital.Job) error {
	var status authgrpc.AuthStatus
	switch job.Type {
	case authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String():
		status = authgrpc.AuthStatus_AUTH_STATUS_APPLYING_ERROR
	case authgrpc.AuthAction_AUTH_ACTION_REMOVE_AUTH.String():
		status = authgrpc.AuthStatus_AUTH_STATUS_REMOVING_ERROR
	default:
		slogctx.Error(ctx, ErrUnexpectedJobType.Error())
		return nil
	}

	err := patchAuth(ctx, a.repo,
		job.ExternalID,
		func(auth *model.Auth) {
			auth.Status = status.String()
			auth.ErrorMessage = job.ErrorMessage
		},
	)
	if errors.Is(err, ErrAuthNotFound) {
		slogctx.Warn(ctx, "auth not found for job aborted")
		return nil
	}
	return err
}

// apply applies update and/or validate functions to all auths for a given tenantID.
//
//nolint:cyclop
func (opts patchAuthOpts) apply(ctx context.Context, r repository.Repository, tenantID string) error {
	if opts.validateFn == nil && opts.updateFn == nil {
		return nil
	}
	// get all auths for the tenantID
	cond := repository.NewCompositeKey().Where(repository.TenantIDField, tenantID)
	var auths []model.Auth
	if err := r.List(ctx, &auths, *repository.NewQuery(&model.Auth{}).Where(cond)); err != nil {
		return ErrAuthSelect
	}

	// iterate through all auths and apply the update and/or validate functions
	for _, auth := range auths {
		if opts.validateFn != nil {
			err := opts.validateFn(&auth)
			if err != nil {
				return err
			}
		}
		if opts.updateFn != nil {
			if opts.skipUpdateFn != nil && opts.skipUpdateFn(&auth) {
				continue
			}
			err := patchAuth(ctx, r, auth.ExternalID, opts.updateFn)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getAuth(ctx context.Context, r repository.Repository, id string) (*model.Auth, error) {
	auth := &model.Auth{
		ExternalID: id,
	}

	found, err := r.Find(ctx, auth)
	if err != nil {
		slogctx.Error(ctx, SelectAuthErrMsg, "error", err)
		return nil, fmt.Errorf("%w: %w", ErrAuthSelect, err)
	}
	if !found {
		return nil, ErrAuthNotFound
	}

	return auth, nil
}

func patchAuth(ctx context.Context, r repository.Repository, id string, updateFunc func(*model.Auth)) error {
	auth := &model.Auth{
		ExternalID: id,
	}
	updateFunc(auth)

	found, err := r.Patch(ctx, auth)
	if err != nil {
		slogctx.Error(ctx, UpdateAuthErrMsg, "error", err)
		return fmt.Errorf("%w: %w", ErrAuthUpdate, err)
	}
	if !found {
		return ErrAuthNotFound
	}

	return nil
}
