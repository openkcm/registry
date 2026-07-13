package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/service"
	"github.com/openkcm/registry/internal/validation"
)

var (
	errPatchExploded = errors.New("db exploded")
	errCreateBoom    = errors.New("boom")
	errAuthFindBoom  = errors.New("find exploded")
)

// fakeAuthRepo is a minimal repository.Repository implementation that lets
// ApplyAuth's transaction body be driven from unit tests.
//
// Only the calls made by the ApplyAuth path are meaningfully implemented;
// the rest satisfy the interface with zero values.
type fakeAuthRepo struct {
	tenant *model.Tenant
	// existingAuth, when non-nil, will be returned from Find(*model.Auth)
	// with a matching ExternalID, and its content copied into the argument.
	existingAuth *model.Auth

	// authFindErr is returned from Find when looking up an *model.Auth.
	authFindErr error

	createErr error

	patchFound bool
	patchErr   error

	authFindCalled bool
	patchCalled    bool
	createCalled   bool

	patched *model.Auth
	created *model.Auth
}

func (f *fakeAuthRepo) Transaction(ctx context.Context, txFunc repository.TransactionFunc) error {
	return txFunc(ctx, f)
}

func (f *fakeAuthRepo) Find(_ context.Context, resource repository.Resource) (bool, error) {
	switch r := resource.(type) {
	case *model.Tenant:
		if f.tenant == nil || f.tenant.ID != r.ID {
			return false, nil
		}
		*r = *f.tenant
		return true, nil
	case *model.Auth:
		f.authFindCalled = true
		if f.authFindErr != nil {
			return false, f.authFindErr
		}
		if f.existingAuth == nil || f.existingAuth.ExternalID != r.ExternalID {
			return false, nil
		}
		*r = *f.existingAuth
		return true, nil
	default:
		return false, nil
	}
}

func (f *fakeAuthRepo) Create(_ context.Context, resource repository.Resource) error {
	f.createCalled = true
	if a, ok := resource.(*model.Auth); ok {
		copied := *a
		f.created = &copied
	}
	return f.createErr
}

func (f *fakeAuthRepo) Patch(_ context.Context, resource repository.Resource) (bool, error) {
	f.patchCalled = true
	if a, ok := resource.(*model.Auth); ok {
		copied := *a
		f.patched = &copied
	}
	return f.patchFound, f.patchErr
}

func (f *fakeAuthRepo) List(_ context.Context, _ any, _ repository.Query) error {
	return nil
}

func (f *fakeAuthRepo) Delete(_ context.Context, _ repository.Resource) (bool, error) {
	return false, nil
}

func (f *fakeAuthRepo) PatchAll(_ context.Context, _ repository.Resource, _ any, _ repository.Query) (int64, error) {
	return 0, nil
}

func newTestValidation(t *testing.T) *validation.Validation {
	t.Helper()
	v, err := validation.New(validation.Config{
		Models: []validation.Model{&model.Auth{}, &model.Tenant{}},
	})
	require.NoError(t, err)
	return v
}

func newActiveTenant() *model.Tenant {
	return &model.Tenant{
		ID:     "tenant-1",
		Region: "eu-1",
		Status: model.TenantStatus(pb.Status_STATUS_ACTIVE.String()),
	}
}

func validApplyAuthRequest() *authgrpc.ApplyAuthRequest {
	return &authgrpc.ApplyAuthRequest{
		ExternalId: "auth-1",
		TenantId:   "tenant-1",
		Type:       "oidc",
		Properties: map[string]string{"requiredProperty": "value"},
	}
}

