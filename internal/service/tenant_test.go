package service_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/service"
	"github.com/openkcm/registry/internal/validation"
)

var (
	errListFailed     = errors.New("db down")
	errPatchFailed    = errors.New("patch failed")
	errPatchAllFailed = errors.New("patch all failed")
)

// --- helpers -----------------------------------------------------------------

func newTenantTestValidation(t *testing.T) *validation.Validation {
	t.Helper()
	v, err := validation.New(validation.Config{
		Models: []validation.Model{&model.Tenant{}},
	})
	require.NoError(t, err)
	return v
}

func linkedSystem() model.System {
	const tenantID = "t-1"
	id := uuid.New()
	tid := tenantID
	return model.System{ID: id, ExternalID: "sys-1", Type: "application", TenantID: &tid}
}

// activeTenant returns a Tenant with STATUS_ACTIVE and all fields required by validation.
func activeTenant(id string) *model.Tenant {
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

// noopJobPreparer satisfies service.JobPreparer and always returns nil.
type noopJobPreparer struct{}

func (noopJobPreparer) PrepareJob(_ context.Context, _ []byte, _, _ string) error { return nil }

// fakeTenantRepo supports Find / Transaction / Patch for tenant unit tests.
type fakeTenantRepo struct {
	service.NoopRepo

	tenant   *model.Tenant
	findErr  error
	patchErr error
}

func (f *fakeTenantRepo) Find(_ context.Context, resource repository.Resource) (bool, error) {
	if f.findErr != nil {
		return false, f.findErr
	}
	t, ok := resource.(*model.Tenant)
	if !ok || f.tenant == nil {
		return false, nil
	}
	*t = *f.tenant
	return true, nil
}

func (f *fakeTenantRepo) Transaction(_ context.Context, fn repository.TransactionFunc) error {
	return fn(context.Background(), f)
}

func (f *fakeTenantRepo) Patch(_ context.Context, _ repository.Resource) (bool, error) {
	if f.patchErr != nil {
		return false, f.patchErr
	}
	return true, nil
}

// --- fake repo for detachAllSystems ------------------------------------------

type fakeDetachRepo struct {
	service.NoopRepo

	systems     []model.System
	listErr     error
	patchErr    error
	patchAllErr error

	patchedSystems    []*model.System
	patchAllResource  repository.Resource
	patchAllCallCount int
}

func (f *fakeDetachRepo) List(_ context.Context, dest any, _ repository.Query) error {
	if f.listErr != nil {
		return f.listErr
	}
	if out, ok := dest.(*[]model.System); ok {
		*out = append(*out, f.systems...)
	}
	return nil
}

func (f *fakeDetachRepo) Patch(_ context.Context, resource repository.Resource) (bool, error) {
	if f.patchErr != nil {
		return false, f.patchErr
	}
	if s, ok := resource.(*model.System); ok {
		copied := *s
		f.patchedSystems = append(f.patchedSystems, &copied)
	}
	return true, nil
}

func (f *fakeDetachRepo) PatchAll(_ context.Context, resource repository.Resource, _ any, _ repository.Query) (int64, error) {
	f.patchAllCallCount++
	f.patchAllResource = resource
	if f.patchAllErr != nil {
		return 0, f.patchAllErr
	}
	return 0, nil
}

// --- TestTerminateTenant -----------------------------------------------------

func TestTerminateTenant(t *testing.T) {
	t.Run("should return InvalidArgument when tenant ID is empty", func(t *testing.T) {
		subj := service.NewTenantForTest(nil, newTenantTestValidation(t))

		resp, err := subj.TerminateTenant(context.Background(), &tenantgrpc.TerminateTenantRequest{
			Id: "",
		})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

// --- TestDetachAllSystems ----------------------------------------------------

func TestDetachAllSystems(t *testing.T) {
	ctx := context.Background()

	t.Run("returns ErrSystemUpdate when PatchAll fails", func(t *testing.T) {
		repo := &fakeDetachRepo{
			systems:     []model.System{linkedSystem()},
			patchAllErr: errPatchAllFailed,
		}

		err := service.DetachAllSystems(ctx, repo, "t-1")

		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("returns ErrSystemSelect when listing systems fails", func(t *testing.T) {
		repo := &fakeDetachRepo{listErr: errListFailed}

		err := service.DetachAllSystems(ctx, repo, "t-1")

		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("returns ErrSystemUpdate when patching a system fails", func(t *testing.T) {
		repo := &fakeDetachRepo{
			systems:  []model.System{linkedSystem()},
			patchErr: errPatchFailed,
		}

		err := service.DetachAllSystems(ctx, repo, "t-1")

		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("calls PatchAll with HasL1KeyClaim=false before unlinking systems", func(t *testing.T) {
		sys := linkedSystem()
		repo := &fakeDetachRepo{
			systems: []model.System{sys},
		}

		err := service.DetachAllSystems(ctx, repo, "t-1")

		require.NoError(t, err)
		assert.Equal(t, 1, repo.patchAllCallCount)
		rs, ok := repo.patchAllResource.(*model.RegionalSystem)
		require.True(t, ok, "PatchAll resource should be *model.RegionalSystem")
		require.NotNil(t, rs.HasL1KeyClaim)
		assert.False(t, *rs.HasL1KeyClaim)
	})

	t.Run("clears TenantID on all linked systems", func(t *testing.T) {
		sys := linkedSystem()
		repo := &fakeDetachRepo{
			systems: []model.System{sys},
		}

		err := service.DetachAllSystems(ctx, repo, "t-1")

		require.NoError(t, err)
		require.Len(t, repo.patchedSystems, 1)
		assert.Equal(t, sys.ID, repo.patchedSystems[0].ID)
		assert.False(t, repo.patchedSystems[0].IsLinkedToTenant())
	})

	t.Run("succeeds with no-op when no systems are linked", func(t *testing.T) {
		repo := &fakeDetachRepo{}

		err := service.DetachAllSystems(ctx, repo, "t-1")

		require.NoError(t, err)
		assert.Empty(t, repo.patchedSystems)
		assert.Equal(t, 0, repo.patchAllCallCount)
	})
}

// --- TestApplyConfigMask -----------------------------------------------------

func TestApplyConfigMask(t *testing.T) {
	mask := func(paths ...string) *fieldmaskpb.FieldMask {
		return &fieldmaskpb.FieldMask{Paths: paths}
	}
	ptr := func(v int32) *int32 { return &v }

	t.Run("initialises Config and sets system_limit when Config is nil", func(t *testing.T) {
		tenant := &model.Tenant{}
		cfg := &tenantgrpc.TenantConfiguration{SystemLimit: ptr(5)}

		service.ApplyConfigMask(tenant, cfg, mask("system_limit"))

		require.NotNil(t, tenant.Config)
		require.NotNil(t, tenant.Config.SystemLimit)
		assert.Equal(t, int32(5), *tenant.Config.SystemLimit)
	})

	t.Run("overwrites existing system_limit", func(t *testing.T) {
		old := int32(3)
		tenant := &model.Tenant{Config: &model.TenantConfigModel{SystemLimit: &old}}
		cfg := &tenantgrpc.TenantConfiguration{SystemLimit: ptr(99)}

		service.ApplyConfigMask(tenant, cfg, mask("system_limit"))

		require.NotNil(t, tenant.Config.SystemLimit)
		assert.Equal(t, int32(99), *tenant.Config.SystemLimit)
	})

	t.Run("clears system_limit when cfg is nil", func(t *testing.T) {
		limit := int32(7)
		tenant := &model.Tenant{Config: &model.TenantConfigModel{SystemLimit: &limit}}

		service.ApplyConfigMask(tenant, nil, mask("system_limit"))

		require.NotNil(t, tenant.Config)
		assert.Nil(t, tenant.Config.SystemLimit)
	})

	t.Run("clears system_limit when cfg has no SystemLimit set", func(t *testing.T) {
		limit := int32(7)
		tenant := &model.Tenant{Config: &model.TenantConfigModel{SystemLimit: &limit}}

		service.ApplyConfigMask(tenant, &tenantgrpc.TenantConfiguration{}, mask("system_limit"))

		require.NotNil(t, tenant.Config)
		assert.Nil(t, tenant.Config.SystemLimit)
	})

	t.Run("unknown path is silently ignored", func(t *testing.T) {
		tenant := &model.Tenant{}

		service.ApplyConfigMask(tenant, &tenantgrpc.TenantConfiguration{SystemLimit: ptr(1)}, mask("unknown_field"))

		// Config was initialised but no known field was written.
		require.NotNil(t, tenant.Config)
		assert.Nil(t, tenant.Config.SystemLimit)
	})
}

// --- TestGetTenantConfig -----------------------------------------------------

func TestGetTenantConfig(t *testing.T) {
	ptr := func(v int32) *int32 { return &v }

	t.Run("returns InvalidArgument when tenant ID is empty", func(t *testing.T) {
		subj := service.NewTenantForTest(nil, newTenantTestValidation(t))

		resp, err := subj.GetTenantConfig(context.Background(), &tenantgrpc.GetTenantConfigRequest{Id: ""})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns Internal when repo.Find fails", func(t *testing.T) {
		repo := &fakeTenantRepo{findErr: errListFailed}
		subj := service.NewTenantForTest(repo, newTenantTestValidation(t))

		resp, err := subj.GetTenantConfig(context.Background(), &tenantgrpc.GetTenantConfigRequest{Id: "t-1"})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("returns NotFound when tenant does not exist", func(t *testing.T) {
		repo := &fakeTenantRepo{}
		subj := service.NewTenantForTest(repo, newTenantTestValidation(t))

		resp, err := subj.GetTenantConfig(context.Background(), &tenantgrpc.GetTenantConfigRequest{Id: "t-1"})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("returns empty config when tenant has no overrides", func(t *testing.T) {
		repo := &fakeTenantRepo{tenant: &model.Tenant{ID: "t-1"}}
		subj := service.NewTenantForTest(repo, newTenantTestValidation(t))

		resp, err := subj.GetTenantConfig(context.Background(), &tenantgrpc.GetTenantConfigRequest{Id: "t-1"})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Config.SystemLimit)
	})

	t.Run("returns system_limit when tenant has a config override", func(t *testing.T) {
		repo := &fakeTenantRepo{
			tenant: &model.Tenant{
				ID:     "t-1",
				Config: &model.TenantConfigModel{SystemLimit: ptr(42)},
			},
		}
		subj := service.NewTenantForTest(repo, newTenantTestValidation(t))

		resp, err := subj.GetTenantConfig(context.Background(), &tenantgrpc.GetTenantConfigRequest{Id: "t-1"})

		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Config.SystemLimit)
		assert.Equal(t, int32(42), *resp.Config.SystemLimit)
	})
}

// --- TestUpdateTenantConfig --------------------------------------------------

func TestUpdateTenantConfig(t *testing.T) {
	ptr := func(v int32) *int32 { return &v }
	subj := func() *service.Tenant { return service.NewTenantForTest(nil, newTenantTestValidation(t)) }

	t.Run("returns InvalidArgument when tenant ID is empty", func(t *testing.T) {
		resp, err := subj().UpdateTenantConfig(context.Background(), &tenantgrpc.UpdateTenantConfigRequest{
			Id: "",
		})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns InvalidArgument when update_mask is empty", func(t *testing.T) {
		resp, err := subj().UpdateTenantConfig(context.Background(), &tenantgrpc.UpdateTenantConfigRequest{
			Id:         "tenant-1",
			UpdateMask: &fieldmaskpb.FieldMask{},
		})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns InvalidArgument for unknown field mask path", func(t *testing.T) {
		resp, err := subj().UpdateTenantConfig(context.Background(), &tenantgrpc.UpdateTenantConfigRequest{
			Id:         "tenant-1",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"nonexistent_field"}},
		})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns InvalidArgument when system_limit is zero", func(t *testing.T) {
		zero := int32(0)
		resp, err := subj().UpdateTenantConfig(context.Background(), &tenantgrpc.UpdateTenantConfigRequest{
			Id:         "tenant-1",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"system_limit"}},
			Config:     &tenantgrpc.TenantConfiguration{SystemLimit: &zero},
		})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns InvalidArgument when system_limit is negative", func(t *testing.T) {
		neg := int32(-1)
		resp, err := subj().UpdateTenantConfig(context.Background(), &tenantgrpc.UpdateTenantConfigRequest{
			Id:         "tenant-1",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"system_limit"}},
			Config:     &tenantgrpc.TenantConfiguration{SystemLimit: &neg},
		})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("returns NotFound when tenant does not exist", func(t *testing.T) {
		repo := &fakeTenantRepo{}
		s := service.NewTenantForTest(repo, newTenantTestValidation(t))

		resp, err := s.UpdateTenantConfig(context.Background(), &tenantgrpc.UpdateTenantConfigRequest{
			Id:         "t-1",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"system_limit"}},
			Config:     &tenantgrpc.TenantConfiguration{SystemLimit: ptr(10)},
		})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("returns FailedPrecondition when tenant is not active", func(t *testing.T) {
		repo := &fakeTenantRepo{
			tenant: &model.Tenant{
				ID:     "t-1",
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String()),
			},
		}
		s := service.NewTenantForTest(repo, newTenantTestValidation(t))

		resp, err := s.UpdateTenantConfig(context.Background(), &tenantgrpc.UpdateTenantConfigRequest{
			Id:         "t-1",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"system_limit"}},
			Config:     &tenantgrpc.TenantConfiguration{SystemLimit: ptr(10)},
		})

		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("applies system_limit and returns updated config", func(t *testing.T) {
		repo := &fakeTenantRepo{tenant: activeTenant("t-1")}
		s := service.NewTenantWithOrbitalForTest(repo, noopJobPreparer{}, newTenantTestValidation(t))

		resp, err := s.UpdateTenantConfig(context.Background(), &tenantgrpc.UpdateTenantConfigRequest{
			Id:         "t-1",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"system_limit"}},
			Config:     &tenantgrpc.TenantConfiguration{SystemLimit: ptr(50)},
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Config.SystemLimit)
		assert.Equal(t, int32(50), *resp.Config.SystemLimit)
	})

	t.Run("clears system_limit when mask includes path but config field is unset", func(t *testing.T) {
		existing := int32(99)
		base := activeTenant("t-1")
		base.Config = &model.TenantConfigModel{SystemLimit: &existing}
		repo := &fakeTenantRepo{tenant: base}
		s := service.NewTenantWithOrbitalForTest(repo, noopJobPreparer{}, newTenantTestValidation(t))

		resp, err := s.UpdateTenantConfig(context.Background(), &tenantgrpc.UpdateTenantConfigRequest{
			Id:         "t-1",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"system_limit"}},
			Config:     &tenantgrpc.TenantConfiguration{},
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Config.SystemLimit)
	})
}
