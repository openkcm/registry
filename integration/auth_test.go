//go:build integration
// +build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/integration/operatortest"
	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository/sql"
)

func TestAuth(t *testing.T) {
	// given
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer conn.Close()

	subj := authgrpc.NewServiceClient(conn)

	ctx := t.Context()
	db, err := startDB()
	require.NoError(t, err)
	repo := sql.NewRepository(db)

	operator, err := operatortest.New(ctx)
	require.NoError(t, err)
	go operator.ListenAndRespond(ctx)

	t.Run("ApplyAuth", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("tenant does not exist", func(t *testing.T) {
				// when
				resp, err := subj.ApplyAuth(ctx, &authgrpc.ApplyAuthRequest{
					TenantId: "non-existing-tenant",
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})

			t.Run("tenant is not active", func(t *testing.T) {
				// given
				inactiveTenant := validTenant()
				inactiveTenant.Status = model.TenantStatus(pb.Status_STATUS_BLOCKED.String())
				err := repo.Create(ctx, inactiveTenant)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, inactiveTenant)
					assert.NoError(t, err)
				}()

				// when
				resp, err := subj.ApplyAuth(ctx, &authgrpc.ApplyAuthRequest{
					TenantId: inactiveTenant.ID.String(),
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})

			t.Run("auth with the same external ID already exists", func(t *testing.T) {
				// given
				tenant := validTenant()
				err := repo.Create(ctx, tenant)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, tenant)
					assert.NoError(t, err)
				}()

				auth := validAuth()
				err = repo.Create(ctx, auth)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, auth)
					assert.NoError(t, err)
				}()

				// when
				resp, err := subj.ApplyAuth(ctx, &authgrpc.ApplyAuthRequest{
					ExternalId: auth.ExternalID.String(),
					TenantId:   tenant.ID.String(),
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.AlreadyExists, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})
		})

		tests := []struct {
			name       string
			externalID string
			region     string
			expStatus  model.AuthStatus
		}{
			{
				name:       "should change status to APPLYING_ERROR if region does not exist",
				externalID: "test-auth-cancel",
				region:     "non-existing-region",
				expStatus:  model.AuthStatus(authgrpc.AuthStatus_AUTH_STATUS_APPLYING_ERROR.String()),
			},
			{
				name:       "should change status to APPLYING_ERROR if operator fails to process the request",
				externalID: operatortest.AuthExternalIDFail,
				region:     operatortest.Region,
				expStatus:  model.AuthStatus(authgrpc.AuthStatus_AUTH_STATUS_APPLYING_ERROR.String()),
			},
			{
				name:       "should change status to APPLIED if operator processes the request successfully",
				externalID: operatortest.AuthExternalIDSuccess,
				region:     operatortest.Region,
				expStatus:  model.AuthStatus(authgrpc.AuthStatus_AUTH_STATUS_APPLIED.String()),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				tenant := validTenant()
				tenant.Region = model.Region(tt.region)
				err := repo.Create(ctx, tenant)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, tenant)
					assert.NoError(t, err)
				}()

				// when
				resp, err := subj.ApplyAuth(ctx, &authgrpc.ApplyAuthRequest{
					ExternalId: tt.externalID,
					TenantId:   tenant.ID.String(),
					Type:       "auth_type",
					Properties: map[string]string{
						"auth_prop": "auth_value",
					},
				})
				defer func() {
					auth := &model.Auth{
						ExternalID: model.ExternalID(tt.externalID),
					}
					_, err := repo.Delete(ctx, auth)
					assert.NoError(t, err)
					err = deleteOrbitalResources(ctx, db, tt.externalID)
					assert.NoError(t, err)
				}()

				// then
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.True(t, resp.Success)

				getResp, err := subj.GetAuth(ctx, &authgrpc.GetAuthRequest{
					ExternalId: tt.externalID,
				})
				assert.NoError(t, err)
				assert.NotNil(t, getResp)
				assert.Equal(t, tt.externalID, getResp.Auth.ExternalId)
				assert.Equal(t, tenant.ID.String(), getResp.Auth.TenantId)
				assert.Equal(t, "auth_type", getResp.Auth.Type)
				assert.Equal(t, "auth_value", getResp.Auth.Properties["auth_prop"])

				err = waitForAuthReconciliation(ctx, subj, tt.externalID, tt.expStatus)
				assert.NoError(t, err)
			})
		}

	})
}

func waitForAuthReconciliation(ctx context.Context, subj authgrpc.ServiceClient, externalID string, expectedStatus model.AuthStatus) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var currentAuth *authgrpc.Auth
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: auth: %s", ctx.Err(), currentAuth)
		default:
			resp, err := subj.GetAuth(ctx, &authgrpc.GetAuthRequest{
				ExternalId: externalID,
			})
			if err != nil {
				return err
			}
			if resp.Auth.Status.String() == string(expectedStatus) {
				return nil
			}

			currentAuth = resp.Auth
		}
		time.Sleep(100 * time.Millisecond)
	}
}
