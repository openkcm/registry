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
				// given
				auth := validAuth()

				// when
				resp, err := subj.ApplyAuth(ctx, &authgrpc.ApplyAuthRequest{
					ExternalId: auth.ExternalID,
					TenantId:   "non-existing-tenant",
					Type:       auth.Type,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})

			t.Run("tenant is not active", func(t *testing.T) {
				// given
				auth := validAuth()

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
					ExternalId: auth.ExternalID,
					TenantId:   inactiveTenant.ID,
					Type:       auth.Type,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})
		})

		t.Run("should not return error if auth with the same external ID already exists", func(t *testing.T) {
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
				ExternalId: auth.ExternalID,
				TenantId:   tenant.ID,
				Type:       auth.Type,
			})

			// then
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.True(t, resp.Success)
		})

		tests := []struct {
			name       string
			externalID string
			region     string
			expStatus  string
		}{
			{
				name:       "should change status to APPLYING_ERROR if region does not exist",
				externalID: operatortest.AuthExternalIDSuccess,
				region:     "non-existing-region",
				expStatus:  authgrpc.AuthStatus_AUTH_STATUS_APPLYING_ERROR.String(),
			},
			{
				name:       "should change status to APPLYING_ERROR if operator fails to process the request",
				externalID: operatortest.AuthExternalIDFail,
				region:     operatortest.Region,
				expStatus:  authgrpc.AuthStatus_AUTH_STATUS_APPLYING_ERROR.String(),
			},
			{
				name:       "should change status to APPLIED if operator processes the request successfully",
				externalID: operatortest.AuthExternalIDSuccess,
				region:     operatortest.Region,
				expStatus:  authgrpc.AuthStatus_AUTH_STATUS_APPLIED.String(),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				auth := validAuth()

				tenant := validTenant()
				tenant.Region = tt.region
				err := repo.Create(ctx, tenant)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, tenant)
					assert.NoError(t, err)
				}()

				// when
				resp, err := subj.ApplyAuth(ctx, &authgrpc.ApplyAuthRequest{
					ExternalId: tt.externalID,
					TenantId:   tenant.ID,
					Type:       auth.Type,
					Properties: map[string]string{
						"auth_prop": "auth_value",
					},
				})
				defer func() {
					auth := &model.Auth{
						ExternalID: tt.externalID,
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
				assert.Equal(t, tenant.ID, getResp.Auth.TenantId)
				assert.Equal(t, auth.Type, getResp.Auth.Type)
				assert.Equal(t, "auth_value", getResp.Auth.Properties["auth_prop"])

				err = waitForAuthReconciliation(ctx, subj, tt.externalID, tt.expStatus)
				assert.NoError(t, err)
			})
		}
	})

	t.Run("RemoveAuth", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("auth does not exist", func(t *testing.T) {
				// when
				resp, err := subj.RemoveAuth(ctx, &authgrpc.RemoveAuthRequest{
					ExternalId: "non-existing-auth",
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})

			t.Run("auth is not in APPLIED status", func(t *testing.T) {
				// given
				auth := validAuth()
				auth.Status = authgrpc.AuthStatus_AUTH_STATUS_APPLYING_ERROR.String()
				err := repo.Create(ctx, auth)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, auth)
					assert.NoError(t, err)
				}()

				// when
				resp, err := subj.RemoveAuth(ctx, &authgrpc.RemoveAuthRequest{
					ExternalId: auth.ExternalID,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})

			t.Run("tenant linked to auth does not exist", func(t *testing.T) {
				// given
				auth := validAuth()
				auth.TenantID = "non-existing-tenant"
				err := repo.Create(ctx, auth)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, auth)
					assert.NoError(t, err)
				}()

				// when
				resp, err := subj.RemoveAuth(ctx, &authgrpc.RemoveAuthRequest{
					ExternalId: auth.ExternalID,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})

			t.Run("tenant linked to auth is not active", func(t *testing.T) {
				// given
				tenant := validTenant()
				tenant.Status = model.TenantStatus(pb.Status_STATUS_BLOCKED.String())
				err := repo.Create(ctx, tenant)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, tenant)
					assert.NoError(t, err)
				}()

				auth := validAuth()
				auth.TenantID = tenant.ID
				err = repo.Create(ctx, auth)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, auth)
					assert.NoError(t, err)
				}()

				// when
				resp, err := subj.RemoveAuth(ctx, &authgrpc.RemoveAuthRequest{
					ExternalId: auth.ExternalID,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})
		})

		tests := []struct {
			name       string
			externalID string
			expStatus  string
		}{
			{
				name:       "should change status to REMOVING_ERROR if operator fails to process the request",
				externalID: operatortest.AuthExternalIDFail,
				expStatus:  authgrpc.AuthStatus_AUTH_STATUS_REMOVING_ERROR.String(),
			},
			{
				name:       "should change status to REMOVED if operator processes the request successfully",
				externalID: operatortest.AuthExternalIDSuccess,
				expStatus:  authgrpc.AuthStatus_AUTH_STATUS_REMOVED.String(),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				tenant := validTenant()
				err := repo.Create(ctx, tenant)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, tenant)
					assert.NoError(t, err)
				}()

				auth := validAuth()
				auth.ExternalID = tt.externalID
				auth.TenantID = tenant.ID
				err = repo.Create(ctx, auth)
				assert.NoError(t, err)
				defer func() {
					_, err := repo.Delete(ctx, auth)
					assert.NoError(t, err)
				}()

				// when
				resp, err := subj.RemoveAuth(ctx, &authgrpc.RemoveAuthRequest{
					ExternalId: tt.externalID,
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.True(t, resp.Success)

				err = waitForAuthReconciliation(ctx, subj, tt.externalID, tt.expStatus)
				assert.NoError(t, err)
			})
		}
	})
}

