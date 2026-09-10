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
