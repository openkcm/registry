//go:build integration
// +build integration

package integration_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/integration/operatortest"
	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository/sql"
	"github.com/openkcm/registry/internal/service"
)

type expStateFunc func(*tenantgrpc.Tenant) bool

var ErrTenantIDEmpty = status.Error(codes.InvalidArgument, "invalid ID: validation failed for Tenant.ID: value is empty")

func TestTenantReconciliation(t *testing.T) {
	// given
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer conn.Close()

	tSubj := tenantgrpc.NewServiceClient(conn)

	ctx := t.Context()
	db, err := startDB()
	require.NoError(t, err)

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
				_, err := tSubj.RegisterTenant(ctx, req)
				assert.NoError(t, err)
				defer func() {
					err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req.GetId()})
					assert.NoError(t, err)
				}()

				// then
				err = waitForTenantReconciliation(ctx, tSubj, req.GetId(), tt.expState)
				assert.NoError(t, err)
			})
		}
	})

	t.Run("BlockTenant", func(t *testing.T) {
		tests := []struct {
			name     string
			tenantID string
			expState expStateFunc
		}{
			{
				name:     "should change status to BLOCKING_ERROR if operator fails to process the request",
				tenantID: operatortest.TenantIDFail,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_BLOCKING_ERROR
				},
			},
			{
				name:     "should change status to BLOCKED if tenant is blocked successfully",
				tenantID: operatortest.TenantIDSuccess,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_BLOCKED
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				tenant := validTenant()
				tenant.ID = tt.tenantID
				tenant.Region = operatortest.Region
				err := createTenantInDB(ctx, db, tenant)
				assert.NoError(t, err)
				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				_, err = tSubj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
					Id: tenant.ID,
				})
				assert.NoError(t, err)

				// then
				err = waitForTenantReconciliation(ctx, tSubj, tenant.ID, tt.expState)
				assert.NoError(t, err)
			})
		}
	})

	t.Run("UnblockTenant", func(t *testing.T) {
		tests := []struct {
			name     string
			tenantID string
			expState expStateFunc
		}{
			{
				name:     "should change status to UNBLOCKING_ERROR if operator fails to process the request",
				tenantID: operatortest.TenantIDFail,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_UNBLOCKING_ERROR
				},
			},
			{
				name:     "should change status to ACTIVE if tenant is unblocked successfully",
				tenantID: operatortest.TenantIDSuccess,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_ACTIVE
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				tenant := validTenant()
				tenant.ID = tt.tenantID
				tenant.Region = operatortest.Region
				tenant.Status = model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String())
				err := createTenantInDB(ctx, db, tenant)
				assert.NoError(t, err)
				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				_, err = tSubj.UnblockTenant(ctx, &tenantgrpc.UnblockTenantRequest{
					Id: tenant.ID,
				})
				assert.NoError(t, err)

				// then
				err = waitForTenantReconciliation(ctx, tSubj, tenant.ID, tt.expState)
				assert.NoError(t, err)
			})
		}
	})

	t.Run("TerminateTenant", func(t *testing.T) {
		tests := []struct {
			name     string
			tenantID string
			expState expStateFunc
		}{
			{
				name:     "should change status to TERMINATION_ERROR if operator fails to process the request",
				tenantID: operatortest.TenantIDFail,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_TERMINATION_ERROR
				},
			},
			{
				name:     "should change status to TERMINATED if tenant is terminated successfully",
				tenantID: operatortest.TenantIDSuccess,
				expState: func(t *tenantgrpc.Tenant) bool {
					return t.GetStatus() == tenantgrpc.Status_STATUS_TERMINATED
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				tenant := validTenant()
				tenant.ID = tt.tenantID
				tenant.Status = model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String())
				tenant.Region = operatortest.Region
				err := createTenantInDB(ctx, db, tenant)
				assert.NoError(t, err)
				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				_, err = tSubj.TerminateTenant(ctx, &tenantgrpc.TerminateTenantRequest{
					Id: tenant.ID,
				})
				assert.NoError(t, err)

				// then
				err = waitForTenantReconciliation(ctx, tSubj, tenant.ID, tt.expState)
				assert.NoError(t, err)
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

func TestTenantValidation(t *testing.T) {
	// given
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer conn.Close()

	tSubj := tenantgrpc.NewServiceClient(conn)

	ctx := t.Context()
	db, err := startDB()
	require.NoError(t, err)

	t.Run("RegisterTenant", func(t *testing.T) {
		t.Run("should return an error if", func(t *testing.T) {
			// given
			tts := map[string]func(t *tenantgrpc.RegisterTenantRequest) *tenantgrpc.RegisterTenantRequest{
				"ownerID is empty": func(req *tenantgrpc.RegisterTenantRequest) *tenantgrpc.RegisterTenantRequest {
					req.OwnerId = ""
					return req
				},
				"request is empty": func(_ *tenantgrpc.RegisterTenantRequest) *tenantgrpc.RegisterTenantRequest {
					return &tenantgrpc.RegisterTenantRequest{}
				},
				"ownerType is empty": func(req *tenantgrpc.RegisterTenantRequest) *tenantgrpc.RegisterTenantRequest {
					req.OwnerType = ""
					return req
				},
				"ownerType is not present in allowed list": func(req *tenantgrpc.RegisterTenantRequest) *tenantgrpc.RegisterTenantRequest {
					req.OwnerType = "unknown"
					return req
				},
				"role is unspecified": func(req *tenantgrpc.RegisterTenantRequest) *tenantgrpc.RegisterTenantRequest {
					req.Role = tenantgrpc.Role_ROLE_UNSPECIFIED
					return req
				},
				"region is empty": func(req *tenantgrpc.RegisterTenantRequest) *tenantgrpc.RegisterTenantRequest {
					req.Region = ""
					return req
				},
				"region is not present in allowed list": func(req *tenantgrpc.RegisterTenantRequest) *tenantgrpc.RegisterTenantRequest {
					req.Region = "unknown"
					return req
				},
			}

			for reason, requestTransformer := range tts {
				t.Run(reason, func(t *testing.T) {
					validRequest := &tenantgrpc.RegisterTenantRequest{
						Name:      "SuccessFactor",
						Id:        validRandID(),
						Region:    "region",
						OwnerId:   "owner-id-1",
						OwnerType: tenantOwnerType1,
						Role:      tenantgrpc.Role_ROLE_TEST,
						Labels: map[string]string{
							"key1": "value1",
							"key2": "value2",
						},
					}

					// when
					resp, err := tSubj.RegisterTenant(ctx, requestTransformer(validRequest))

					// then
					assert.Error(t, err)
					assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
					assert.Nil(t, resp)
				})
			}

			t.Run("ID is found to already exist", func(t *testing.T) {
				// given
				req := &tenantgrpc.RegisterTenantRequest{
					Name:      "SuccessFactor",
					Id:        validRandID(),
					Region:    "region",
					OwnerId:   "owner-id-123",
					OwnerType: tenantOwnerType1,
					Role:      tenantgrpc.Role_ROLE_TEST,
				}

				// when
				firstResp, err := tSubj.RegisterTenant(ctx, req)
				defer func() {
					err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req.GetId()})
					assert.NoError(t, err)
				}()

				// then
				assert.NoError(t, err)
				assert.NotNil(t, firstResp)

				// when
				secondResp, err := tSubj.RegisterTenant(ctx, req)

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Nil(t, secondResp)
			})
		})

		t.Run("should succeed", func(t *testing.T) {
			// given
			req := &tenantgrpc.RegisterTenantRequest{
				Name:      "SuccessFactor",
				Id:        validRandID(),
				Region:    "region",
				OwnerId:   "owner-id-123",
				OwnerType: tenantOwnerType1,
				Role:      tenantgrpc.Role_ROLE_TEST,
				Labels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			}

			// when
			resp, err := tSubj.RegisterTenant(ctx, req)
			defer func() {
				err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req.GetId()})
				assert.NoError(t, err)
			}()

			// then
			assert.Equal(t, req.Id, resp.GetId())
			assert.NoError(t, err)
		})
	})

	t.Run("ListTenants", func(t *testing.T) {
		t.Run("should return error given no entries exist if", func(t *testing.T) {
			// given
			tests := []struct {
				name    string
				request *tenantgrpc.ListTenantsRequest
			}{
				{
					name:    "no filters are provided",
					request: &tenantgrpc.ListTenantsRequest{},
				},
				{
					name: "only id filter is provided",
					request: &tenantgrpc.ListTenantsRequest{
						Id: validRandID(),
					},
				},
				{
					name: "only name filter is provided",
					request: &tenantgrpc.ListTenantsRequest{
						Name: "some-name",
					},
				},
				{
					name: "only Region filter is provided",
					request: &tenantgrpc.ListTenantsRequest{
						Region: "CMK_REGION_EU",
					},
				},
				{
					name: "only OwnerID filter is provided",
					request: &tenantgrpc.ListTenantsRequest{
						OwnerId: "some-owner-id",
					},
				},
				{
					name: "only OwnerType filter is provided",
					request: &tenantgrpc.ListTenantsRequest{
						OwnerType: tenantOwnerType1,
					},
				},
				{
					name: "all filter are provided",
					request: &tenantgrpc.ListTenantsRequest{
						Id:        validRandID(),
						Name:      "some-name",
						Region:    "region",
						OwnerId:   "owner-id-123",
						OwnerType: tenantOwnerType1,
					},
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// when
					result, err := tSubj.ListTenants(ctx, tt.request)

					// then
					assert.Error(t, err)
					assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
					assert.Nil(t, result)
				})
			}
		})

		t.Run("given entries exist", func(t *testing.T) {
			// given
			req1 := &tenantgrpc.RegisterTenantRequest{
				Name:      "SuccessFactor",
				Id:        validRandID(),
				Region:    "region",
				OwnerId:   "owner-id-123",
				OwnerType: tenantOwnerType1,
				Role:      tenantgrpc.Role_ROLE_TEST,
				Labels: map[string]string{
					"key11": "value11",
					"key12": "value12",
				},
			}
			resp1, err := tSubj.RegisterTenant(ctx, req1)
			assert.NoError(t, err)
			defer func() {
				err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req1.GetId()})
				assert.NoError(t, err)
			}()

			time.Sleep(time.Millisecond)

			req2 := &tenantgrpc.RegisterTenantRequest{
				Name:      "Ariba",
				Id:        validRandID(),
				Region:    "region-2",
				OwnerId:   "owner-id-1",
				OwnerType: "ownerType2",
				Role:      tenantgrpc.Role_ROLE_TEST,
				Labels: map[string]string{
					"key21": "value21",
					"key22": "value22",
				},
			}
			resp2, err := tSubj.RegisterTenant(ctx, req2)
			assert.NoError(t, err)
			defer func() {
				err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req2.GetId()})
				assert.NoError(t, err)
			}()

			t.Run("should return all tenants if no filter is applied", func(t *testing.T) {
				// when
				resp, err := tSubj.ListTenants(ctx, &tenantgrpc.ListTenantsRequest{})

				// then
				assert.NoError(t, err)
				assert.Len(t, resp.GetTenants(), 2)
				assert.Empty(t, resp.GetNextPageToken())
				assertEqualValues(t, req1, resp.Tenants[1])
				assert.Equal(t, tenantgrpc.Status_STATUS_PROVISIONING, resp.Tenants[1].Status)
				assert.Empty(t, resp.Tenants[1].GetUserGroups())
				assertEqualValues(t, req2, resp.Tenants[0])
				assert.Equal(t, tenantgrpc.Status_STATUS_PROVISIONING, resp.Tenants[0].Status)
				assert.Empty(t, resp.Tenants[0].GetUserGroups())
			})

			t.Run("should return tenant filtered by", func(t *testing.T) {
				// given
				tests := []struct {
					name             string
					request          *tenantgrpc.ListTenantsRequest
					expectedTenantID string
				}{
					{
						name: "ID",
						request: &tenantgrpc.ListTenantsRequest{
							Id: resp1.GetId(),
						},
						expectedTenantID: resp1.GetId(),
					},
					{
						name: "Name",
						request: &tenantgrpc.ListTenantsRequest{
							Name: req2.Name,
						},
						expectedTenantID: resp2.GetId(),
					},
					{
						name: "Region",
						request: &tenantgrpc.ListTenantsRequest{
							Region: req2.Region,
						},
						expectedTenantID: resp2.GetId(),
					},
					{
						name: "OwnerID",
						request: &tenantgrpc.ListTenantsRequest{
							OwnerId: req1.OwnerId,
						},
						expectedTenantID: resp1.GetId(),
					},
					{
						name: "OwnerType",
						request: &tenantgrpc.ListTenantsRequest{
							OwnerType: req2.OwnerType,
						},
						expectedTenantID: resp2.GetId(),
					},
				}

				for _, test := range tests {
					t.Run(test.name, func(t *testing.T) {
						// when
						list, err := tSubj.ListTenants(ctx, test.request)

						// then
						assert.NoError(t, err)
						assert.Len(t, list.GetTenants(), 1)
						assert.Equal(t, test.expectedTenantID, list.GetTenants()[0].Id)
					})
				}
			})
		})
	})

	t.Run("BlockTenant", func(t *testing.T) {
		t.Run("should return an error if", func(t *testing.T) {
			t.Run("tenant cannot be found", func(t *testing.T) {
				// when
				resp, err := tSubj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
					Id: validRandID(),
				})

				// then
				assert.Nil(t, resp)
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})

			t.Run("tenant is in a state that prevents blocking", func(t *testing.T) {
				// given
				state := model.TenantStatus(tenantgrpc.Status_STATUS_PROVISIONING.String())
				tenant, err := persistTenant(ctx, db, validRandID(), state, time.Now())
				assert.NoError(t, err)
				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				resp, err := tSubj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
					Id: tenant.ID,
				})

				// then
				assert.Nil(t, resp)
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
			})
		})

		t.Run("should succeed if tenant is active", func(t *testing.T) {
			// given
			state := model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String())
			tenant, err := persistTenant(ctx, db, validRandID(), state, time.Now())
			assert.NoError(t, err)

			defer func() {
				err = deleteTenantFromDB(ctx, db, tenant)
				assert.NoError(t, err)
			}()

			expStatus := tenantgrpc.Status_STATUS_BLOCKING

			// when
			utResp, err := tSubj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
				Id: tenant.ID,
			})

			// then
			assert.NoError(t, err)
			assert.NotNil(t, utResp)
			ltResp, err := listTenants(ctx, tSubj)
			assert.NoError(t, err)
			assert.Len(t, ltResp.Tenants, 1)
			assert.Equal(t, expStatus, ltResp.Tenants[0].Status)
		})
	})

	t.Run("UnblockTenant", func(t *testing.T) {
		t.Run("should return an error if", func(t *testing.T) {
			t.Run("tenant cannot be found", func(t *testing.T) {
				// when
				resp, err := tSubj.UnblockTenant(ctx, &tenantgrpc.UnblockTenantRequest{
					Id: validRandID(),
				})

				// then
				assert.Nil(t, resp)
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})

			t.Run("tenant is in a state that prevents unblocking", func(t *testing.T) {
				// given
				state := model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATED.String())
				tenant, err := persistTenant(ctx, db, validRandID(), state, time.Now())
				assert.NoError(t, err)
				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				resp, err := tSubj.UnblockTenant(ctx, &tenantgrpc.UnblockTenantRequest{
					Id: tenant.ID,
				})

				// then
				assert.Nil(t, resp)
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
			})
		})

		t.Run("should succeed if tenant is blocked", func(t *testing.T) {
			// given
			state := model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String())
			tenant, err := persistTenant(ctx, db, validRandID(), state, time.Now())
			assert.NoError(t, err)

			defer func() {
				err = deleteTenantFromDB(ctx, db, tenant)
				assert.NoError(t, err)
			}()

			expStatus := tenantgrpc.Status_STATUS_UNBLOCKING

			// when
			utResp, err := tSubj.UnblockTenant(ctx, &tenantgrpc.UnblockTenantRequest{
				Id: tenant.ID,
			})

			// then
			assert.NoError(t, err)
			assert.NotNil(t, utResp)
			ltResp, err := listTenants(ctx, tSubj)
			assert.NoError(t, err)
			assert.Len(t, ltResp.Tenants, 1)
			assert.Equal(t, expStatus, ltResp.Tenants[0].Status)
		})
	})

	t.Run("TerminateTenant", func(t *testing.T) {
		t.Run("should return an error if", func(t *testing.T) {
			t.Run("tenant cannot be found", func(t *testing.T) {
				// when
				resp, err := tSubj.TerminateTenant(ctx, &tenantgrpc.TerminateTenantRequest{
					Id: validRandID(),
				})

				// then
				assert.Nil(t, resp)
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})

			t.Run("tenant is in a state that prevents terminating", func(t *testing.T) {
				// given
				state := model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATING.String())
				tenant, err := persistTenant(ctx, db, validRandID(), state, time.Now())
				assert.NoError(t, err)
				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				resp, err := tSubj.TerminateTenant(ctx, &tenantgrpc.TerminateTenantRequest{
					Id: tenant.ID,
				})

				// then
				assert.Nil(t, resp)
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
			})
		})

		t.Run("should succeed if tenant is blocked", func(t *testing.T) {
			// given
			state := model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String())
			tenant, err := persistTenant(ctx, db, validRandID(), state, time.Now())
			assert.NoError(t, err)

			defer func() {
				err = deleteTenantFromDB(ctx, db, tenant)
				assert.NoError(t, err)
			}()

			expStatus := tenantgrpc.Status_STATUS_TERMINATING

			// when
			utResp, err := tSubj.TerminateTenant(ctx, &tenantgrpc.TerminateTenantRequest{
				Id: tenant.ID,
			})

			// then
			assert.NoError(t, err)
			assert.NotNil(t, utResp)
			ltResp, err := listTenants(ctx, tSubj)
			assert.NoError(t, err)
			assert.Len(t, ltResp.Tenants, 1)
			assert.Equal(t, expStatus, ltResp.Tenants[0].Status)
		})
	})

	t.Run("SetTenantLabels", func(t *testing.T) {
		t.Run("should succeed if", func(t *testing.T) {
			t.Run("request is valid", func(t *testing.T) {
				// given
				// For creating a tenant with a specific status, direct database access is needed
				tenant, err := persistTenant(ctx, db, validRandID(),
					model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()), time.Now())
				assert.NoError(t, err)

				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				res, err := tSubj.SetTenantLabels(ctx, &tenantgrpc.SetTenantLabelsRequest{
					Id: tenant.ID,
					Labels: map[string]string{
						"key1": "value12",
						"key3": "value3",
					},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
				actTenants, err := listTenants(ctx, tSubj)
				assert.NoError(t, err)
				assert.Len(t, actTenants.GetTenants(), 1)
				assert.Len(t, actTenants.GetTenants()[0].GetLabels(), 3)
				assert.Equal(t, "value12", actTenants.GetTenants()[0].GetLabels()["key1"])
				assert.Equal(t, "value2", actTenants.GetTenants()[0].GetLabels()["key2"])
				assert.Equal(t, "value3", actTenants.GetTenants()[0].GetLabels()["key3"])
			})
		})

		t.Run("should return error if", func(t *testing.T) {
			t.Run("ID is empty", func(t *testing.T) {
				// when
				res, err := tSubj.SetTenantLabels(ctx, &tenantgrpc.SetTenantLabelsRequest{
					Id: "",
					Labels: map[string]string{
						"key1": "value1",
					},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, ErrTenantIDEmpty, err)
				assert.Nil(t, res)
			})
			t.Run("labels are empty", func(t *testing.T) {
				// when
				res, err := tSubj.SetTenantLabels(ctx, &tenantgrpc.SetTenantLabelsRequest{
					Id: "ID1",
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrMissingLabels, err)
				assert.Nil(t, res)
			})
			t.Run("labels keys are empty", func(t *testing.T) {
				// when
				res, err := tSubj.SetTenantLabels(ctx, &tenantgrpc.SetTenantLabelsRequest{
					Id: "ID1",
					Labels: map[string]string{
						"": "value1",
					},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, model.ErrLabelsIncludeEmptyString, err)
				assert.Nil(t, res)
			})
			t.Run("labels values are empty", func(t *testing.T) {
				// when
				res, err := tSubj.SetTenantLabels(ctx, &tenantgrpc.SetTenantLabelsRequest{
					Id: "ID1",
					Labels: map[string]string{
						"key1": "",
					},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, model.ErrLabelsIncludeEmptyString, err)
				assert.Nil(t, res)
			})
			t.Run("tenant to update is not present in the database", func(t *testing.T) {
				// when
				id := uuid.NewString()
				res, err := tSubj.SetTenantLabels(ctx, &tenantgrpc.SetTenantLabelsRequest{
					Id: id,
					Labels: map[string]string{
						"key1": "value1",
					},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrTenantNotFound, err)
				assert.Nil(t, res)
			})
		})
	})

	t.Run("RemoveTenantLabels", func(t *testing.T) {
		t.Run("should succeed if", func(t *testing.T) {
			t.Run("request is valid", func(t *testing.T) {
				// given
				// For creating a tenant with a specific status, direct database access is needed
				tenant, err := persistTenant(ctx, db, validRandID(),
					model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()), time.Now())
				assert.NoError(t, err)

				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				res, err := tSubj.RemoveTenantLabels(ctx, &tenantgrpc.RemoveTenantLabelsRequest{
					Id:        tenant.ID,
					LabelKeys: []string{"key1"},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
				actTenants, err := listTenants(ctx, tSubj)
				assert.NoError(t, err)
				assert.Len(t, actTenants.GetTenants(), 1)
				assert.Len(t, actTenants.GetTenants()[0].GetLabels(), 1)
				assert.Equal(t, "value2", actTenants.GetTenants()[0].GetLabels()["key2"])
				_, ok := actTenants.GetTenants()[0].GetLabels()["key1"]
				assert.False(t, ok, "key1 should have been removed")
			})
			t.Run("label does not exist", func(t *testing.T) {
				// given
				// For creating a tenant with a specific status, direct database access is needed
				tenant, err := persistTenant(ctx, db, validRandID(),
					model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()), time.Now())
				assert.NoError(t, err)

				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				res, err := tSubj.RemoveTenantLabels(ctx, &tenantgrpc.RemoveTenantLabelsRequest{
					Id:        tenant.ID,
					LabelKeys: []string{"key3"},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
				actTenants, err := listTenants(ctx, tSubj)
				assert.NoError(t, err)
				assert.Len(t, actTenants.GetTenants(), 1)
				assert.Len(t, actTenants.GetTenants()[0].GetLabels(), 2)
			})
		})

		t.Run("should return error if", func(t *testing.T) {
			t.Run("external ID is empty", func(t *testing.T) {
				// when
				res, err := tSubj.RemoveTenantLabels(ctx, &tenantgrpc.RemoveTenantLabelsRequest{
					Id:        "",
					LabelKeys: []string{"key1"},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, ErrTenantIDEmpty, err)
				assert.Nil(t, res)
			})
			t.Run("labels keys are empty", func(t *testing.T) {
				// when
				res, err := tSubj.RemoveTenantLabels(ctx, &tenantgrpc.RemoveTenantLabelsRequest{
					Id: "ID1",
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrMissingLabelKeys, err)
				assert.Nil(t, res)
			})
			t.Run("labels keys have empty value", func(t *testing.T) {
				// when
				res, err := tSubj.RemoveTenantLabels(ctx, &tenantgrpc.RemoveTenantLabelsRequest{
					Id:        "ID1",
					LabelKeys: []string{""},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrEmptyLabelKeys, err)
				assert.Nil(t, res)
			})
			t.Run("tenant to update is not present in the database", func(t *testing.T) {
				// when
				id := uuid.NewString()
				res, err := tSubj.RemoveTenantLabels(ctx, &tenantgrpc.RemoveTenantLabelsRequest{
					Id:        id,
					LabelKeys: []string{"key1"},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrTenantNotFound, err)
				assert.Nil(t, res)
			})
		})
	})

	t.Run("GetTenant", func(t *testing.T) {
		t.Run("should return error if tenant with the given ID does not exist", func(t *testing.T) {
			resp, err := tSubj.GetTenant(ctx, &tenantgrpc.GetTenantRequest{
				Id: validRandID(),
			})

			assert.Nil(t, resp)
			assert.Error(t, err)
			assert.ErrorIs(t, service.ErrTenantNotFound, err)
		})

		t.Run("should fetch the tenant if tenant with the given ID exists", func(t *testing.T) {
			req := validRegisterTenantReq()
			_, err := tSubj.RegisterTenant(ctx, req)
			assert.NoError(t, err)
			defer func() {
				err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req.GetId()})
				assert.NoError(t, err)
			}()

			resp, err := tSubj.GetTenant(ctx, &tenantgrpc.GetTenantRequest{
				Id: req.GetId(),
			})

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, req.GetId(), resp.GetTenant().GetId())
		})

		t.Run("should return error if requested with empty ID", func(t *testing.T) {
			resp, err := tSubj.GetTenant(ctx, &tenantgrpc.GetTenantRequest{
				Id: "",
			})

			assert.Nil(t, resp)
			assert.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
		})
	})
}

func TestSetTenantUserGroups(t *testing.T) {
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer conn.Close()

	tSubj := tenantgrpc.NewServiceClient(conn)

	db, err := startDB()
	require.NoError(t, err)

	ctx := t.Context()

	t.Run("SetTenantUserGroups", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			tts := []struct {
				name       string
				tenantID   string
				userGroups model.UserGroups
				expCode    codes.Code
			}{
				{
					name:       "UserGroups is nil",
					tenantID:   "some-tenant-id",
					userGroups: nil,
					expCode:    codes.InvalidArgument,
				},
				{
					name:       "UserGroups is empty",
					tenantID:   "some-tenant-id",
					userGroups: []string{},
					expCode:    codes.InvalidArgument,
				},
				{
					name:       "UserGroups has a empty string",
					tenantID:   "some-tenant-id",
					userGroups: []string{"admin", ""},
					expCode:    codes.InvalidArgument,
				},
				{
					name:       "UserGroups has a blank string",
					tenantID:   "some-tenant-id",
					userGroups: []string{"admin", " "},
					expCode:    codes.InvalidArgument,
				},
				{
					name:       "tenant is not present",
					tenantID:   "some-tenant-id",
					userGroups: []string{"admin", "audit"},
					expCode:    codes.NotFound,
				},
				{
					name:       "tenant is empty",
					tenantID:   "",
					userGroups: []string{"admin", "audit"},
					expCode:    codes.InvalidArgument,
				},
			}
			for _, tt := range tts {
				t.Run(tt.name, func(t *testing.T) {
					// given

					// when
					res, err := tSubj.SetTenantUserGroups(ctx, &tenantgrpc.SetTenantUserGroupsRequest{
						Id:         tt.tenantID,
						UserGroups: tt.userGroups,
					})

					// then
					assert.Error(t, err)
					assert.Equal(t, tt.expCode, status.Code(err), err.Error())
					assert.Nil(t, res)
				})
			}
		})
		t.Run("should succeed if", func(t *testing.T) {
			t.Run("request is valid", func(t *testing.T) {
				// given
				// For creating a tenant
				tenant, err := persistTenant(ctx, db, validRandID(),
					model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()), time.Now())
				assert.NoError(t, err)

				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				res, err := tSubj.SetTenantUserGroups(ctx, &tenantgrpc.SetTenantUserGroupsRequest{
					Id: tenant.ID,
					UserGroups: []string{
						"admin",
						"audit",
					},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
				actTenants, err := listTenants(ctx, tSubj)
				assert.NoError(t, err)
				assert.Len(t, actTenants.GetTenants(), 1)
				assert.Equal(t, []string{"admin", "audit"}, actTenants.GetTenants()[0].GetUserGroups())
			})

			t.Run("if UserGroups are updated twice", func(t *testing.T) {
				// given
				// For creating a tenant
				tenant, err := persistTenant(ctx, db, validRandID(),
					model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()), time.Now())
				assert.NoError(t, err)

				defer func() {
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				res, err := tSubj.SetTenantUserGroups(ctx, &tenantgrpc.SetTenantUserGroupsRequest{
					Id: tenant.ID,
					UserGroups: []string{
						"admin",
						"audit",
					},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())

				// when
				res, err = tSubj.SetTenantUserGroups(ctx, &tenantgrpc.SetTenantUserGroupsRequest{
					Id: tenant.ID,
					UserGroups: []string{
						"admin 1",
						"audit 2",
					},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
				actTenants, err := listTenants(ctx, tSubj)
				assert.NoError(t, err)
				assert.Len(t, actTenants.GetTenants(), 1)
				assert.Equal(t, []string{"admin 1", "audit 2"}, actTenants.GetTenants()[0].GetUserGroups())
			})
		})
	})
}

func TestListTenantsPagination(t *testing.T) {
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		assert.NoError(t, err)
	}(conn)
	subj := tenantgrpc.NewServiceClient(conn)

	ctx := t.Context()
	db, err := startDB()
	require.NoError(t, err)

	t.Run("ListTenantsPagination", func(t *testing.T) {
		t.Run("given tenants with different creation timestamps", func(t *testing.T) {
			// given
			tenantRequest1 := validRegisterTenantReq()
			tenantRequest1.Name = "t1"
			_, err := subj.RegisterTenant(ctx, tenantRequest1)
			assert.NoError(t, err)
			defer func() {
				err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: tenantRequest1.GetId()})
				assert.NoError(t, err)
			}()
			tenantRequest2 := validRegisterTenantReq()
			tenantRequest2.Name = "t2"
			_, err = subj.RegisterTenant(ctx, tenantRequest2)
			assert.NoError(t, err)
			defer func() {
				err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: tenantRequest2.GetId()})
				assert.NoError(t, err)
			}()

			t.Run("should return next page token with applied limit", func(t *testing.T) {
				req := &tenantgrpc.ListTenantsRequest{
					Limit: 1,
				}
				res, _ := subj.ListTenants(ctx, req)
				assert.NoError(t, err)
				assert.NotEmpty(t, res.NextPageToken)
				assert.Len(t, res.Tenants, 1)
				assert.Equal(t, "t2", res.Tenants[0].Name)
			})

			t.Run("should not return next page token if limit is greater than number of tenants", func(t *testing.T) {
				req := &tenantgrpc.ListTenantsRequest{
					Limit: 3,
				}
				res, _ := subj.ListTenants(ctx, req)
				assert.NoError(t, err)
				assert.Empty(t, res.NextPageToken)
				assert.Len(t, res.Tenants, 2)
			})

			t.Run("page token point to next page", func(t *testing.T) {
				// first page
				req := &tenantgrpc.ListTenantsRequest{
					Limit: 1,
				}
				res, err := subj.ListTenants(ctx, req)
				assert.NoError(t, err)
				assert.NotEmpty(t, res.NextPageToken)
				assert.Len(t, res.Tenants, 1)
				assert.Equal(t, "t2", res.Tenants[0].Name)

				// second page
				req = &tenantgrpc.ListTenantsRequest{
					Limit:     1,
					PageToken: res.NextPageToken,
				}
				res, err = subj.ListTenants(ctx, req)
				assert.NoError(t, err)
				assert.Len(t, res.Tenants, 1)
				assert.Equal(t, "t1", res.Tenants[0].Name)
			})
		})
		t.Run("given tenants with same creation timestamp", func(t *testing.T) {
			// for reliably creating tenants with the same created_at timestamp
			// direct database access is needed
			require.NoError(t, err)
			tenants := make([]model.Tenant, 3)
			now := time.Now()
			for i := range tenants {
				tenant, err := persistTenant(ctx, db, validRandID(),
					model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()), now)
				assert.NoError(t, err)
				tenants[i] = *tenant
			}
			defer func() {
				for _, tenant := range tenants {
					err = deleteTenantFromDB(ctx, db, &tenant)
					assert.NoError(t, err)
				}
			}()

			// mirrors the order in which the tenants are queried by ID besides the created_at timestamp
			sort.Slice(tenants, func(i, j int) bool {
				return tenants[i].ID > tenants[j].ID
			})

			t.Run("should avoid duplicates across pages", func(t *testing.T) {
				// first page
				res1, err := subj.ListTenants(ctx, &tenantgrpc.ListTenantsRequest{
					Limit: 2,
				})
				assert.NoError(t, err)
				assert.Len(t, res1.Tenants, 2)

				// second page
				res2, err := subj.ListTenants(ctx, &tenantgrpc.ListTenantsRequest{
					PageToken: res1.NextPageToken,
				})
				assert.NoError(t, err)
				assert.Len(t, res2.Tenants, 1)
				assert.Equal(t, tenants[2].ID, res2.Tenants[0].Id)

				assert.Equal(t, len(tenants), len(res1.Tenants)+len(res2.Tenants))

				for _, prevTenants := range res1.GetTenants() {
					assert.NotEqual(t, prevTenants.Id, res2.GetTenants()[0].Id)
				}
			})
		})
	})
}

func listTenants(ctx context.Context, subj tenantgrpc.ServiceClient) (*tenantgrpc.ListTenantsResponse, error) {
	req := &tenantgrpc.ListTenantsRequest{}
	return subj.ListTenants(ctx, req)
}

func persistTenant(ctx context.Context, db *gorm.DB, id string, status model.TenantStatus, createdAt time.Time) (*model.Tenant, error) {
	repo := sql.NewRepository(db)
	tenant := &model.Tenant{
		Name:      "t1",
		ID:        id,
		Region:    "region",
		OwnerID:   "owner-id-123",
		OwnerType: tenantOwnerType1,
		Status:    status,
		Role:      "ROLE_LIVE",
		Labels: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		CreatedAt: createdAt,
	}
	err := repo.Create(ctx, tenant)
	return tenant, err
}

func assertEqualValues(t *testing.T, req1 *tenantgrpc.RegisterTenantRequest, tenant *tenantgrpc.Tenant) {
	t.Helper()
	assert.Equal(t, req1.Id, tenant.Id)
	assert.Equal(t, req1.Name, tenant.Name)
	assert.Equal(t, req1.Region, tenant.Region)
	assert.Equal(t, req1.OwnerId, tenant.OwnerId)
	assert.Equal(t, req1.OwnerType, tenant.OwnerType)
	assert.Equal(t, req1.Role, tenant.Role)
	assert.Equal(t, req1.Labels, tenant.Labels)
}
