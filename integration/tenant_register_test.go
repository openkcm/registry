//go:build integration
// +build integration

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/model"
)

func TestTenantRegister(t *testing.T) {
	testCtx := newTenantTestContext(t)
	subj := testCtx.tenantClient
	db := testCtx.db
	ctx := t.Context()

	t.Run("RegisterTenant", func(t *testing.T) {
		t.Run("should return an error if", func(t *testing.T) {
			t.Run("provisioned tenant with the same ID already exists", func(t *testing.T) {
				// given
				activeTenant := validTenant()
				activeTenant.Status = model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String())

				err := createTenantInDB(ctx, db, activeTenant)
				assert.NoError(t, err)
				t.Cleanup(func() {
					err = deleteTenantFromDB(ctx, db, activeTenant)
					assert.NoError(t, err)
				})

				req := validRegisterTenantReq()
				req.Id = activeTenant.ID

				// when
				actResp, err := subj.RegisterTenant(ctx, req)

				// then
				assert.Nil(t, actResp)
				assert.Error(t, err)
				assert.Equal(t, codes.AlreadyExists, status.Code(err), err.Error())
			})
		})

		t.Run("should upsert a tenant in PROVISIONING state if", func(t *testing.T) {
			t.Run("tenant with given ID does not exist", func(t *testing.T) {
				// given
				req := validRegisterTenantReq()
				tenantID := req.Id
				t.Cleanup(func() {
					err := deleteTenantFromDB(ctx, db, &model.Tenant{ID: tenantID})
					assert.NoError(t, err)
				})

				// when
				actResp, err := subj.RegisterTenant(ctx, req)

				// then
				assert.NoError(t, err)
				assert.NotNil(t, actResp)
				assert.Equal(t, tenantID, actResp.GetId())

				actListResp, err := listTenants(ctx, subj)
				assert.NoError(t, err)
				assert.Len(t, actListResp.Tenants, 1)
				assert.Equal(t, tenantgrpc.Status_STATUS_PROVISIONING, actListResp.Tenants[0].Status)
			})

			t.Run("tenant is in PROVISIONING_ERROR state", func(t *testing.T) {
				// given
				errorTenant := validTenant()
				errorTenant.Status = model.TenantStatus(tenantgrpc.Status_STATUS_PROVISIONING_ERROR.String())

				err := createTenantInDB(ctx, db, errorTenant)
				assert.NoError(t, err)
				t.Cleanup(func() {
					err = deleteTenantFromDB(ctx, db, errorTenant)
					assert.NoError(t, err)
				})

				req := validRegisterTenantReq()
				req.Id = errorTenant.ID

				// when
				actResp, err := subj.RegisterTenant(ctx, req)

				// then
				assert.NoError(t, err)
				assert.NotNil(t, actResp)
				assert.Equal(t, errorTenant.ID, actResp.GetId())

				actListResp, err := listTenants(ctx, subj)
				assert.NoError(t, err)
				assert.Len(t, actListResp.Tenants, 1)
				assert.Equal(t, tenantgrpc.Status_STATUS_PROVISIONING, actListResp.Tenants[0].Status)
			})
		})
	})
}
