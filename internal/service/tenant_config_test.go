package service_test

import (
	"context"
	"testing"

	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	tenantconfiggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant_config/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/service"
)

type errJobPreparer struct{ err error }

func (e errJobPreparer) PrepareJob(_ context.Context, _ []byte, _, _ string) error { return e.err }

// --- fake repo for TenantConfig tests ----------------------------------------

type fakeTenantConfigRepo struct {
	service.NoopRepo

	tenant          *model.Tenant
	cfg             *model.TenantConfig
	tenantFindErr   error
	cfgFindErr      error
	cfgCreateErr    error
	cfgPatchErr     error
	cfgPatchedCalls int
}

func (f *fakeTenantConfigRepo) Find(_ context.Context, resource repository.Resource) (bool, error) {
	switch r := resource.(type) {
	case *model.Tenant:
		if f.tenantFindErr != nil {
			return false, f.tenantFindErr
		}
		if f.tenant == nil {
			return false, nil
		}
		*r = *f.tenant
		return true, nil
	case *model.TenantConfig:
		if f.cfgFindErr != nil {
			return false, f.cfgFindErr
		}
		if f.cfg == nil {
			return false, nil
		}
		*r = *f.cfg
		return true, nil
	}
	return false, nil
}

func (f *fakeTenantConfigRepo) Create(_ context.Context, _ repository.Resource) error {
	return f.cfgCreateErr
}

func (f *fakeTenantConfigRepo) Patch(_ context.Context, _ repository.Resource) (bool, error) {
	if f.cfgPatchErr != nil {
		return false, f.cfgPatchErr
	}
	f.cfgPatchedCalls++
	return f.cfg != nil, nil
}

func (f *fakeTenantConfigRepo) Transaction(_ context.Context, fn repository.TransactionFunc) error {
	return fn(context.Background(), f)
}

// --- helpers -----------------------------------------------------------------

func activeTenantForConfig() *model.Tenant {
	return &model.Tenant{
		ID:        "t-1",
		Name:      "test-tenant",
		Region:    "eu-west-1",
		OwnerID:   "owner-1",
		OwnerType: "account",
		Role:      tenantgrpc.Role_ROLE_LIVE.String(),
		Status:    model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()),
	}
}

func mask(paths ...string) *fieldmaskpb.FieldMask {
	return &fieldmaskpb.FieldMask{Paths: paths}
}

func newOrbitalJob(jobType, errMsg string) orbital.Job {
	return orbital.Job{ExternalID: "t-1", Type: jobType, ErrorMessage: errMsg}
}

func getTenantConfigReq(tenantID string) *tenantconfiggrpc.GetTenantConfigRequest {
	return tenantconfiggrpc.GetTenantConfigRequest_builder{TenantId: &tenantID}.Build()
}

func updateTenantConfigReq(tenantID string, values *tenantconfiggrpc.TenantConfigurationValues, m *fieldmaskpb.FieldMask) *tenantconfiggrpc.UpdateTenantConfigRequest {
	return tenantconfiggrpc.UpdateTenantConfigRequest_builder{
		TenantId:   &tenantID,
		Values:     values,
		UpdateMask: m,
	}.Build()
}

func configValues(systemLimit int32) *tenantconfiggrpc.TenantConfigurationValues {
	return tenantconfiggrpc.TenantConfigurationValues_builder{SystemLimit: &systemLimit}.Build()
}

// --- TestGetTenantConfig -----------------------------------------------------

func TestGetTenantConfig(t *testing.T) {
	t.Run("returns InvalidArgument when tenant_id is empty", func(t *testing.T) {
		subj := service.NewTenantConfigForTest(nil)

		resp, err := subj.GetTenantConfig(context.Background(), getTenantConfigReq(""))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns Internal when repo.Find fails", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{cfgFindErr: errListFailed}
		subj := service.NewTenantConfigForTest(repo)

		resp, err := subj.GetTenantConfig(context.Background(), getTenantConfigReq("t-1"))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("returns NotFound when no config record exists", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{}
		subj := service.NewTenantConfigForTest(repo)

		resp, err := subj.GetTenantConfig(context.Background(), getTenantConfigReq("t-1"))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("returns config when record exists without values", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{
			cfg: &model.TenantConfig{
				TenantID: "t-1",
				Status:   tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_ACTIVE.String(),
			},
		}
		subj := service.NewTenantConfigForTest(repo)

		resp, err := subj.GetTenantConfig(context.Background(), getTenantConfigReq("t-1"))

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "t-1", resp.GetTenantId())
		assert.Equal(t, tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_ACTIVE, resp.GetStatus())
		assert.Nil(t, resp.GetValues())
	})

	t.Run("returns system_limit when config has a value", func(t *testing.T) {
		sl := int32(42)
		repo := &fakeTenantConfigRepo{
			cfg: &model.TenantConfig{
				TenantID:    "t-1",
				SystemLimit: &sl,
				Status:      tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_ACTIVE.String(),
			},
		}
		subj := service.NewTenantConfigForTest(repo)

		resp, err := subj.GetTenantConfig(context.Background(), getTenantConfigReq("t-1"))

		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.GetValues())
		assert.Equal(t, int32(42), resp.GetValues().GetSystemLimit())
	})
}

