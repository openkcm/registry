package service_test

import (
	"context"
	"testing"

	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	tenantconfiggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant_config/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/service"
)

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

func activeTenantForConfig(id string) *model.Tenant {
	return &model.Tenant{
		ID:        id,
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
		repo := &fakeTenantConfigRepo{tenant: activeTenantForConfig("t-1")}
		subj := service.NewTenantConfigWithOrbitalForTest(repo, noopJobPreparer{})

		resp, err := subj.UpdateTenantConfig(context.Background(), updateTenantConfigReq("t-1", configValues(50), mask("system_limit")))

		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("patches existing config record and enqueues orbital job", func(t *testing.T) {
		sl := int32(10)
		repo := &fakeTenantConfigRepo{
			tenant: activeTenantForConfig("t-1"),
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
}
