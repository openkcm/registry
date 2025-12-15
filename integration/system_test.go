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

				defer cleanupSystem(t, ctx, sSubj, externalID, req.TenantId, req.Type, req.Region, req.HasL1KeyClaim)

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
				err := deleteSystem(ctx, sSubj, req.GetExternalId(), req.GetRegion())
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
				assert.NoError(t, deleteSystem(ctx, sSubj, externalID, req1.Region))
				assert.NoError(t, deleteSystem(ctx, sSubj, externalID, req2.Region))
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
					SystemIdentifier: nil,
					Region:           allowedSystemRegion,
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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: externalID,
						Type:       systemType,
					},
					Region: systemRegion,
				})

				// clean up
				defer cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, systemType, systemRegion, false)

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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: req.GetExternalId(),
						Type:       req.GetType(),
					},
					Region: req.GetRegion(),
				})

				// clean up
				defer func() {
					err = deleteSystem(ctx, sSubj, req.GetExternalId(), req.GetRegion())
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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: externalID,
						Type:       systemType,
					},
					Region: systemRegion,
				})

				// then
				assert.NoError(t, err)
				assert.True(t, result.Success)
			})

			t.Run("system cannot be found", func(t *testing.T) {
				// when
				res, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: uuid.NewString(),
						Type:       allowedSystemType,
					},
					Region: allowedSystemRegion,
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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: externalID,
						Type:       systemType,
					},
					Region: systemRegion,
				})

				// then
				assert.NoError(t, err)
				assert.True(t, result.Success)

				defer func() {
					result, err = sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
						SystemIdentifier: &systemgrpc.SystemIdentifier{
							ExternalId: externalID,
							Type:       systemType,
						},
						Region: systemRegion2,
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
					SystemIdentifier: nil,
					Region:           allowedSystemRegion,
					TenantId:         existingTenantID,
					L1KeyClaim:       true,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Nil(t, res)
			})
			t.Run("system is not present", func(t *testing.T) {
				// when
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       allowedSystemType,
						ExternalId: uuid.NewString(),
					},
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
					err = deleteSystem(ctx, sSubj, req.GetExternalId(), req.GetRegion())
					assert.NoError(t, err)
				}()
				// when
				result, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: req.GetExternalId(),
						Type:       req.GetType(),
					},
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
					err = unlinkSystemFromTenant(ctx, sSubj, req.GetExternalId(), req.GetRegion())
					assert.NoError(t, err)
					err = deleteSystem(ctx, sSubj, req.GetExternalId(), req.GetRegion())
					assert.NoError(t, err)
				}()
				// when
				result, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: req.GetExternalId(),
						Type:       req.GetType(),
					},
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
				defer cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, systemType, region, false)

				// when
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: externalID,
						Type:       systemType,
					},
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
						defer cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, systemType, region, l1KeyClaim)

						// when
						res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
							SystemIdentifier: &systemgrpc.SystemIdentifier{
								ExternalId: externalID,
								Type:       systemType,
							},
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
			defer cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, systemType, region, false)

			t.Run("true then to false", func(t *testing.T) {
				// when
				result, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: externalID,
						Type:       systemType,
					},
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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: externalID,
						Type:       systemType,
					},
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
				cleanupSystem(t, ctx, sSubj, externalID1, existingTenantID, type1, region1, false)
				cleanupSystem(t, ctx, sSubj, externalID2, "", type2, region2, false)
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

		defer cleanupSystem(t, ctx, sSubj, req1.GetExternalId(), req1.GetTenantId(), req1.GetType(), req1.GetRegion(), req1.GetHasL1KeyClaim())

		req2 := validRegisterSystemReq()
		req2.L2KeyId = "l2"
		req2.TenantId = existingTenantID
		res2, err := sSubj.RegisterSystem(ctx, req2)
		assert.NoError(t, err)
		assert.NotNil(t, res2)
		assert.True(t, res2.Success)
		assert.NoError(t, err)

		defer cleanupSystem(t, ctx, sSubj, req2.GetExternalId(), req2.GetTenantId(), req2.GetType(), req2.GetRegion(), req2.GetHasL1KeyClaim())

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

	t.Run("UnlinkSystemsFromTenant", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("system Identifier is empty", func(t *testing.T) {
				// when
				res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{},
					},
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Nil(t, res)
			})
			t.Run("type in system Identifier is not valid", func(t *testing.T) {
				// when
				res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{
							ExternalId: "externalID",
						},
					},
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Contains(t, err.Error(), model.SystemTypeValidationID)
				assert.Nil(t, res)
			})
			t.Run("system to update is not present in the database", func(t *testing.T) {
				// when
				id := uuid.NewString()
				systemType := allowedSystemType
				res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{
							ExternalId: id,
							Type:       systemType,
						},
					},
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
				assert.Nil(t, res)
			})

			t.Run("one of the systems has active L1 key claim and rollback transaction", func(t *testing.T) {
				// given
				// system2 has active L1 key claim
				system2Region := "region-system"
				sys1ExternalID, sys1Type, sys1Region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)
				sys2ExternalID, sys2Type, sys2Region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, true, allowedSystemType, &system2Region, &sys1ExternalID)

				// clean up
				defer func() {
					cleanupSystem(t, ctx, sSubj, sys2ExternalID, existingTenantID, sys2Type, sys2Region, true)
					cleanupSystem(t, ctx, sSubj, sys1ExternalID, "", sys1Type, sys1Region, false)
				}()

				// when
				res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{
							ExternalId: sys1ExternalID,
							Type:       sys1Type,
						},
					},
				})

				// then
				expErr := service.ErrorWithParams(service.ErrSystemHasL1KeyClaim, "externalID", sys2ExternalID, "type", sys2Type, "region", sys2Region).Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
				actSys, err := listSystems(ctx, sSubj, existingTenantID)
				assert.NoError(t, err)
				assert.Len(t, actSys.GetSystems(), 2)
			})

			t.Run("one of the systems is not linked to the tenant and rollback transaction", func(t *testing.T) {
				// given: system2 is not linked to the tenant
				systemID1, systemType1, region1 := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, systemID1, existingTenantID, systemType1, region1, false)

				systemID2, type2, region2 := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, systemID2, "", type2, region2, false)

				// when
				res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{
							ExternalId: systemID1,
							Type:       systemType1,
						}, {
							ExternalId: systemID2,
							Type:       type2,
						},
					},
				})
				// then
				expErr := service.ErrorWithParams(service.ErrSystemIsNotLinkedToTenant, "externalID", systemID2, "type", type2).Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
				actSys, err := listSystems(ctx, sSubj, existingTenantID)
				assert.NoError(t, err)
				assert.Len(t, actSys.GetSystems(), 1)
				assert.Equal(t, systemID1, actSys.GetSystems()[0].GetExternalId())
			})
		})

		t.Run("should succeed if l1KeyClaim is false", func(t *testing.T) {
			// given
			sys1ExternalID, type1, sys1Region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)
			sys2ExternalID, type2, sys2Region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)

			// clean up
			defer func() {
				cleanupSystem(t, ctx, sSubj, sys1ExternalID, "", type1, sys1Region, false)
				cleanupSystem(t, ctx, sSubj, sys2ExternalID, "", type2, sys2Region, false)
			}()

			// when
			res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
				SystemIdentifiers: []*systemgrpc.SystemIdentifier{
					{
						ExternalId: sys1ExternalID,
						Type:       type1,
					},
					{
						ExternalId: sys2ExternalID,
						Type:       type2,
					},
				},
			})

			// then
			assert.NoError(t, err)
			assert.True(t, res.GetSuccess())

			actSys1, err := getRegionalSystem(t, ctx, sSubj, sys1ExternalID, sys1Region, type1)
			assert.NoError(t, err)
			assert.NotNil(t, actSys1)

			actSys2, err := getRegionalSystem(t, ctx, sSubj, sys2ExternalID, sys2Region, type2)
			assert.NoError(t, err)
			assert.NotNil(t, actSys2)

			assert.Empty(t, actSys1.GetTenantId())
			assert.Empty(t, actSys2.GetTenantId())
		})
	})

	t.Run("LinkSystemsToTenant", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("external id is empty", func(t *testing.T) {
				// when
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: uuid.NewString(), Type: allowedSystemType},
						{ExternalId: "", Type: allowedSystemType},
						{ExternalId: uuid.NewString(), Type: allowedSystemType},
					},
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err), err.Error())
				assert.Contains(t, err.Error(), model.SystemExternalIDValidationID)
				assert.Contains(t, err.Error(), validation.ErrValueEmpty.Error())
				assert.Nil(t, res)
			})

			t.Run("Tenant does not exist", func(t *testing.T) {
				// given
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, externalID, "", systemType, region, false)
				tenantID := validRandID()

				// when
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: externalID, Type: systemType},
					},
					TenantId: tenantID,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
				assert.Nil(t, res)
			})

			t.Run("system has active L1KeyClaim", func(t *testing.T) {
				// given
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", true, allowedSystemType, nil, nil)

				// clean up
				defer func() {
					assert.NoError(t, deleteSystem(ctx, sSubj, externalID, region))
					assert.NoError(t, deleteSystem(ctx, sSubj, externalID, region))
				}()

				// when
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: externalID, Type: systemType},
					},
					TenantId: existingTenantID,
				})

				// then
				expErr := service.ErrorWithParams(service.ErrSystemHasL1KeyClaim, "externalID", externalID, "type", systemType, "region", region).Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
			})

			t.Run("system already has linked tenant", func(t *testing.T) {
				// given
				externalID, systemType, region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)

				tenant := validTenant()
				err := createTenantInDB(ctx, db, tenant)
				assert.NoError(t, err)

				// clean up
				defer func() {
					cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, systemType, region, false)
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: externalID, Type: systemType},
					},
					TenantId: tenant.ID,
				})

				// then
				expErr := service.ErrorWithParams(service.ErrSystemIsLinkedToTenant, "externalID", externalID, "type", systemType).Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
			})

			t.Run("should rollback transaction if any system fails to link", func(t *testing.T) {
				// given
				// system1 is already linked to the tenant
				sys1ExternalID, systemType1, sys1Region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)

				sys2ExternalID, systemType2, sys2Region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)

				defer func() {
					cleanupSystem(t, ctx, sSubj, sys1ExternalID, existingTenantID, systemType1, sys1Region, false)
					cleanupSystem(t, ctx, sSubj, sys2ExternalID, "", systemType2, sys2Region, false)
				}()

				// when
				tenantID := existingTenantID
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: sys1ExternalID, Type: systemType1},
						{ExternalId: sys2ExternalID, Type: systemType2},
					},
					TenantId: tenantID,
				})

				// then
				expErr := service.ErrorWithParams(service.ErrSystemIsLinkedToTenant, "externalID", sys1ExternalID, "type", systemType1).Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
				actSys, err := listSystems(ctx, sSubj, tenantID)
				assert.NoError(t, err)
				assert.Len(t, actSys.GetSystems(), 1)
				assert.Equal(t, tenantID, actSys.GetSystems()[0].GetTenantId())
			})
		})

		t.Run("should succeed when", func(t *testing.T) {
			t.Run("L1KeyClaim is false and Tenant is not linked", func(t *testing.T) {
				// given
				externalID, systemType, systemRegion := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				// when
				tenantID := existingTenantID

				// clean up
				defer cleanupSystem(t, ctx, sSubj, externalID, tenantID, systemType, systemRegion, false)

				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: externalID, Type: systemType},
					},
					TenantId: tenantID,
				})

				// then
				assert.NoError(t, err)
				assert.True(t, res.GetSuccess())
				actSys, err := listSystems(ctx, sSubj, tenantID)
				assert.NoError(t, err)
				assert.Len(t, actSys.GetSystems(), 1)
				assert.Equal(t, tenantID, actSys.GetSystems()[0].GetTenantId())
			})

			t.Run("system to link is not in the database so new system is created and then linked", func(t *testing.T) {
				// given
				systemID := uuid.NewString()
				systemType := allowedSystemType

				//clean up
				defer func() {
					res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
						SystemIdentifiers: []*systemgrpc.SystemIdentifier{
							{ExternalId: systemID, Type: systemType},
						},
					})
					assert.NoError(t, err)
					assert.True(t, res.GetSuccess())
					assert.NoError(t, deleteSystemInDB(ctx, db, systemID, systemType))
				}()

				// when
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: systemID, Type: systemType},
					},
					TenantId: existingTenantID,
				})

				assert.NoError(t, err)
				assert.True(t, res.GetSuccess())

				system, err := getSystemFromDB(ctx, db, systemID, systemType)
				assert.NoError(t, err)
				assert.NotNil(t, system)

				assert.Equal(t, existingTenantID, *system.TenantID)
				assert.Equal(t, allowedSystemType, system.Type)
				assert.Equal(t, systemID, system.ExternalID)
			})

			t.Run("linking multiple systems in one transaction", func(t *testing.T) {
				// given
				sys1ExternalID, sys1Type, sys1Region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				sys2ExternalID, sys2Type, sys2Region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				tenantID := existingTenantID
				defer func() {
					cleanupSystem(t, ctx, sSubj, sys1ExternalID, tenantID, sys1Type, sys1Region, false)
					cleanupSystem(t, ctx, sSubj, sys2ExternalID, tenantID, sys2Type, sys2Region, false)
				}()

				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: sys1ExternalID, Type: sys1Type},
						{ExternalId: sys2ExternalID, Type: sys2Type},
					},
					TenantId: tenantID,
				})

				// then
				assert.NoError(t, err)
				assert.True(t, res.GetSuccess())
				actSys, err := listSystems(ctx, sSubj, tenantID)
				assert.NoError(t, err)
				assert.Len(t, actSys.GetSystems(), 2)
				assert.Equal(t, tenantID, actSys.GetSystems()[0].GetTenantId())
				assert.Equal(t, tenantID, actSys.GetSystems()[1].GetTenantId())
			})
		})
	})

	t.Run("UpdateSystemStatus", func(t *testing.T) {
		t.Run("should succeed if", func(t *testing.T) {
			t.Run("request is valid", func(t *testing.T) {
				// given
				externalID, systemType, systemRegion := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)

				defer cleanupSystem(t, ctx, sSubj, externalID, "", systemType, systemRegion, false)

				// when
				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: externalID,
						Type:       systemType,
					},
					Region: systemRegion,
					Status: typespb.Status_STATUS_PROCESSING,
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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: uuid.NewString(),
						Type:       allowedSystemType,
					},
					Region: allowedSystemRegion,
					Status: typespb.Status_STATUS_AVAILABLE,
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

				defer cleanupSystem(t, ctx, sSubj, externalID, "", systemType, systemRegion, false)

				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       systemType,
						ExternalId: externalID,
					},
					Region: systemRegion,
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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						ExternalId: "externalID1",
						Type:       allowedSystemType,
					},
					Region: "",
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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       allowedSystemType,
						ExternalId: "externalID1",
					},
					Region: allowedSystemRegion,
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, service.ErrMissingLabels)
				assert.Nil(t, res)
			})
			t.Run("labels keys are empty", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       allowedSystemType,
						ExternalId: "externalID1",
					},
					Region: allowedSystemRegion,
					Labels: map[string]string{
						"": "value1",
					},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, model.ErrLabelsIncludeEmptyString)
				assert.Nil(t, res)
			})
			t.Run("system to update is not present in the database", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       allowedSystemType,
						ExternalId: "externalID1",
					},
					Region: allowedSystemRegion,
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

				defer cleanupSystem(t, ctx, sSubj, externalID, "", systemType, systemRegion, false)

				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       systemType,
						ExternalId: externalID,
					},
					Region:    systemRegion,
					LabelKeys: []string{"key1"},
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
				defer cleanupSystem(t, ctx, sSubj, externalID, "", systemType, systemRegion, false)

				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       systemType,
						ExternalId: externalID,
					},
					Region:    systemRegion,
					LabelKeys: []string{"key3"},
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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type: allowedSystemType,
					},
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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       allowedSystemType,
						ExternalId: "externalID1",
					},
					Region:    "",
					LabelKeys: []string{"key1"},
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
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       allowedSystemType,
						ExternalId: "externalID1",
					},
					Region: allowedSystemRegion,
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, service.ErrMissingLabelKeys)
				assert.Nil(t, res)
			})
			t.Run("labels keys have empty value", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       allowedSystemType,
						ExternalId: "externalID1",
					},
					Region:    allowedSystemRegion,
					LabelKeys: []string{""},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, service.ErrEmptyLabelKeys)
				assert.Nil(t, res)
			})
			t.Run("system to update is not present in the database", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					SystemIdentifier: &systemgrpc.SystemIdentifier{
						Type:       allowedSystemType,
						ExternalId: "externalID1",
					},
					Region:    allowedSystemRegion,
					LabelKeys: []string{"key1"},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrSystemNotFound, err)
				assert.Nil(t, res)
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

func registerRegionalSystem(t *testing.T, ctx context.Context, sSubj systemgrpc.ServiceClient, tenantID string, l1KeyClaim bool, systemType string, region, externalID *string) (string, string, string) {
	req := validRegisterSystemReq()
	req.TenantId = tenantID
	req.HasL1KeyClaim = l1KeyClaim
	req.Type = systemType
	if region != nil {
		req.Region = *region
	}
	if externalID != nil {
		req.ExternalId = *externalID
	}

	res, err := sSubj.RegisterSystem(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.Success)

	return req.GetExternalId(), req.GetType(), req.GetRegion()
}