func TestApplyAuth_PatchOnExistingResource(t *testing.T) {
	t.Run("patches when an auth with the same external ID already exists", func(t *testing.T) {
		// given
		existing := &model.Auth{
			ExternalID: "auth-1",
			TenantID:   "some-other-tenant",
			Type:       "oidc",
			Status:     authgrpc.AuthStatus_AUTH_STATUS_APPLIED.String(),
		}
		repo := &fakeAuthRepo{
			tenant:       newActiveTenant(),
			existingAuth: existing,
			patchFound:   true,
		}

		auth := service.NewAuthForTest(repo, nil, newTestValidation(t))

		// when
		resp, err := auth.ApplyAuth(context.Background(), validApplyAuthRequest())

		// then
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		assert.True(t, repo.authFindCalled, "Find must be attempted to detect an existing auth")
		assert.True(t, repo.patchCalled, "Patch must be invoked when the auth already exists")
		assert.False(t, repo.createCalled, "Create must not be attempted when the auth already exists")

		require.NotNil(t, repo.patched)
		assert.Equal(t, "auth-1", repo.patched.ExternalID)
		assert.Equal(t, "tenant-1", repo.patched.TenantID)
		assert.Equal(t, "oidc", repo.patched.Type)
		assert.Equal(t, "value", repo.patched.Properties["requiredProperty"])
		assert.Equal(t, authgrpc.AuthStatus_AUTH_STATUS_APPLYING.String(), repo.patched.Status)
	})

	t.Run("returns Internal error when Patch fails", func(t *testing.T) {
		// given
		repo := &fakeAuthRepo{
			tenant:       newActiveTenant(),
			existingAuth: &model.Auth{ExternalID: "auth-1"},
			patchErr:     errPatchExploded,
		}

		auth := service.NewAuthForTest(repo, nil, newTestValidation(t))

		// when
		resp, err := auth.ApplyAuth(context.Background(), validApplyAuthRequest())

		// then
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.True(t, repo.patchCalled)
		assert.False(t, repo.createCalled)
	})

	t.Run("returns Internal error when the pre-check Find fails", func(t *testing.T) {
		// given
		repo := &fakeAuthRepo{
			tenant:      newActiveTenant(),
			authFindErr: errAuthFindBoom,
		}

		auth := service.NewAuthForTest(repo, nil, newTestValidation(t))

		// when
		resp, err := auth.ApplyAuth(context.Background(), validApplyAuthRequest())

		// then
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.False(t, repo.createCalled)
		assert.False(t, repo.patchCalled)
	})

	t.Run("returns Internal error when Create fails for a fresh auth", func(t *testing.T) {
		// given: no existing auth, Create fails.
		repo := &fakeAuthRepo{
			tenant:    newActiveTenant(),
			createErr: errCreateBoom,
		}

		auth := service.NewAuthForTest(repo, nil, newTestValidation(t))

		// when
		resp, err := auth.ApplyAuth(context.Background(), validApplyAuthRequest())

		// then
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.True(t, repo.authFindCalled)
		assert.True(t, repo.createCalled)
		assert.False(t, repo.patchCalled, "Patch must not run when Create is exercised on a fresh auth")
	})
}

func TestApplyAuth_InvalidRequest(t *testing.T) {
	t.Run("returns InvalidArgument when the request fails validation", func(t *testing.T) {
		// given
		repo := &fakeAuthRepo{tenant: newActiveTenant()}
		auth := service.NewAuthForTest(repo, nil, newTestValidation(t))

		req := validApplyAuthRequest()
		req.ExternalId = "" // triggers non-empty constraint on Auth.ExternalID

		// when
		resp, err := auth.ApplyAuth(context.Background(), req)

		// then
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.False(t, repo.createCalled, "Create must not run when validation fails")
		assert.False(t, repo.patchCalled)
	})
}

func TestApplyAuth_TenantNotActive(t *testing.T) {
	t.Run("returns FailedPrecondition when linked tenant is not active", func(t *testing.T) {
		// given
		blockedTenant := newActiveTenant()
		blockedTenant.Status = model.TenantStatus(pb.Status_STATUS_BLOCKED.String())

		repo := &fakeAuthRepo{tenant: blockedTenant}
		auth := service.NewAuthForTest(repo, nil, newTestValidation(t))

		// when
		resp, err := auth.ApplyAuth(context.Background(), validApplyAuthRequest())

		// then
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
		assert.False(t, repo.createCalled)
		assert.False(t, repo.patchCalled)
	})

	t.Run("returns NotFound when linked tenant does not exist", func(t *testing.T) {
		// given
		repo := &fakeAuthRepo{tenant: nil}
		auth := service.NewAuthForTest(repo, nil, newTestValidation(t))

		// when
		resp, err := auth.ApplyAuth(context.Background(), validApplyAuthRequest())

		// then
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.NotFound, status.Code(err))
		assert.False(t, repo.createCalled)
		assert.False(t, repo.patchCalled)
	})
}