func waitForAuthReconciliation(ctx context.Context, subj authgrpc.ServiceClient, externalID, expStatus string) error {
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
			if resp.Auth.Status.String() == expStatus {
				return nil
			}

			currentAuth = resp.Auth
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestListAuth(t *testing.T) {
	// given
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer conn.Close()

	db, err := startDB()
	require.NoError(t, err)
	repo := sql.NewRepository(db)

	subj := authgrpc.NewServiceClient(conn)

	ctx := t.Context()

	t.Run("ListAuth", func(t *testing.T) {
		t.Run("should return an error", func(t *testing.T) {
			t.Run("if no entries exist", func(t *testing.T) {
				// when
				resp, err := subj.ListAuths(ctx, &authgrpc.ListAuthsRequest{
					TenantId: "random-tenant-id",
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})
			t.Run("if tenantID is empty", func(t *testing.T) {
				// when
				resp, err := subj.ListAuths(ctx, &authgrpc.ListAuthsRequest{
					TenantId: "",
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})
		})

		t.Run("when entries exist", func(t *testing.T) {
			// given
			tenantID1 := model.Tenant{ID: validRandID()}
			_, cleanupAuth1 := authWithNonTransientState(t, repo, &tenantID1)
			t.Cleanup(func() {
				cleanupAuth1(ctx)
			})

			tenantID2 := model.Tenant{ID: validRandID()}
			_, cleanupAuth2 := authWithNonTransientState(t, repo, &tenantID2)
			t.Cleanup(func() {
				cleanupAuth2(ctx)
			})

			t.Run("should retrieve", func(t *testing.T) {
				t.Run("records only for a given tenantID", func(t *testing.T) {
					// when
					resp, err := subj.ListAuths(ctx, &authgrpc.ListAuthsRequest{TenantId: tenantID1.ID})

					// then
					assert.NoError(t, err)
					assert.Len(t, resp.GetAuth(), len(nonTransientAuthStatus()))
					for _, auth := range resp.GetAuth() {
						assert.Equal(t, tenantID1.ID, auth.GetTenantId())
					}
				})

				t.Run("records and a next page token when a limit is specified", func(t *testing.T) {
					// when
					resp, err := subj.ListAuths(ctx, &authgrpc.ListAuthsRequest{
						TenantId: tenantID2.ID,
						Limit:    1,
					})

					// then
					assert.NoError(t, err)
					assert.Len(t, resp.GetAuth(), 1)
					for _, auth := range resp.GetAuth() {
						assert.Equal(t, tenantID2.ID, auth.GetTenantId())
					}
					assert.NotEmpty(t, resp.GetNextPageToken())
				})

				t.Run("records without a next page token if limit is greater than number of auths", func(t *testing.T) {
					// when
					resp, err := subj.ListAuths(ctx, &authgrpc.ListAuthsRequest{
						TenantId: tenantID2.ID,
						Limit:    100,
					})

					// then
					assert.NoError(t, err)
					assert.Len(t, resp.GetAuth(), len(nonTransientAuthStatus()))
					for _, auth := range resp.GetAuth() {
						assert.Equal(t, tenantID2.ID, auth.GetTenantId())
					}
					assert.Empty(t, resp.GetNextPageToken())
				})

				t.Run("next page records from the next page token", func(t *testing.T) {
					// given
					resp1, err := subj.ListAuths(ctx, &authgrpc.ListAuthsRequest{
						TenantId: tenantID2.ID,
						Limit:    1,
					})

					assert.NoError(t, err)
					assert.NotEmpty(t, resp1.GetNextPageToken())

					// when
					resp2, err := subj.ListAuths(ctx, &authgrpc.ListAuthsRequest{
						TenantId:      tenantID2.ID,
						Limit:         100,
						NextPageToken: resp1.GetNextPageToken(),
					})

					// then
					assert.NoError(t, err)
					assert.NotNil(t, resp2)
					assert.Len(t, resp2.GetAuth(), len(nonTransientAuthStatus())-1)
					for _, auth := range resp2.GetAuth() {
						//  making sure that there i no duplicates
						assert.NotEqual(t, resp1.GetAuth()[0].Status, auth.Status)
					}
				})
			})
		})
	})
}

func TestAuthValidation(t *testing.T) {
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer conn.Close()

	subj := authgrpc.NewServiceClient(conn)

	t.Run("ApplyAuth should return error for invalid requests", func(t *testing.T) {
		tests := []struct {
			name       string
			request    *authgrpc.ApplyAuthRequest
			expErrCode codes.Code
		}{
			{
				name: "should return error for failed model validation (Auth.ExternalId)",
				request: &authgrpc.ApplyAuthRequest{
					TenantId: "tenant-id",
					Type:     "oidc",
				},
				expErrCode: codes.InvalidArgument,
			},
			{
				name: "should return error for failed configured validation with pre-existing validation ID (Auth.Type)",
				request: &authgrpc.ApplyAuthRequest{
					ExternalId: "external-id",
					TenantId:   "tenant-id",
					Type:       "saml",
				},
				expErrCode: codes.InvalidArgument,
			},
			{
				name: "should return error for failed configured validation without pre-existing validation ID (Auth.Properties)",
				request: &authgrpc.ApplyAuthRequest{
					ExternalId: "external-id",
					TenantId:   "tenant-id",
					Type:       "oidc",
					Properties: map[string]string{
						"issuer": "",
					},
				},
				expErrCode: codes.InvalidArgument,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// when
				resp, err := subj.ApplyAuth(t.Context(), tt.request)

				// then
				assert.Nil(t, resp)
				assert.Error(t, err)
				assert.Equal(t, tt.expErrCode, status.Code(err), err.Error())
			})
		}
	})

	t.Run("GetAuth should return error for invalid request", func(t *testing.T) {
		resp, err := subj.GetAuth(t.Context(), &authgrpc.GetAuthRequest{
			ExternalId: "",
		})

		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
	})

	t.Run("RemoveAuth should return error for invalid request", func(t *testing.T) {
		resp, err := subj.RemoveAuth(t.Context(), &authgrpc.RemoveAuthRequest{
			ExternalId: "",
		})

		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
	})
}