// --- TestUpdateTenantConfig --------------------------------------------------

func TestUpdateTenantConfig(t *testing.T) {
	t.Run("returns InvalidArgument when tenant_id is empty", func(t *testing.T) {
		subj := service.NewTenantConfigForTest(nil)

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("", nil, nil))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns InvalidArgument when update_mask is empty", func(t *testing.T) {
		subj := service.NewTenantConfigForTest(nil)

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("tenant-1", nil, &fieldmaskpb.FieldMask{}))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns InvalidArgument for unknown field mask path", func(t *testing.T) {
		subj := service.NewTenantConfigForTest(nil)

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("tenant-1", nil, mask("nonexistent_field")))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns InvalidArgument when system_limit is zero", func(t *testing.T) {
		subj := service.NewTenantConfigForTest(nil)

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("tenant-1", configValues(0), mask("system_limit")))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns InvalidArgument when system_limit is negative", func(t *testing.T) {
		subj := service.NewTenantConfigForTest(nil)

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("tenant-1", configValues(-1), mask("system_limit")))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns NotFound when tenant does not exist", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{}
		subj := service.NewTenantConfigForTest(repo)

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("t-1", configValues(10), mask("system_limit")))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("returns FailedPrecondition when tenant is not active", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{
			tenant: &model.Tenant{
				ID:     "t-1",
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String()),
			},
		}
		subj := service.NewTenantConfigForTest(repo)

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("t-1", configValues(10), mask("system_limit")))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("creates new config record and enqueues orbital job", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{tenant: activeTenantForConfig()}
		subj := service.NewTenantConfigWithOrbitalForTest(repo, noopJobPreparer{})

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("t-1", configValues(50), mask("system_limit")))

		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("patches existing config record and enqueues orbital job", func(t *testing.T) {
		sl := int32(10)
		repo := &fakeTenantConfigRepo{
			tenant: activeTenantForConfig(),
			cfg: &model.TenantConfig{
				TenantID:    "t-1",
				SystemLimit: &sl,
				Status:      tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_ACTIVE.String(),
			},
		}
		subj := service.NewTenantConfigWithOrbitalForTest(repo, noopJobPreparer{})

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("t-1", configValues(99), mask("system_limit")))

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 1, repo.cfgPatchedCalls)
	})
}

// --- TestTenantConfigJobHandlers ---------------------------------------------

func TestTenantConfigHandleJobDone(t *testing.T) {
	t.Run("patches status to ACTIVE", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{
			cfg: &model.TenantConfig{TenantID: "t-1", Status: tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING.String()},
		}
		subj := service.NewTenantConfigForTest(repo)

		err := subj.HandleJobDone(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), ""))

		require.NoError(t, err)
		assert.Equal(t, 1, repo.cfgPatchedCalls)
	})

	t.Run("no error when config not found", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{}
		subj := service.NewTenantConfigForTest(repo)

		err := subj.HandleJobDone(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), ""))

		require.NoError(t, err)
	})
}

func TestTenantConfigHandleJobFailed(t *testing.T) {
	t.Run("patches status to UPDATING_ERROR with error message", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{
			cfg: &model.TenantConfig{TenantID: "t-1", Status: tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING.String()},
		}
		subj := service.NewTenantConfigForTest(repo)

		err := subj.HandleJobFailed(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), "timeout"))

		require.NoError(t, err)
		assert.Equal(t, 1, repo.cfgPatchedCalls)
	})

	t.Run("no error when config not found", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{}
		subj := service.NewTenantConfigForTest(repo)

		err := subj.HandleJobFailed(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), "timeout"))

		require.NoError(t, err)
	})

	t.Run("returns error when patch fails", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{cfgPatchErr: errPatchFailed}
		subj := service.NewTenantConfigForTest(repo)

		err := subj.HandleJobFailed(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), "timeout"))

		require.Error(t, err)
	})
}

// --- TestTenantConfigHandleJobCanceled ---------------------------------------

func TestTenantConfigHandleJobCanceled(t *testing.T) {
	t.Run("patches status to UPDATING_ERROR", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{
			cfg: &model.TenantConfig{TenantID: "t-1", Status: tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING.String()},
		}
		subj := service.NewTenantConfigForTest(repo)

		err := subj.HandleJobCanceled(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), "canceled"))

		require.NoError(t, err)
		assert.Equal(t, 1, repo.cfgPatchedCalls)
	})

	t.Run("no error when config not found", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{}
		subj := service.NewTenantConfigForTest(repo)

		err := subj.HandleJobCanceled(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), "canceled"))

		require.NoError(t, err)
	})

	t.Run("returns error when patch fails", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{cfgPatchErr: errPatchFailed}
		subj := service.NewTenantConfigForTest(repo)

		err := subj.HandleJobCanceled(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), "canceled"))

		require.Error(t, err)
	})
}

