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

func TestTenantUnblock(t *testing.T) {
	// given
	testCtx := newTenantTestContext(t)
	subj := testCtx.tenantClient
	db := testCtx.db
	repo := testCtx.repo
	authClient := testCtx.authClient

	ctx := t.Context()

	t.Run("UnblockTenant", func(t *testing.T) {
		t.Run("should return an error if", func(t *testing.T) {
			t.Run("tenant cannot be found", func(t *testing.T) {
				// when
				actResp, err := subj.UnblockTenant(ctx, &tenantgrpc.UnblockTenantRequest{
					Id: validRandID(),
				})

				// then
				assert.Nil(t, actResp)
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})

			t.Run("tenant is in a state that prevents unblocking", func(t *testing.T) {
				// given
				state := model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATED.String())
				tenant, err := persistTenant(ctx, db, validRandID(), state, time.Now())
				assert.NoError(t, err)
				t.Cleanup(func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				})

				// when
				actResp, err := subj.UnblockTenant(ctx, &tenantgrpc.UnblockTenantRequest{
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
						expBlockedStatus := tenantgrpc.Status_STATUS_BLOCKED
						blockedTenant, err := persistTenant(ctx, db, validRandID(), model.TenantStatus(expBlockedStatus.String()), time.Now())
						assert.NoError(t, err)
						t.Cleanup(func() {
							err := deleteTenantFromDB(ctx, db, blockedTenant)
							assert.NoError(t, err)
						})

						authWithTransientState := validAuth()
						authWithTransientState.TenantID = blockedTenant.ID
						authWithTransientState.Status = status
						err = repo.Create(ctx, authWithTransientState)
						assert.NoError(t, err)
						t.Cleanup(func() {
							_, err = repo.Delete(ctx, authWithTransientState)
							assert.NoError(t, err)
						})

						// when
						actResp, err := subj.UnblockTenant(ctx, &tenantgrpc.UnblockTenantRequest{
							Id: blockedTenant.ID,
						})

						// then
						assert.Error(t, err)
						assert.Nil(t, actResp)

						actListResp, err := listTenants(ctx, subj)
						assert.NoError(t, err)
						assert.Len(t, actListResp.Tenants, 1)
						assert.Equal(t, expBlockedStatus, actListResp.Tenants[0].Status)

						actAuth, err := authClient.GetAuth(ctx, &authgrpc.GetAuthRequest{
							ExternalId: authWithTransientState.ExternalID,
						})
						assert.NoError(t, err)
						assert.Equal(t, status, actAuth.GetAuth().GetStatus().String())
					})
				}
			})
		})
		t.Run("should succeed", func(t *testing.T) {
			t.Run("if tenant is blocked", func(t *testing.T) {
				// given
				blockedTenant := validTenant()
				blockedTenant.Status = model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String())
				err := createTenantInDB(ctx, db, blockedTenant)
				assert.NoError(t, err)

				t.Cleanup(func() {
					err := deleteTenantFromDB(ctx, db, blockedTenant)
					assert.NoError(t, err)
				})

				expStatus := tenantgrpc.Status_STATUS_UNBLOCKING

				// when
				actResp, err := subj.UnblockTenant(ctx, &tenantgrpc.UnblockTenantRequest{
					Id: blockedTenant.ID,
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, actResp)

				ltResp, err := listTenants(ctx, subj)
				assert.NoError(t, err)
				assert.Len(t, ltResp.Tenants, 1)
				assert.Equal(t, expStatus, ltResp.Tenants[0].Status)
			})

			t.Run("when auth status is not transient, both tenant and auth statuses are set to UNBLOCKING", func(t *testing.T) {
				// given
				blockedTenant := validTenant()
				blockedTenant.Status = model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String())
				err := createTenantInDB(ctx, db, blockedTenant)
				assert.NoError(t, err)
				t.Cleanup(func() {
					err := deleteTenantFromDB(ctx, db, blockedTenant)
					assert.NoError(t, err)
				})

				auths, authCleanup := authWithNonTransientState(t, repo, blockedTenant)
				t.Cleanup(func() {
					authCleanup(ctx)
				})

				// when
				actResp, err := subj.UnblockTenant(ctx, &tenantgrpc.UnblockTenantRequest{
					Id: blockedTenant.ID,
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, actResp)
				assert.True(t, actResp.Success)

				actListResp, err := listTenants(ctx, subj)
				assert.NoError(t, err)
				assert.Len(t, actListResp.Tenants, 1)
				assert.Equal(t, tenantgrpc.Status_STATUS_UNBLOCKING, actListResp.Tenants[0].Status)

				assertAuthUpdatableStateConsistency(t, authClient, auths, authgrpc.AuthStatus_AUTH_STATUS_UNBLOCKING)
			})
		})
	})
}
