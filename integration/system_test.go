//go:build integration
// +build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	typespb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/service"
	"github.com/openkcm/registry/internal/validation"
)

func TestSystemService(t *testing.T) {
	// given
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer func() {
		require.NoError(t, conn.Close())
	}()

	sSubj := systemgrpc.NewServiceClient(conn)
	mSubj := mappinggrpc.NewServiceClient(conn)

	ctx := t.Context()
	db, err := startDB()
	require.NoError(t, err)

	tenant := validTenant()
	err = createTenantInDB(ctx, db, tenant)
	assert.NoError(t, err)
	existingTenantID := tenant.ID
	defer func() {
		err = deleteTenantFromDB(ctx, db, tenant)
		assert.NoError(t, err)
	}()

	t.Run("RegisterSystem", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			// given
			tts := map[string]func(req *systemgrpc.RegisterSystemRequest) *systemgrpc.RegisterSystemRequest{
				"request if empty": func(req *systemgrpc.RegisterSystemRequest) *systemgrpc.RegisterSystemRequest {
					return &systemgrpc.RegisterSystemRequest{}
				},
				"external ID is empty": func(req *systemgrpc.RegisterSystemRequest) *systemgrpc.RegisterSystemRequest {
					req.ExternalId = ""
					return req
				},
				"region is empty": func(req *systemgrpc.RegisterSystemRequest) *systemgrpc.RegisterSystemRequest {
					req.Region = ""
					return req
				},
				"region is not present in allowlist": func(req *systemgrpc.RegisterSystemRequest) *systemgrpc.RegisterSystemRequest {
					req.Region = "unknown"
					return req
				},
				"type is empty": func(req *systemgrpc.RegisterSystemRequest) *systemgrpc.RegisterSystemRequest {
					req.Type = ""
					return req
				},
				"type is not present in allowlist": func(req *systemgrpc.RegisterSystemRequest) *systemgrpc.RegisterSystemRequest {
					req.Type = "unknown"
					return req
				},
				"status is unspecified": func(req *systemgrpc.RegisterSystemRequest) *systemgrpc.RegisterSystemRequest {
					req.Status = typespb.Status_STATUS_UNSPECIFIED
					return req
				},
				"l2 key ID is empty": func(req *systemgrpc.RegisterSystemRequest) *systemgrpc.RegisterSystemRequest {
					req.L2KeyId = ""
					return req
				},
			}

			for reason, requestTransformer := range tts {
				t.Run(reason, func(t *testing.T) {
					validReq := validRegisterSystemReq()
					req := requestTransformer(validReq)

					// when
					resp, err := sSubj.RegisterSystem(ctx, req)

					// then
					assert.Error(t, err)
					assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
					assert.Nil(t, resp)
				})
			}

			t.Run("Tenant doesn't exist", func(t *testing.T) {
				// when
				req := validRegisterSystemReq()
				req.TenantId = validRandID()
				res, err := sSubj.RegisterSystem(ctx, req)

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
				assert.Nil(t, res)
			})

			t.Run("system already exist but tenantID does not match", func(t *testing.T) {
				req := validRegisterSystemReq()
				externalID := req.ExternalId
				req.TenantId = existingTenantID
				res, err := sSubj.RegisterSystem(ctx, req)
				assert.NoError(t, err)
				assert.True(t, res.Success)

				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, req.TenantId, req.Type, req.Region, req.HasL1KeyClaim)

				req = validRegisterSystemReq()
				req.ExternalId = externalID
				req.TenantId = validRandID()
				res, err = sSubj.RegisterSystem(ctx, req)

				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Nil(t, res)
			})
		})

		t.Run("should succeed", func(t *testing.T) {
			// given
			req := validRegisterSystemReq()
			req.HasL1KeyClaim = true
			// when
			res, err := sSubj.RegisterSystem(ctx, req)

			// cleanup
			defer func() {
				err := deleteSystem(ctx, sSubj, req.GetExternalId(), req.GetType(), req.GetRegion())
				assert.NoError(t, err)
			}()

			// then
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.True(t, res.Success)
			actSys, err := getRegionalSystem(t, ctx, sSubj, req.GetExternalId(), req.GetRegion(), req.GetType())
			assert.NoError(t, err)
			assert.NotNil(t, actSys)
			assert.Equal(t, req.GetExternalId(), actSys.GetExternalId())
			assert.Equal(t, req.GetRegion(), actSys.GetRegion())
			assert.Equal(t, req.GetType(), actSys.GetType())
			assert.Equal(t, req.GetL2KeyId(), actSys.GetL2KeyId())
			assert.Equal(t, req.GetTenantId(), actSys.GetTenantId())
			assert.True(t, actSys.GetHasL1KeyClaim())
			assert.Equal(t, req.Labels, actSys.GetLabels())
		})

		t.Run("should only register system once when multiple regional systems are registered for the system", func(t *testing.T) {
			req1 := validRegisterSystemReq()
			externalID := req1.ExternalId
			res, err := sSubj.RegisterSystem(ctx, req1)
			assert.NoError(t, err)
			assert.True(t, res.Success)

			req2 := validRegisterSystemReq()
			req2.ExternalId = externalID
			req2.Region = "region-system"
			res, err = sSubj.RegisterSystem(ctx, req2)
			assert.NoError(t, err)
			assert.True(t, res.Success)

			defer func() {
				assert.NoError(t, deleteSystem(ctx, sSubj, externalID, req1.GetType(), req1.Region))
				assert.NoError(t, deleteSystem(ctx, sSubj, externalID, req2.GetType(), req2.Region))
			}()

			sys1, err := getRegionalSystem(t, ctx, sSubj, externalID, req1.Region, req1.Type)
			assert.NoError(t, err)

			sys2, err := getRegionalSystem(t, ctx, sSubj, externalID, req2.Region, req2.Type)
			assert.NoError(t, err)

			assert.Equal(t, sys1.GetExternalId(), sys2.GetExternalId())
			assert.Equal(t, sys1.GetType(), sys2.GetType())
		})
	})

	t.Run("DeleteSystem", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("system identifier is nil", func(t *testing.T) {
				res, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					Region: allowedSystemRegion,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Nil(t, res)
			})
			t.Run("tenant is already assigned", func(t *testing.T) {
				// given
				externalID, systemType, systemRegion := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)

				// when
				result, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: externalID,
					Type:       systemType,
					Region:     systemRegion,
				})

				// clean up
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, existingTenantID, systemType, systemRegion, false)

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, result)
				actSys, err := listSystems(ctx, sSubj, existingTenantID)
				assert.NoError(t, err)
				assert.Len(t, actSys.GetSystems(), 1)
			})

			t.Run("regional system status unavailable", func(t *testing.T) {
				// given
				req := validRegisterSystemReq()
				req.Status = typespb.Status_STATUS_PROCESSING
				res, err := sSubj.RegisterSystem(ctx, req)
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.Success)

				// when
				result, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: req.GetExternalId(),
					Type:       req.GetType(),
					Region:     req.GetRegion(),
				})

				// clean up
				defer func() {
					err = deleteSystem(ctx, sSubj, req.GetExternalId(), req.GetType(), req.GetRegion())
					assert.NoError(t, err)
				}()

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, result)
				actSys, err := getRegionalSystem(t, ctx, sSubj, req.GetExternalId(), req.GetRegion(), req.GetType())
				assert.NoError(t, err)
				assert.NotNil(t, actSys)
			})
		})

		t.Run("should succeed if ", func(t *testing.T) {
			t.Run("regional system can be deleted", func(t *testing.T) {
				// given
				externalID, systemType, systemRegion := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)

				// when
				result, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: externalID,
					Type:       systemType,
					Region:     systemRegion,
				})

				// then
				assert.NoError(t, err)
				assert.True(t, result.Success)
			})

			t.Run("system cannot be found", func(t *testing.T) {
				// when
				res, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: uuid.NewString(),
					Type:       allowedSystemType,
					Region:     allowedSystemRegion,
				})

				// then
				assert.NoError(t, err)
				assert.True(t, res.Success)
			})
		})

		t.Run("should not delete system when ", func(t *testing.T) {
			t.Run("other regional systems exist", func(t *testing.T) {
				externalID := uuid.NewString()
				region := "region-system"
				externalID, systemType, systemRegion := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, &externalID)
				_, _, systemRegion2 := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, &region, &externalID)

				result, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: externalID,
					Type:       systemType,
					Region:     systemRegion,
				})

				// then
				assert.NoError(t, err)
				assert.True(t, result.Success)

				defer func() {
					result, err = sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
						ExternalId: externalID,
						Type:       systemType,
						Region:     systemRegion2,
					})

					assert.NoError(t, err)
					assert.True(t, result.Success)
				}()

				sys, err := getSystemFromDB(ctx, db, externalID, systemType)
				assert.NoError(t, err)
				assert.NotNil(t, sys)

				assert.Equal(t, sys.ExternalID, externalID)
				assert.Equal(t, sys.Type, systemType)
			})
		})
	})

	t.Run("UpdateSystemL1KeyClaim", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("system identifier is nil", func(t *testing.T) {
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					Region:     allowedSystemRegion,
					TenantId:   existingTenantID,
					L1KeyClaim: true,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Nil(t, res)
			})
			t.Run("system is not present", func(t *testing.T) {
				// when
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					Type:       allowedSystemType,
					ExternalId: uuid.NewString(),
					Region:     allowedSystemRegion,
					TenantId:   existingTenantID,
					L1KeyClaim: true,
				})
				// then
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
				assert.Nil(t, res)
			})

			t.Run("system is not linked to the tenant", func(t *testing.T) {
				// given
				req := validRegisterSystemReq()
				res, err := sSubj.RegisterSystem(ctx, req)
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.Success)

				defer func() {
					err = deleteSystem(ctx, sSubj, req.GetExternalId(), req.GetType(), req.GetRegion())
					assert.NoError(t, err)
				}()
				// when
				result, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: req.GetExternalId(),
					Type:       req.GetType(),
					Region:     req.GetRegion(),
					TenantId:   existingTenantID,
					L1KeyClaim: true,
				})
				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, result)
			})

			t.Run("system status unavailable", func(t *testing.T) {
				// given
				req := validRegisterSystemReq()
				req.TenantId = existingTenantID
				req.Status = typespb.Status_STATUS_PROCESSING
				res, err := sSubj.RegisterSystem(ctx, req)
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.Success)

				defer func() {
					err = unlinkSystemFromTenant(ctx, sSubj, mSubj, req.GetExternalId(), req.GetRegion(), req.TenantId)
					assert.NoError(t, err)
					err = deleteSystem(ctx, sSubj, req.GetExternalId(), req.GetType(), req.GetRegion())
					assert.NoError(t, err)
				}()
				// when
				result, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{

					ExternalId: req.GetExternalId(),
					Type:       req.GetType(),
					Region:     req.GetRegion(),
					TenantId:   existingTenantID,
					L1KeyClaim: true,
				})
				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, result)
			})

			t.Run("tenant_id is not provided in request", func(t *testing.T) {
				// given
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, existingTenantID, systemType, region, false)

				// when
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: externalID,
					Type:       systemType,
					Region:     region,
					L1KeyClaim: true,
				})
				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, res)
			})

			t.Run("L1KeyClaim is already", func(t *testing.T) {
				tts := map[string]bool{
					"false": false,
					"true":  true,
				}

				for name, l1KeyClaim := range tts {
					t.Run(name, func(t *testing.T) {
						externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, l1KeyClaim, allowedSystemType, nil, nil)
						defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, existingTenantID, systemType, region, l1KeyClaim)

						// when
						res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
							ExternalId: externalID,
							Type:       systemType,
							Region:     region,
							TenantId:   existingTenantID,
							L1KeyClaim: l1KeyClaim,
						})
						// then
						assert.Error(t, err)
						assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
						assert.Nil(t, res)
					})
				}
			})
		})

		t.Run("should successfully update L1KeyClaim to", func(t *testing.T) {
			// given
			externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)
			defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, existingTenantID, systemType, region, false)

			t.Run("true then to false", func(t *testing.T) {
				// when
				result, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: externalID,
					Type:       systemType,
					Region:     region,
					TenantId:   existingTenantID,
					L1KeyClaim: true,
				})
				// then
				assert.NoError(t, err)
				assert.True(t, result.GetSuccess())
				lRes, err := listSystems(ctx, sSubj, existingTenantID)
				assert.NoError(t, err)
				assert.True(t, lRes.Systems[0].HasL1KeyClaim)

				// when updating to false
				result, err = sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: externalID,
					Type:       systemType,
					Region:     region,
					TenantId:   existingTenantID,
					L1KeyClaim: false,
				})
				// then
				assert.NoError(t, err)
				assert.True(t, result.GetSuccess())
				lRes, err = listSystems(ctx, sSubj, existingTenantID)
				assert.NoError(t, err)
				assert.False(t, lRes.Systems[0].HasL1KeyClaim)
			})
		})
	})

	t.Run("ListSystems", func(t *testing.T) {
		t.Run("should return an error if no entries exist", func(t *testing.T) {
			// when
			resp, err := sSubj.ListSystems(ctx, &systemgrpc.ListSystemsRequest{
				TenantId: "random-tenant-id",
			})

			// then
			assert.Error(t, err)
			assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			assert.Nil(t, resp)
		})

		t.Run("when entries exist", func(t *testing.T) {
			// given
			externalID1, type1, region1 := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)
			externalID2, type2, region2 := registerRegionalSystem(t, ctx, sSubj, "", false, "application", nil, nil)

			// clean up
			defer func() {
				cleanupSystem(t, ctx, sSubj, mSubj, externalID1, existingTenantID, type1, region1, false)
				cleanupSystem(t, ctx, sSubj, mSubj, externalID2, "", type2, region2, false)
			}()

			t.Run("should return an error when no filter is applied", func(t *testing.T) {
				// when
				resp, err := sSubj.ListSystems(ctx, &systemgrpc.ListSystemsRequest{})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Nil(t, resp)
			})

			t.Run("should return System filtered by", func(t *testing.T) {
				// given
				tests := []struct {
					name               string
					request            *systemgrpc.ListSystemsRequest
					expectedExternalID string
				}{
					{
						name: "externalID",
						request: &systemgrpc.ListSystemsRequest{
							ExternalId: externalID1,
						},
						expectedExternalID: externalID1,
					},
					{
						name: "TenantID",
						request: &systemgrpc.ListSystemsRequest{
							TenantId: existingTenantID,
						},
						expectedExternalID: externalID1,
					},
					{
						name: "Type",
						request: &systemgrpc.ListSystemsRequest{
							ExternalId: externalID2,
							Type:       type2,
						},
						expectedExternalID: externalID2,
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						// when
						resp, err := sSubj.ListSystems(ctx, tt.request)
						// then
						assert.NoError(t, err)
						assert.Len(t, resp.GetSystems(), 1)
						assert.Equal(t, tt.expectedExternalID, resp.GetSystems()[0].ExternalId)
					})
				}
			})

			t.Run("should return an error if", func(t *testing.T) {
				// given
				tests := []struct {
					name      string
					request   *systemgrpc.ListSystemsRequest
					errorCode codes.Code
				}{
					{
						name: "non-existent TenantID is provided",
						request: &systemgrpc.ListSystemsRequest{
							TenantId: uuid.NewString(),
						},
						errorCode: codes.NotFound,
					},
					{
						name:      "no tenantID and no externalID is provided in query",
						request:   &systemgrpc.ListSystemsRequest{},
						errorCode: codes.InvalidArgument,
					},
					{
						name: "only system type is provided in query",
						request: &systemgrpc.ListSystemsRequest{
							Type: "system",
						},
						errorCode: codes.InvalidArgument,
					},
					{
						name: "non-existent system type is provided in query",
						request: &systemgrpc.ListSystemsRequest{
							TenantId: existingTenantID,
							Type:     "non-existent-type",
						},
						errorCode: codes.NotFound,
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						// when
						resp, err := sSubj.ListSystems(ctx, tt.request)
						// then
						assert.Error(t, err)
						assert.Equal(t, tt.errorCode, status.Code(err), err.Error())
						assert.Nil(t, resp)
					})
				}
			})
		})
	})

	t.Run("ListSystemsPagination", func(t *testing.T) {
		req1 := validRegisterSystemReq()
		req1.L2KeyId = "l1"
		req1.TenantId = existingTenantID
		res1, err := sSubj.RegisterSystem(ctx, req1)
		assert.NoError(t, err)
		assert.NotNil(t, res1)
		assert.True(t, res1.Success)

		defer cleanupSystem(t, ctx, sSubj, mSubj, req1.GetExternalId(), req1.GetTenantId(), req1.GetType(), req1.GetRegion(), req1.GetHasL1KeyClaim())

		req2 := validRegisterSystemReq()
		req2.L2KeyId = "l2"
		req2.TenantId = existingTenantID
		res2, err := sSubj.RegisterSystem(ctx, req2)
		assert.NoError(t, err)
		assert.NotNil(t, res2)
		assert.True(t, res2.Success)
		assert.NoError(t, err)

		defer cleanupSystem(t, ctx, sSubj, mSubj, req2.GetExternalId(), req2.GetTenantId(), req2.GetType(), req2.GetRegion(), req2.GetHasL1KeyClaim())

		t.Run("should return next page token with applied limit", func(t *testing.T) {
			req := &systemgrpc.ListSystemsRequest{
				Limit:    1,
				TenantId: existingTenantID,
			}
			res, err := sSubj.ListSystems(ctx, req)
			assert.NoError(t, err)
			assert.NotEmpty(t, res.NextPageToken)
			assert.Len(t, res.Systems, 1)
			assert.Equal(t, "l2", res.Systems[0].L2KeyId)
		})

		t.Run("should not return next page token if limit is greater than number of systems", func(t *testing.T) {
			req := &systemgrpc.ListSystemsRequest{
				TenantId: existingTenantID,
				Limit:    3,
			}
			res, _ := sSubj.ListSystems(ctx, req)
			assert.NoError(t, err)
			assert.Empty(t, res.NextPageToken)
			assert.Len(t, res.Systems, 2)
		})

		t.Run("page token point to next page", func(t *testing.T) {
			// first page
			req := &systemgrpc.ListSystemsRequest{
				Limit:    1,
				TenantId: existingTenantID,
			}
			res, err := sSubj.ListSystems(ctx, req)
			assert.NoError(t, err)
			assert.NotEmpty(t, res.NextPageToken)
			assert.Len(t, res.Systems, 1)
			assert.Equal(t, "l2", res.Systems[0].L2KeyId)

			// second page
			req = &systemgrpc.ListSystemsRequest{
				Limit:     1,
				TenantId:  existingTenantID,
				PageToken: res.NextPageToken,
			}
			res, err = sSubj.ListSystems(ctx, req)
			assert.NoError(t, err)
			assert.Len(t, res.Systems, 1)
			assert.Equal(t, "l1", res.Systems[0].L2KeyId)
		})
	})

	t.Run("UpdateSystemStatus", func(t *testing.T) {
		t.Run("should succeed if", func(t *testing.T) {
			t.Run("request is valid", func(t *testing.T) {
				// given
				externalID, systemType, systemRegion := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)

				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, systemRegion, false)

				// when
				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					ExternalId: externalID,
					Type:       systemType,
					Region:     systemRegion,
					Status:     typespb.Status_STATUS_PROCESSING,
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
			})
		})

		t.Run("should return error if", func(t *testing.T) {
			t.Run("system identifier is nil", func(t *testing.T) {
				// when
				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					Region: allowedSystemRegion,
					Status: typespb.Status_STATUS_AVAILABLE,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Nil(t, res)
			})

			t.Run("system to update is not present in the database", func(t *testing.T) {
				// when
				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					ExternalId: uuid.NewString(),
					Type:       allowedSystemType,
					Region:     allowedSystemRegion,
					Status:     typespb.Status_STATUS_AVAILABLE,
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, service.ErrSystemNotFound)
				assert.Nil(t, res)
			})
		})
	})

	t.Run("SetSystemLabels", func(t *testing.T) {
		t.Run("should succeed if", func(t *testing.T) {
			t.Run("request is valid", func(t *testing.T) {
				// given
				externalID, systemType, systemRegion := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)

				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, systemRegion, false)

				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					Type:       systemType,
					ExternalId: externalID,
					Region:     systemRegion,
					Labels: map[string]string{
						"key1": "value12",
						"key3": "value3",
					},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
				actSys, err := getRegionalSystem(t, ctx, sSubj, externalID, systemRegion, systemType)
				assert.NoError(t, err)
				assert.NotNil(t, actSys)
				assert.Len(t, actSys.GetLabels(), 3)
				assert.Equal(t, "value12", actSys.GetLabels()["key1"])
				assert.Equal(t, "value2", actSys.GetLabels()["key2"])
				assert.Equal(t, "value3", actSys.GetLabels()["key3"])
			})
		})

		t.Run("should return error if", func(t *testing.T) {
			t.Run("system identifier is nil", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					Region: allowedSystemRegion,
					Labels: map[string]string{
						"key1": "value1",
					},
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Nil(t, res)
			})
			t.Run("region is not valid", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: "externalID1",
					Type:       allowedSystemType,
					Region:     "",
					Labels: map[string]string{
						"key1": "value1",
					},
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Contains(t, err.Error(), model.RegionalSystemRegionValidationID)
				assert.Nil(t, res)
			})
			t.Run("labels are empty", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					Type:       allowedSystemType,
					ExternalId: "externalID1",
					Region:     allowedSystemRegion,
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, service.ErrMissingLabels)
				assert.Nil(t, res)
			})
			t.Run("labels keys are empty", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					Type:       allowedSystemType,
					ExternalId: "externalID1",
					Region:     allowedSystemRegion,
					Labels: map[string]string{
						"": "value1",
					},
				})

				// then
				assert.Error(t, err)
				assert.Nil(t, res)
			})
			t.Run("system to update is not present in the database", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					Type:       allowedSystemType,
					ExternalId: "externalID1",
					Region:     allowedSystemRegion,
					Labels: map[string]string{
						"key1": "value1",
					},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrSystemNotFound, err)
				assert.Nil(t, res)
			})
		})
	})

	t.Run("RemoveSystemLabels", func(t *testing.T) {
		t.Run("should succeed if", func(t *testing.T) {
			t.Run("request is valid", func(t *testing.T) {
				// given
				externalID, systemType, systemRegion := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)

				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, systemRegion, false)

				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					Type:       systemType,
					ExternalId: externalID,
					Region:     systemRegion,
					LabelKeys:  []string{"key1"},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
				actSys, err := getRegionalSystem(t, ctx, sSubj, externalID, systemRegion, systemType)
				assert.NoError(t, err)
				assert.NotNil(t, actSys)
				assert.Len(t, actSys.GetLabels(), 1)
				assert.Empty(t, actSys.GetLabels()["key1"])
			})
			t.Run("label does not exist", func(t *testing.T) {
				// given
				externalID, systemType, systemRegion := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, systemRegion, false)

				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					Type:       systemType,
					ExternalId: externalID,
					Region:     systemRegion,
					LabelKeys:  []string{"key3"},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
				actSys, err := getRegionalSystem(t, ctx, sSubj, externalID, systemRegion, systemType)
				assert.NoError(t, err)
				assert.NotNil(t, actSys)
				assert.Len(t, actSys.GetLabels(), 2)
			})
		})

		t.Run("should return error if", func(t *testing.T) {
			t.Run("external ID is empty", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					Type:      allowedSystemType,
					Region:    allowedSystemRegion,
					LabelKeys: []string{"key1"},
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Contains(t, err.Error(), model.SystemExternalIDValidationID)
				assert.Contains(t, err.Error(), validation.ErrValueEmpty.Error())
				assert.Nil(t, res)
			})
			t.Run("region is not valid", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					Type:       allowedSystemType,
					ExternalId: "externalID1",
					Region:     "",
					LabelKeys:  []string{"key1"},
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Contains(t, err.Error(), model.RegionalSystemRegionValidationID)
				assert.Nil(t, res)
			})
			t.Run("labels keys are empty", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					Type:       allowedSystemType,
					ExternalId: "externalID1",
					Region:     allowedSystemRegion,
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, service.ErrMissingLabelKeys)
				assert.Nil(t, res)
			})
			t.Run("labels keys have empty value", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					Type:       allowedSystemType,
					ExternalId: "externalID1",
					Region:     allowedSystemRegion,
					LabelKeys:  []string{""},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, service.ErrEmptyLabelKeys)
				assert.Nil(t, res)
			})
			t.Run("system to update is not present in the database", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					Type:       allowedSystemType,
					ExternalId: "externalID1",
					Region:     allowedSystemRegion,
					LabelKeys:  []string{"key1"},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrSystemNotFound, err)
				assert.Nil(t, res)
			})
		})
	})

	t.Run("API Backwards Compatibility", func(t *testing.T) {
		t.Run("deleteSystem without type", func(t *testing.T) {
			t.Run("should error out when", func(t *testing.T) {
				tt := []struct {
					name string
					req  *systemgrpc.DeleteSystemRequest
				}{
					{
						name: "externalID is not provided",
						req:  &systemgrpc.DeleteSystemRequest{Region: allowedSystemRegion},
					},
					{
						name: "region is not provided",
						req:  &systemgrpc.DeleteSystemRequest{ExternalId: validRandID()},
					},
					{
						name: "region is invalid",
						req: &systemgrpc.DeleteSystemRequest{
							ExternalId: validRandID(),
							Region:     "invalid-region",
						},
					},
					{
						name: "both are not provided",
						req:  &systemgrpc.DeleteSystemRequest{},
					},
				}
				for _, tc := range tt {
					t.Run(tc.name, func(t *testing.T) {
						res, err := sSubj.DeleteSystem(ctx, tc.req)
						assert.Error(t, err)
						assert.Nil(t, res)
						assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
					})
				}

				t.Run("multiple systems are found", func(t *testing.T) {
					externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
					defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)

					anotherExternalID, anotherSystemType, anotherRegion := registerRegionalSystem(t, ctx, sSubj, "", false, "system", nil, &externalID)
					defer cleanupSystem(t, ctx, sSubj, mSubj, anotherExternalID, "", anotherSystemType, anotherRegion, false)

					deleteRes, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
						ExternalId: externalID,
						Region:     region,
					})
					assert.Error(t, err)
					assert.Nil(t, deleteRes)
					assert.Equal(t, status.Code(service.ErrTooManyTypes), status.Code(err))
				})
			})
			t.Run("should not error out when no system is found", func(t *testing.T) {
				res, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: validRandID(),
					Region:     allowedSystemRegion,
				})
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
			})
			t.Run("should not error out when regional system has different region", func(t *testing.T) {
				req := validRegisterSystemReq()
				res, err := sSubj.RegisterSystem(ctx, req)
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.Success)

				defer cleanupSystem(t, ctx, sSubj, mSubj, req.GetExternalId(), req.GetTenantId(), req.GetType(), req.GetRegion(), req.GetHasL1KeyClaim())
				delRes, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: req.GetExternalId(),
					Region:     "region-system",
				})
				assert.NoError(t, err)
				assert.NotNil(t, delRes)
				assert.True(t, res.GetSuccess())
			})
			t.Run("should not error out when no regional system is found", func(t *testing.T) {
				system := &model.System{
					ExternalID: validRandID(),
					Type:       allowedSystemType,
				}
				assert.NoError(t, createSystemInDB(ctx, db, system))
				defer func() {
					assert.NoError(t, deleteSystemInDB(ctx, db, system.ExternalID, system.Type))
				}()

				res, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: system.ExternalID,
					Region:     allowedSystemRegion,
				})
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
			})
			t.Run("should delete successfully", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)

				result, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: externalID,
					Region:     region,
				})
				assert.NoError(t, err)
				assert.True(t, result.GetSuccess())

				listRes, err := sSubj.ListSystems(ctx, &systemgrpc.ListSystemsRequest{
					ExternalId: externalID,
					Region:     region,
				})
				assert.Error(t, err)
				assert.Nil(t, listRes)
				assert.Equal(t, codes.NotFound, status.Code(err))
			})
		})

		t.Run("updateSystemL1KeyClaim without type", func(t *testing.T) {
			t.Run("should error out when", func(t *testing.T) {
				tt := []struct {
					name string
					req  *systemgrpc.UpdateSystemL1KeyClaimRequest
				}{
					{
						name: "externalID is not provided",
						req:  &systemgrpc.UpdateSystemL1KeyClaimRequest{Region: allowedSystemRegion},
					},
					{
						name: "region is not provided",
						req:  &systemgrpc.UpdateSystemL1KeyClaimRequest{ExternalId: validRandID()},
					},
					{
						name: "region is invalid",
						req: &systemgrpc.UpdateSystemL1KeyClaimRequest{
							ExternalId: validRandID(),
							Region:     "invalid-region",
						},
					},
					{
						name: "both are not provided",
						req:  &systemgrpc.UpdateSystemL1KeyClaimRequest{},
					},
				}
				for _, tc := range tt {
					t.Run(tc.name, func(t *testing.T) {
						res, err := sSubj.UpdateSystemL1KeyClaim(ctx, tc.req)
						assert.Error(t, err)
						assert.Nil(t, res)
						assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
					})
				}

				t.Run("multiple systems are found", func(t *testing.T) {
					externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
					defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)

					anotherExternalID, anotherSystemType, anotherRegion := registerRegionalSystem(t, ctx, sSubj, "", false, "system", nil, &externalID)
					defer cleanupSystem(t, ctx, sSubj, mSubj, anotherExternalID, "", anotherSystemType, anotherRegion, false)

					deleteRes, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
						ExternalId: externalID,
						Region:     region,
					})
					assert.Error(t, err)
					assert.Nil(t, deleteRes)
					assert.Equal(t, status.Code(service.ErrTooManyTypes), status.Code(err))
				})
			})
			t.Run("should error out when no system is found", func(t *testing.T) {
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: validRandID(),
					Region:     allowedSystemRegion,
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should error out when regional system has different region", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: externalID,
					Region:     "region-system",
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should error out when no regional system is found", func(t *testing.T) {
				system := &model.System{
					ExternalID: validRandID(),
					Type:       allowedSystemType,
				}
				assert.NoError(t, createSystemInDB(ctx, db, system))
				defer func() {
					assert.NoError(t, deleteSystemInDB(ctx, db, system.ExternalID, system.Type))
				}()

				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: system.ExternalID,
					Region:     allowedSystemRegion,
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should update L1KeyClaim successfully", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, existingTenantID, systemType, region, false)

				result, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: externalID,
					Region:     region,
					TenantId:   existingTenantID,
					L1KeyClaim: true,
				})
				// then
				assert.NoError(t, err)
				assert.True(t, result.GetSuccess())
				lRes, err := sSubj.ListSystems(ctx, &systemgrpc.ListSystemsRequest{
					TenantId: existingTenantID,
				})
				assert.NoError(t, err)
				assert.True(t, lRes.Systems[0].HasL1KeyClaim)

				// when updating to false
				result, err = sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: externalID,
					Region:     region,
					TenantId:   existingTenantID,
					L1KeyClaim: false,
				})
				// then
				assert.NoError(t, err)
				assert.True(t, result.GetSuccess())
				lRes, err = sSubj.ListSystems(ctx, &systemgrpc.ListSystemsRequest{
					TenantId: existingTenantID,
				})
				assert.NoError(t, err)
				assert.False(t, lRes.Systems[0].HasL1KeyClaim)
			})
		})

		t.Run("updateSystemStatus without type", func(t *testing.T) {
			t.Run("should error out when", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)
				tt := []struct {
					name string
					req  *systemgrpc.UpdateSystemStatusRequest
				}{
					{
						name: "externalID is not provided",
						req:  &systemgrpc.UpdateSystemStatusRequest{Region: region},
					},
					{
						name: "region is not provided",
						req:  &systemgrpc.UpdateSystemStatusRequest{ExternalId: externalID},
					},
					{
						name: "region is invalid",
						req: &systemgrpc.UpdateSystemStatusRequest{
							ExternalId: externalID,
							Region:     "invalid-region",
						},
					},
					{
						name: "both are not provided",
						req:  &systemgrpc.UpdateSystemStatusRequest{},
					},
				}
				for _, tc := range tt {
					t.Run(tc.name, func(t *testing.T) {
						res, err := sSubj.UpdateSystemStatus(ctx, tc.req)
						assert.Error(t, err)
						assert.Nil(t, res)
						assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
					})
				}

				t.Run("multiple systems are found", func(t *testing.T) {
					externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
					defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)

					anotherExternalID, anotherSystemType, anotherRegion := registerRegionalSystem(t, ctx, sSubj, "", false, "system", nil, &externalID)
					defer cleanupSystem(t, ctx, sSubj, mSubj, anotherExternalID, "", anotherSystemType, anotherRegion, false)

					deleteRes, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
						ExternalId: externalID,
						Region:     region,
					})
					assert.Error(t, err)
					assert.Nil(t, deleteRes)
					assert.Equal(t, status.Code(service.ErrTooManyTypes), status.Code(err))
				})
			})
			t.Run("should error out when no system is found", func(t *testing.T) {
				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					ExternalId: validRandID(),
					Region:     allowedSystemRegion,
					Status:     typespb.Status_STATUS_PROCESSING,
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should error out when regional system has different region", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)

				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					ExternalId: externalID,
					Region:     "region-system",
					Status:     typespb.Status_STATUS_PROCESSING,
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should error out when no regional system is found", func(t *testing.T) {
				system := &model.System{
					ExternalID: validRandID(),
					Type:       allowedSystemType,
				}
				assert.NoError(t, createSystemInDB(ctx, db, system))
				defer func() {
					assert.NoError(t, deleteSystemInDB(ctx, db, system.ExternalID, system.Type))
				}()

				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					ExternalId: system.ExternalID,
					Region:     allowedSystemRegion,
					Status:     typespb.Status_STATUS_PROCESSING,
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should update status successfully", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)

				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					ExternalId: externalID,
					Region:     region,
					Status:     typespb.Status_STATUS_PROCESSING,
				})
				// then
				assert.NoError(t, err)
				assert.True(t, res.GetSuccess())
				listRes, err := sSubj.ListSystems(ctx, &systemgrpc.ListSystemsRequest{
					ExternalId: externalID,
					Region:     region,
				})
				assert.NoError(t, err)
				assert.Len(t, listRes.Systems, 1)
				assert.Equal(t, typespb.Status_STATUS_PROCESSING, listRes.Systems[0].GetStatus())
			})
		})

		t.Run("setSystemLabels without type", func(t *testing.T) {
			t.Run("should error out when", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)
				tt := []struct {
					name string
					req  *systemgrpc.SetSystemLabelsRequest
				}{
					{
						name: "externalID is not provided",
						req:  &systemgrpc.SetSystemLabelsRequest{Region: region},
					},
					{
						name: "region is not provided",
						req:  &systemgrpc.SetSystemLabelsRequest{ExternalId: externalID},
					},
					{
						name: "region is invalid",
						req: &systemgrpc.SetSystemLabelsRequest{
							ExternalId: region,
							Region:     "invalid-region",
						},
					},
					{
						name: "both are not provided",
						req:  &systemgrpc.SetSystemLabelsRequest{},
					},
				}
				for _, tc := range tt {
					t.Run(tc.name, func(t *testing.T) {
						res, err := sSubj.SetSystemLabels(ctx, tc.req)
						assert.Error(t, err)
						assert.Nil(t, res)
						assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
					})
				}

				t.Run("multiple systems are found", func(t *testing.T) {
					externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
					defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)

					anotherExternalID, anotherSystemType, anotherRegion := registerRegionalSystem(t, ctx, sSubj, "", false, "system", nil, &externalID)
					defer cleanupSystem(t, ctx, sSubj, mSubj, anotherExternalID, "", anotherSystemType, anotherRegion, false)

					labelRes, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
						ExternalId: externalID,
						Region:     region,
						Labels: map[string]string{
							"key1": "value12",
							"key3": "value3",
						},
					})
					assert.Error(t, err)
					assert.Nil(t, labelRes)
					assert.Equal(t, status.Code(service.ErrTooManyTypes), status.Code(err))
				})
			})
			t.Run("should error out when no system is found", func(t *testing.T) {
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: validRandID(),
					Region:     allowedSystemRegion,
					Labels: map[string]string{
						"key1": "value12",
						"key3": "value3",
					},
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should error out when regional system has different region", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: externalID,
					Region:     "region-system",
					Labels: map[string]string{
						"key1": "value12",
						"key3": "value3",
					},
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should error out when no regional system is found", func(t *testing.T) {
				system := &model.System{
					ExternalID: validRandID(),
					Type:       allowedSystemType,
				}
				assert.NoError(t, createSystemInDB(ctx, db, system))
				defer func() {
					assert.NoError(t, deleteSystemInDB(ctx, db, system.ExternalID, system.Type))
				}()

				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: system.ExternalID,
					Region:     allowedSystemRegion,
					Labels: map[string]string{
						"key1": "value12",
						"key3": "value3",
					},
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should set labels successfully", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)

				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: externalID,
					Region:     region,
					Labels: map[string]string{
						"key1": "value12",
						"key3": "value3",
					},
				})
				// then
				assert.NoError(t, err)
				assert.True(t, res.GetSuccess())
				listRes, err := sSubj.ListSystems(ctx, &systemgrpc.ListSystemsRequest{
					ExternalId: externalID,
					Region:     region,
				})
				assert.NoError(t, err)
				assert.Len(t, listRes.Systems, 1)
				assert.Len(t, listRes.Systems[0].GetLabels(), 3)
				assert.Equal(t, "value12", listRes.Systems[0].GetLabels()["key1"])
				assert.Equal(t, "value2", listRes.Systems[0].GetLabels()["key2"])
				assert.Equal(t, "value3", listRes.Systems[0].GetLabels()["key3"])
			})
		})

		t.Run("removeSystemLabels without type", func(t *testing.T) {
			t.Run("should error out when", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)
				tt := []struct {
					name string
					req  *systemgrpc.RemoveSystemLabelsRequest
				}{
					{
						name: "externalID is not provided",
						req:  &systemgrpc.RemoveSystemLabelsRequest{Region: region},
					},
					{
						name: "region is not provided",
						req:  &systemgrpc.RemoveSystemLabelsRequest{ExternalId: externalID},
					},
					{
						name: "region is invalid",
						req: &systemgrpc.RemoveSystemLabelsRequest{
							ExternalId: externalID,
							Region:     "invalid-region",
						},
					},
					{
						name: "both are not provided",
						req:  &systemgrpc.RemoveSystemLabelsRequest{},
					},
				}
				for _, tc := range tt {
					t.Run(tc.name, func(t *testing.T) {
						res, err := sSubj.RemoveSystemLabels(ctx, tc.req)
						assert.Error(t, err)
						assert.Nil(t, res)
						assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
					})
				}

				t.Run("multiple systems are found", func(t *testing.T) {
					externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
					defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)

					anotherExternalID, anotherSystemType, anotherRegion := registerRegionalSystem(t, ctx, sSubj, "", false, "system", nil, &externalID)
					defer cleanupSystem(t, ctx, sSubj, mSubj, anotherExternalID, "", anotherSystemType, anotherRegion, false)

					labelRes, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
						ExternalId: externalID,
						Region:     region,
						LabelKeys:  []string{"key1"},
					})
					assert.Error(t, err)
					assert.Nil(t, labelRes)
					assert.Equal(t, status.Code(service.ErrTooManyTypes), status.Code(err))
				})
			})
			t.Run("should error out when no system is found", func(t *testing.T) {
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: validRandID(),
					Region:     allowedSystemRegion,
					LabelKeys:  []string{"key1"},
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should error out when regional system has different region", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: externalID,
					Region:     "region-system",
					LabelKeys:  []string{"key1"},
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should error out when no regional system is found", func(t *testing.T) {
				system := &model.System{
					ExternalID: validRandID(),
					Type:       allowedSystemType,
				}
				assert.NoError(t, createSystemInDB(ctx, db, system))
				defer func() {
					assert.NoError(t, deleteSystemInDB(ctx, db, system.ExternalID, system.Type))
				}()

				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: system.ExternalID,
					Region:     allowedSystemRegion,
					LabelKeys:  []string{"key1"},
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
			})
			t.Run("should remove labels successfully", func(t *testing.T) {
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, externalID, "", systemType, region, false)

				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: externalID,
					Region:     region,
					LabelKeys:  []string{"key1"},
				})
				// then
				assert.NoError(t, err)
				assert.True(t, res.GetSuccess())
				listRes, err := sSubj.ListSystems(ctx, &systemgrpc.ListSystemsRequest{
					ExternalId: externalID,
					Region:     region,
				})
				assert.NoError(t, err)
				assert.Len(t, listRes.Systems, 1)
				assert.Len(t, listRes.Systems[0].GetLabels(), 1)
				assert.Equal(t, "value2", listRes.Systems[0].GetLabels()["key2"])
			})
		})
	})
}

func listSystems(ctx context.Context, subj systemgrpc.ServiceClient, tenantID string) (*systemgrpc.ListSystemsResponse, error) {
	req := &systemgrpc.ListSystemsRequest{}
	if tenantID != "" {
		req.TenantId = tenantID
	}
	return subj.ListSystems(ctx, req)
}

func getRegionalSystem(t *testing.T, ctx context.Context, subj systemgrpc.ServiceClient, externalID, region, systemType string) (*systemgrpc.System, error) {
	req := &systemgrpc.ListSystemsRequest{
		ExternalId: externalID,
		Region:     region,
		Type:       systemType,
	}
	resp, err := subj.ListSystems(ctx, req)
	if err != nil {
		return nil, err
	}
	assert.Len(t, resp.GetSystems(), 1, "Expected exactly one system to be returned")

	return resp.GetSystems()[0], nil
}
