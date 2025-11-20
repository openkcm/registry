//go:build integration
// +build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/integration/operatortest"
	"github.com/openkcm/registry/internal/model"
)

type expStateFunc func(*tenantgrpc.Tenant) bool

func TestTenantReconciliation(t *testing.T) {
	// given
	testCtx := newTenantTestContext(t)
	subj := testCtx.tenantClient
	db := testCtx.db
	repo := testCtx.repo
	authClient := testCtx.authClient
	ctx := t.Context()

	operator, err := operatortest.New(ctx)
	require.NoError(t, err)

	go operator.ListenAndRespond(ctx)

	t.Run("ProvisionTenant", func(t *testing.T) {
		tests := []struct {
			name     string
			tenantID string
			region   string
			expState expStateFunc
		}{
			{
				name:     "should change status to PROVISIONING_ERROR if region is not found",
				tenantID: "test-tenant-cancel",
				region:   "region-2",
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_PROVISIONING_ERROR
				},
			},
			{
				name:     "should change status to PROVISIONING_ERROR if operator fails to process the request",
				tenantID: operatortest.TenantIDFail,
				region:   operatortest.Region,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_PROVISIONING_ERROR
				},
			},
			{
				name:     "should change status to ACTIVE if tenant is provisioned successfully",
				tenantID: operatortest.TenantIDSuccess,
				region:   operatortest.Region,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_ACTIVE
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				req := validRegisterTenantReq()
				req.Id = tt.tenantID
				req.Region = tt.region

				// when
				_, err := subj.RegisterTenant(ctx, req)
				assert.NoError(t, err)
				t.Cleanup(func() {
					err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req.GetId()})
					assert.NoError(t, err)
				})

				// then
				err = waitForTenantReconciliation(ctx, subj, req.GetId(), tt.expState)
				assert.NoError(t, err)
			})
		}
	})

	t.Run("BlockTenant", func(t *testing.T) {
		tests := []struct {
			name         string
			tenantID     string
			expState     expStateFunc
			expAuthState authgrpc.AuthStatus
		}{
			{
				name:     "should change status to BLOCKING_ERROR if operator fails to process the request",
				tenantID: operatortest.TenantIDFail,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_BLOCKING_ERROR
				},
				expAuthState: authgrpc.AuthStatus_AUTH_STATUS_BLOCKING_ERROR,
			},
			{
				name:     "should change status to BLOCKED if tenant is blocked successfully",
				tenantID: operatortest.TenantIDSuccess,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_BLOCKED
				},
				expAuthState: authgrpc.AuthStatus_AUTH_STATUS_BLOCKED,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				tenant := validTenant()
				tenant.ID = tt.tenantID
				err := createTenantInDB(ctx, db, tenant)
				assert.NoError(t, err)

				t.Cleanup(func() {
					err := deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				})

				auths, authCleanup := authWithNonTransientState(t, repo, tenant)
				t.Cleanup(func() {
					authCleanup(ctx)
				})

				// when
				_, err = subj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
					Id: tenant.ID,
				})
				assert.NoError(t, err)

				// then
				err = waitForTenantReconciliation(ctx, subj, tenant.ID, tt.expState)
				assert.NoError(t, err)
				assertAuthUpdatableStateConsistency(t, authClient, auths, tt.expAuthState)
			})
		}
	})

	t.Run("UnblockTenant", func(t *testing.T) {
		tests := []struct {
			name         string
			tenantID     string
			expState     expStateFunc
			expAuthState authgrpc.AuthStatus
		}{
			{
				name:     "should change status to UNBLOCKING_ERROR if operator fails to process the request",
				tenantID: operatortest.TenantIDFail,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_UNBLOCKING_ERROR
				},
				expAuthState: authgrpc.AuthStatus_AUTH_STATUS_UNBLOCKING_ERROR,
			},
			{
				name:     "should change status to ACTIVE if tenant is unblocked successfully",
				tenantID: operatortest.TenantIDSuccess,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_ACTIVE
				},
				expAuthState: authgrpc.AuthStatus_AUTH_STATUS_APPLIED,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				tenant := validTenant()
				tenant.ID = tt.tenantID
				tenant.Status = model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String())
				err := createTenantInDB(ctx, db, tenant)
				assert.NoError(t, err)

				t.Cleanup(func() {
					err := deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				})

				auths, authCleanupFns := authWithNonTransientState(t, repo, tenant)
				t.Cleanup(func() {
					authCleanupFns(ctx)
				})

				// when
				_, err = subj.UnblockTenant(ctx, &tenantgrpc.UnblockTenantRequest{
					Id: tenant.ID,
				})
				assert.NoError(t, err)

				// then
				err = waitForTenantReconciliation(ctx, subj, tenant.ID, tt.expState)
				assert.NoError(t, err)

				assertAuthUpdatableStateConsistency(t, authClient, auths, tt.expAuthState)
			})
		}
	})

	t.Run("TerminateTenant", func(t *testing.T) {
		tests := []struct {
			name         string
			tenantID     string
			expState     expStateFunc
			expAuthState authgrpc.AuthStatus
		}{
			{
				name:     "should change status to TERMINATION_ERROR if operator fails to process the request",
				tenantID: operatortest.TenantIDFail,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_TERMINATION_ERROR
				},
				expAuthState: authgrpc.AuthStatus_AUTH_STATUS_REMOVING_ERROR,
			},
			{
				name:     "should change status to TERMINATED if tenant is terminated successfully",
				tenantID: operatortest.TenantIDSuccess,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_TERMINATED
				},
				expAuthState: authgrpc.AuthStatus_AUTH_STATUS_REMOVED,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				tenant := validTenant()
				tenant.ID = tt.tenantID
				tenant.Status = model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String())
				err := createTenantInDB(ctx, db, tenant)
				assert.NoError(t, err)
				t.Cleanup(func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				})

				auths, authCleanup := authWithNonTransientState(t, repo, tenant)
				t.Cleanup(func() {
					authCleanup(ctx)
				})

				// when
				_, err = subj.TerminateTenant(ctx, &tenantgrpc.TerminateTenantRequest{
					Id: tenant.ID,
				})
				assert.NoError(t, err)

				// then
				err = waitForTenantReconciliation(ctx, subj, tenant.ID, tt.expState)
				assert.NoError(t, err)
				assertAuthUpdatableStateConsistency(t, authClient, auths, tt.expAuthState)
			})
		}
	})
}

func waitForTenantReconciliation(ctx context.Context, tSubj tenantgrpc.ServiceClient, tenantID string, expState expStateFunc) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var currentTenant *tenantgrpc.Tenant
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w; tenant: %s", ctx.Err(), currentTenant)
		default:
			resp, err := tSubj.GetTenant(ctx, &tenantgrpc.GetTenantRequest{
				Id: tenantID,
			})
			if err != nil {
				return err
			}
			if expState(resp.GetTenant()) {
				return nil
			}

			currentTenant = resp.GetTenant()
		}
		time.Sleep(100 * time.Millisecond)
	}
}
