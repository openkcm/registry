package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gofrs/uuid/v5"
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
	errListFailed  = errors.New("db down")
	errPatchFailed = errors.New("patch failed")
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

func linkedSystem(tenantID string) model.System {
	id, _ := uuid.NewV4()
	tid := tenantID
	return model.System{ID: id, ExternalID: "sys-1", Type: "application", TenantID: &tid}
}

// --- fake repo for unlinkAllSystems ------------------------------------------

type fakeUnlinkRepo struct {
	service.NoopRepo

	systems  []model.System
	listErr  error
	patchErr error

	patchedSystems []*model.System
}

func (f *fakeUnlinkRepo) List(_ context.Context, dest any, _ repository.Query) error {
	if f.listErr != nil {
		return f.listErr
	}
	if out, ok := dest.(*[]model.System); ok {
		*out = append(*out, f.systems...)
	}
	return nil
}

func (f *fakeUnlinkRepo) Patch(_ context.Context, resource repository.Resource) (bool, error) {
	if f.patchErr != nil {
		return false, f.patchErr
	}
	if s, ok := resource.(*model.System); ok {
		copied := *s
		f.patchedSystems = append(f.patchedSystems, &copied)
	}
	return true, nil
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

// --- TestUnlinkAllSystems ----------------------------------------------------

func TestUnlinkAllSystems(t *testing.T) {
	ctx := context.Background()

	t.Run("returns ErrSystemSelect when listing systems fails", func(t *testing.T) {
		repo := &fakeUnlinkRepo{listErr: errListFailed}

		err := service.UnlinkAllSystems(ctx, repo, "t-1")

		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("returns ErrSystemUpdate when patching a system fails", func(t *testing.T) {
		repo := &fakeUnlinkRepo{
			systems:  []model.System{linkedSystem("t-1")},
			patchErr: errPatchFailed,
		}

		err := service.UnlinkAllSystems(ctx, repo, "t-1")

		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("clears TenantID on all linked systems", func(t *testing.T) {
		sys := linkedSystem("t-1")
		repo := &fakeUnlinkRepo{
			systems: []model.System{sys},
		}

		err := service.UnlinkAllSystems(ctx, repo, "t-1")

		require.NoError(t, err)
		require.Len(t, repo.patchedSystems, 1)
		assert.Equal(t, sys.ID, repo.patchedSystems[0].ID)
		assert.False(t, repo.patchedSystems[0].IsLinkedToTenant())
	})

	t.Run("succeeds with no-op when no systems are linked", func(t *testing.T) {
		repo := &fakeUnlinkRepo{}

		err := service.UnlinkAllSystems(ctx, repo, "t-1")

		require.NoError(t, err)
		assert.Empty(t, repo.patchedSystems)
	})
}
