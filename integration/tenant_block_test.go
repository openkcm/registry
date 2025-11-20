//go:build integration
// +build integration

package integration_test

import (
	"testing"
	"time"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/service"
)

func TestBlockTenant(t *testing.T) {
	// given
	testCtx := newTenantTestContext(t)
	subj := testCtx.tenantClient
	db := testCtx.db
	repo := testCtx.repo
	authClient := testCtx.authClient
	ctx := t.Context()

	t.Run("BlockTenant", func(t *testing.T) {
		t.Run("should return an error if", func(t *testing.T) {
			t.Run("tenant cannot be found", func(t *testing.T) {
				// when
				actResp, err := subj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
					Id: validRandID(),
				})

				// then
				assert.Nil(t, actResp)
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})

			t.Run("tenant is in a state that prevents blocking", func(t *testing.T) {
				// given
				state := model.TenantStatus(tenantgrpc.Status_STATUS_PROVISIONING.String())
				tenant, err := persistTenant(ctx, db, validRandID(), state, time.Now())
				assert.NoError(t, err)
				t.Cleanup(func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				})

				// when
				actResp, err := subj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
					Id: tenant.ID,
				})

				// then
				assert.Nil(t, actResp)
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
			})

			t.Run("tenant has an auth with transient state, also should not update both tenant and auth", func(t *testing.T) {
				for status := range service.AuthTransientStates {
					t.Run(status, func(t *testing.T) {
						// given
						expActiveStatus := tenantgrpc.Status_STATUS_ACTIVE
						activeTenant, err := persistTenant(ctx, db, validRandID(), model.TenantStatus(expActiveStatus.String()), time.Now())
						assert.NoError(t, err)
						t.Cleanup(func() {
							err := deleteTenantFromDB(ctx, db, activeTenant)
							assert.NoError(t, err)
						})

						authWithTransient := validAuth()
						authWithTransient.TenantID = activeTenant.ID
						authWithTransient.Status = status
						err = repo.Create(ctx, authWithTransient)
						assert.NoError(t, err)
						t.Cleanup(func() {
							_, err = repo.Delete(ctx, authWithTransient)
							assert.NoError(t, err)
						})

						// when
						actResp, err := subj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
							Id: activeTenant.ID,
						})

						// then
						assert.Error(t, err)
						assert.Nil(t, actResp)

						actListResp, err := listTenants(ctx, subj)
						assert.NoError(t, err)
						assert.Len(t, actListResp.Tenants, 1)
						assert.Equal(t, expActiveStatus, actListResp.Tenants[0].Status)

						actAuth, err := authClient.GetAuth(ctx, &authgrpc.GetAuthRequest{
							ExternalId: authWithTransient.ExternalID,
						})
						assert.NoError(t, err)
						assert.Equal(t, status, actAuth.GetAuth().GetStatus().String())
					})
				}
			})
		})

		t.Run("should succeed", func(t *testing.T) {
			t.Run("when auth status is not transient, both tenant and auth statuses are set to BLOCKING", func(t *testing.T) {
				// given
				activeTenant := validTenant()
				activeTenant.Status = model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String())
				err := createTenantInDB(ctx, db, activeTenant)
				assert.NoError(t, err)
				t.Cleanup(func() {
					err := deleteTenantFromDB(ctx, db, activeTenant)
					assert.NoError(t, err)
				})

				auths, authCleanup := authWithNonTransientState(t, repo, activeTenant)
				t.Cleanup(func() {
					authCleanup(ctx)
				})

				// when
				actResp, err := subj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
					Id: activeTenant.ID,
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, actResp)
				assert.True(t, actResp.Success)

				actListResp, err := listTenants(ctx, subj)
				assert.NoError(t, err)
				assert.Len(t, actListResp.Tenants, 1)
				assert.Equal(t, tenantgrpc.Status_STATUS_BLOCKING, actListResp.Tenants[0].Status)

				assertAuthUpdatableStateConsistency(t, authClient, auths, authgrpc.AuthStatus_AUTH_STATUS_BLOCKING)
			})

			t.Run("if tenant is active", func(t *testing.T) {
				// given
				activeTenant := validTenant()
				activeTenant.Status = model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String())
				err := createTenantInDB(ctx, db, activeTenant)
				assert.NoError(t, err)
				t.Cleanup(func() {
					err := deleteTenantFromDB(ctx, db, activeTenant)
					assert.NoError(t, err)
				})

				expStatus := tenantgrpc.Status_STATUS_BLOCKING

				// when
				actResp, err := subj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
					Id: activeTenant.ID,
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, actResp)

				ltResp, err := listTenants(ctx, subj)
				assert.NoError(t, err)
				assert.Len(t, ltResp.Tenants, 1)
				assert.Equal(t, expStatus, ltResp.Tenants[0].Status)
			})
		})
	})
}