// --- TestTenantConfigHandleJobDone (extra) -----------------------------------

func TestTenantConfigHandleJobDonePatchError(t *testing.T) {
	t.Run("returns error when patch fails", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{cfgPatchErr: errPatchFailed}
		subj := service.NewTenantConfigForTest(repo)

		err := subj.HandleJobDone(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), ""))

		require.Error(t, err)
	})
}

// --- TestConfirmJob ----------------------------------------------------------

func TestConfirmJob(t *testing.T) {
	t.Run("returns error when repo.Find fails", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{cfgFindErr: errListFailed}
		subj := service.NewTenantConfigForTest(repo)

		result, err := subj.ConfirmJob(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), ""))

		assert.Nil(t, result)
		require.Error(t, err)
	})

	t.Run("cancels when config not found", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{}
		subj := service.NewTenantConfigForTest(repo)

		result, err := subj.ConfirmJob(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), ""))

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("cancels when status is not UPDATING", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{
			cfg: &model.TenantConfig{TenantID: "t-1", Status: tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_ACTIVE.String()},
		}
		subj := service.NewTenantConfigForTest(repo)

		result, err := subj.ConfirmJob(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), ""))

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("completes when status is UPDATING", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{
			cfg: &model.TenantConfig{TenantID: "t-1", Status: tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_UPDATING.String()},
		}
		subj := service.NewTenantConfigForTest(repo)

		result, err := subj.ConfirmJob(context.Background(), newOrbitalJob(tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String(), ""))

		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// --- TestResolveTasks --------------------------------------------------------

func marshalTenant(t *testing.T, tenant *model.Tenant) []byte {
	t.Helper()
	data, err := proto.Marshal(tenant.ToProto())
	require.NoError(t, err)
	return data
}

func TestResolveTasks(t *testing.T) {
	jobType := tenantconfiggrpc.TenantConfigAction_TENANT_CONFIG_ACTION_UPDATE.String()

	t.Run("cancels when job data is not a valid proto", func(t *testing.T) {
		subj := service.NewTenantConfigForTest(nil)
		job := orbital.Job{ExternalID: "t-1", Type: jobType, Data: []byte("not-valid-proto")}

		result, err := subj.ResolveTasks(context.Background(), job, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("cancels when tenant region has no matching target", func(t *testing.T) {
		subj := service.NewTenantConfigForTest(nil)
		job := orbital.Job{
			ExternalID: "t-1",
			Type:       jobType,
			Data:       marshalTenant(t, activeTenantForConfig()), // region: eu-west-1
		}
		targets := map[string]orbital.TargetManager{"us-east-1": {}}

		result, err := subj.ResolveTasks(context.Background(), job, targets)

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("returns task for matching region", func(t *testing.T) {
		subj := service.NewTenantConfigForTest(nil)
		job := orbital.Job{
			ExternalID: "t-1",
			Type:       jobType,
			Data:       marshalTenant(t, activeTenantForConfig()), // region: eu-west-1
		}
		targets := map[string]orbital.TargetManager{"eu-west-1": {}}

		result, err := subj.ResolveTasks(context.Background(), job, targets)

		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// --- TestUpdateTenantConfig (extra error paths) ------------------------------

func TestUpdateTenantConfigErrors(t *testing.T) {
	t.Run("returns Internal when repo.Create fails", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{
			tenant:       activeTenantForConfig(),
			cfgCreateErr: errListFailed,
		}
		subj := service.NewTenantConfigWithOrbitalForTest(repo, noopJobPreparer{})

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("t-1", configValues(50), mask("system_limit")))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("returns Internal when repo.Patch fails for existing config", func(t *testing.T) {
		sl := int32(10)
		repo := &fakeTenantConfigRepo{
			tenant:      activeTenantForConfig(),
			cfg:         &model.TenantConfig{TenantID: "t-1", SystemLimit: &sl, Status: tenantconfiggrpc.TenantConfigStatus_TENANT_CONFIG_STATUS_ACTIVE.String()},
			cfgPatchErr: errPatchFailed,
		}
		subj := service.NewTenantConfigWithOrbitalForTest(repo, noopJobPreparer{})

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("t-1", configValues(99), mask("system_limit")))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("returns Internal when orbital.PrepareJob fails", func(t *testing.T) {
		repo := &fakeTenantConfigRepo{tenant: activeTenantForConfig()}
		subj := service.NewTenantConfigWithOrbitalForTest(repo, errJobPreparer{err: errListFailed})

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("t-1", configValues(50), mask("system_limit")))

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})
}
