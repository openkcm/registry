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
)

func TestSystemService(t *testing.T) {
	// given
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer conn.Close()

	sSubj := systemgrpc.NewServiceClient(conn)
	ctx := t.Context()
	db, err := startDB()
	require.NoError(t, err)

	tenant := validTenant()
	err = createTenantInDB(ctx, db, tenant)
	assert.NoError(t, err)
	existingTenantID := tenant.ID.String()
	defer func() {
		err = deleteTenantFromDB(ctx, db, tenant)
		assert.NoError(t, err)
	}()

	t.Run("RegisterSystem", func(t *testing.T) {
		t.Run("should fail", func(t *testing.T) {
			t.Run("for empty system", func(t *testing.T) {
				// when
				result, err := sSubj.RegisterSystem(ctx, &systemgrpc.RegisterSystemRequest{})

				// then
				require.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
				assert.Contains(t, status.Convert(err).Message(), "external id is empty")
				assert.Nil(t, result)
			})
			t.Run("for invalid system type", func(t *testing.T) {
				// given
				req := validRegisterSystemReq()
				req.Type = "invalid-type"

				// when
				result, err := sSubj.RegisterSystem(ctx, req)

				// then
				require.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
				assert.Contains(t, status.Convert(err).Message(), "invalid field value: 'invalid-type' for field 'Type'")
				assert.Nil(t, result)
			})
			t.Run("if Tenant doesn't exist", func(t *testing.T) {
				// when
				req := validRegisterSystemReq()
				req.TenantId = validRandID()
				res, err := sSubj.RegisterSystem(ctx, req)

				// then
				require.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err))
				assert.Contains(t, status.Convert(err).Message(), "tenant not found")
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
			actSys, err := getSystem(t, ctx, sSubj, req.GetExternalId(), req.GetRegion())
			assert.NoError(t, err)
			assert.NotNil(t, actSys)
			assert.Equal(t, req.GetExternalId(), actSys.GetExternalId())
			assert.Equal(t, req.GetRegion(), actSys.GetRegion())
			assert.Equal(t, req.GetL2KeyId(), actSys.GetL2KeyId())
			assert.Equal(t, req.GetTenantId(), actSys.GetTenantId())
			assert.True(t, actSys.GetHasL1KeyClaim())
			assert.Equal(t, req.Labels, actSys.GetLabels())
		})
	})

	t.Run("DeleteSystem", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("tenant is already assigned", func(t *testing.T) {
				// given
				externalID, systemRegion := registerSystem(t, ctx, sSubj, existingTenantID, false)

				// when
				result, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: externalID,
					Region:     systemRegion,
				})

				// clean up
				defer cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, systemRegion, false)

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, result)
				actSys, err := listSystems(ctx, sSubj, existingTenantID)
				assert.NoError(t, err)
				assert.Len(t, actSys.GetSystems(), 1)
			})

			t.Run("system status unavailable", func(t *testing.T) {
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
					Region:     req.GetRegion(),
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
				actSys, err := getSystem(t, ctx, sSubj, req.GetExternalId(), req.GetRegion())
				assert.NoError(t, err)
				assert.NotNil(t, actSys)
			})
		})

		t.Run("should succeed if ", func(t *testing.T) {
			t.Run("system can be deleted", func(t *testing.T) {
				// given
				externalID, systemRegion := registerSystem(t, ctx, sSubj, "", false)

				// when
				result, err := sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: externalID,
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
					Region:     "EU",
				})

				// then
				assert.NoError(t, err)
				assert.True(t, res.Success)
			})
		})
	})

	t.Run("UpdateSystemL1KeyClaim", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("system is not present", func(t *testing.T) {
				// when
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: uuid.NewString(),
					Region:     "EU",
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
					ExternalId: req.GetExternalId(),
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
					ExternalId: req.GetExternalId(),
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
				externalID, region := registerSystem(t, ctx, sSubj, existingTenantID, false)
				defer cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, region, false)

				// when
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: externalID,
					Region:     region,
					L1KeyClaim: true,
				})
				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, res)
			})

			t.Run("L1KeyClaim is already false", func(t *testing.T) {
				// given
				externalID, region := registerSystem(t, ctx, sSubj, existingTenantID, false)
				defer cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, region, false)

				// when
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: externalID,
					Region:     region,
					TenantId:   existingTenantID,
					L1KeyClaim: false,
				})
				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, res)
			})

			t.Run("L1KeyClaim is already true", func(t *testing.T) {
				// given
				externalID, region := registerSystem(t, ctx, sSubj, existingTenantID, true)
				defer cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, region, true)

				// when
				res, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: externalID,
					Region:     region,
					TenantId:   existingTenantID,
					L1KeyClaim: true,
				})
				// then
				assert.Error(t, err)
				assert.Equal(t, codes.FailedPrecondition, status.Code(err), err.Error())
				assert.Nil(t, res)
			})
		})

		t.Run("should successfully update L1KeyClaim to", func(t *testing.T) {
			// given
			externalID, region := registerSystem(t, ctx, sSubj, existingTenantID, false)
			defer cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, region, false)

			t.Run("true then to false", func(t *testing.T) {
				// when
				result, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
					ExternalId: externalID,
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
			externalID1, region1 := registerSystemWithType(t, ctx, sSubj, existingTenantID, false, "test")
			externalID2, region2 := registerSystem(t, ctx, sSubj, "", false)

			// clean up
			defer func() {
				cleanupSystem(t, ctx, sSubj, externalID1, existingTenantID, region1, false)
				cleanupSystem(t, ctx, sSubj, externalID2, "", region2, false)
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
						name: "ID",
						request: &systemgrpc.ListSystemsRequest{
							ExternalId: externalID1,
							Region:     region1,
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
							TenantId: existingTenantID,
							Type:     "test",
						},
						expectedExternalID: externalID1,
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
						name:      "no tenantID and no externalID and region is provided in query",
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
		// given

		req1 := validRegisterSystemReq()
		req1.L2KeyId = "l1"
		req1.TenantId = existingTenantID
		res1, err := sSubj.RegisterSystem(ctx, req1)
		assert.NoError(t, err)
		assert.NotNil(t, res1)
		assert.True(t, res1.Success)

		defer func() {
			assert.NoError(t, unlinkSystemFromTenant(ctx, sSubj, req1.GetExternalId(), req1.GetRegion()))
			assert.NoError(t, deleteSystem(ctx, sSubj, req1.GetExternalId(), req1.GetRegion()))
		}()

		req2 := validRegisterSystemReq()
		req2.L2KeyId = "l2"
		req2.TenantId = existingTenantID
		res2, err := sSubj.RegisterSystem(ctx, req2)
		assert.NoError(t, err)
		assert.NotNil(t, res2)
		assert.True(t, res2.Success)
		assert.NoError(t, err)

		defer func() {
			assert.NoError(t, unlinkSystemFromTenant(ctx, sSubj, req2.GetExternalId(), req2.GetRegion()))
			assert.NoError(t, deleteSystem(ctx, sSubj, req2.GetExternalId(), req2.GetRegion()))
		}()

		t.Run("should return next page token with applied limit", func(t *testing.T) {
			req := &systemgrpc.ListSystemsRequest{
				Limit:    1,
				TenantId: existingTenantID,
			}
			res, _ := sSubj.ListSystems(ctx, req)
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
				expErr := model.ErrExternalIDIsEmpty.Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
			})
			t.Run("region in system Identifier is empty", func(t *testing.T) {
				// when
				res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{
							ExternalId: "externalID",
						},
					},
				})

				// then
				expErr := model.ErrRegionIsEmpty.Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
			})
			t.Run("system to update is not present in the database", func(t *testing.T) {
				// when
				id := uuid.NewString()
				region := "EU"
				res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{
							ExternalId: id,
							Region:     region,
						},
					},
				})

				// then
				expErr := service.ErrorWithParams(service.ErrSystemNotFound, "externalID", id, "region", region).Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
			})

			t.Run("one of the systems has active L1 key claim and rollback transaction", func(t *testing.T) {
				// given
				// system2 has active L1 key claim
				sys1ExternalID, sys1Region := registerSystem(t, ctx, sSubj, existingTenantID, false)
				sys2ExternalID, sys2Region := registerSystem(t, ctx, sSubj, existingTenantID, true)

				// clean up
				defer func() {
					cleanupSystem(t, ctx, sSubj, sys1ExternalID, existingTenantID, sys1Region, false)
					cleanupSystem(t, ctx, sSubj, sys2ExternalID, existingTenantID, sys2Region, true)
				}()

				// when
				res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{
							ExternalId: sys1ExternalID,
							Region:     sys1Region,
						},
						{
							ExternalId: sys2ExternalID,
							Region:     sys2Region,
						},
					},
				})

				// then
				expErr := service.ErrorWithParams(service.ErrSystemHasL1KeyClaim, "externalID", sys2ExternalID, "region", sys2Region).Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
				actSys, err := listSystems(ctx, sSubj, existingTenantID)
				assert.NoError(t, err)
				assert.Len(t, actSys.GetSystems(), 2)
			})

			t.Run("one of the systems is not linked to the tenant and rollback transaction", func(t *testing.T) {
				// given: system2 is not linked to the tenant
				systemID1, region1 := registerSystem(t, ctx, sSubj, existingTenantID, false)
				defer cleanupSystem(t, ctx, sSubj, systemID1, existingTenantID, region1, false)

				systemID2, region2 := registerSystem(t, ctx, sSubj, "", false)
				defer cleanupSystem(t, ctx, sSubj, systemID2, "", region2, false)

				// when
				res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{
							ExternalId: systemID1,
							Region:     region1,
						}, {
							ExternalId: systemID2,
							Region:     region2,
						},
					},
				})
				// then
				expErr := service.ErrorWithParams(service.ErrSystemIsNotLinkedToTenant, "externalID", systemID2, "region", region2).Error()
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
			sys1ExternalID, sys1Region := registerSystem(t, ctx, sSubj, existingTenantID, false)
			sys2ExternalID, sys2Region := registerSystem(t, ctx, sSubj, existingTenantID, false)

			// clean up
			defer func() {
				err = deleteSystem(ctx, sSubj, sys1ExternalID, sys1Region)
				assert.NoError(t, err)
				err = deleteSystem(ctx, sSubj, sys2ExternalID, sys2Region)
				assert.NoError(t, err)
			}()

			// when
			res, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
				SystemIdentifiers: []*systemgrpc.SystemIdentifier{
					{
						ExternalId: sys1ExternalID,
						Region:     sys1Region,
					},
					{
						ExternalId: sys2ExternalID,
						Region:     sys1Region,
					},
				},
			})

			// then
			assert.NoError(t, err)
			assert.True(t, res.GetSuccess())
			actSys1, err := getSystem(t, ctx, sSubj, sys1ExternalID, sys1Region)
			assert.NoError(t, err)
			assert.NotNil(t, actSys1)
			actSys2, err := getSystem(t, ctx, sSubj, sys2ExternalID, sys2Region)
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
						{ExternalId: uuid.NewString(), Region: "region1"},
						{ExternalId: "", Region: "region2"},
						{ExternalId: uuid.NewString(), Region: "region3"},
					},
				})

				// then
				expErr := model.ErrExternalIDIsEmpty.Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
			})

			t.Run("Tenant does not exist", func(t *testing.T) {
				// given
				externalID, region := registerSystem(t, ctx, sSubj, "", false)

				tenantID := validRandID()

				// clean up
				defer func() {
					err := deleteSystem(ctx, sSubj, externalID, region)
					assert.NoError(t, err)
				}()
				// when
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: externalID, Region: region},
					},
					TenantId: tenantID,
				})

				// then
				assert.Error(t, err)
				assert.Equal(t, codes.NotFound, status.Code(err), err.Error())
				assert.Nil(t, res)
			})

			t.Run("system to update is not present in the database", func(t *testing.T) {
				// given
				systemID := uuid.NewString()
				region := "region1"
				// when
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: systemID, Region: region},
					},
					TenantId: existingTenantID,
				})

				// then
				expErr := service.ErrorWithParams(service.ErrSystemNotFound, "externalID", systemID, "region", region).Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
			})

			t.Run("system has active L1KeyClaim", func(t *testing.T) {
				// given
				externalID, region := registerSystem(t, ctx, sSubj, "", true)

				// clean up
				defer func() {
					err := deleteSystem(ctx, sSubj, externalID, region)
					assert.NoError(t, err)
				}()

				// when
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: externalID, Region: region},
					},
					TenantId: existingTenantID,
				})

				// then
				expErr := service.ErrorWithParams(service.ErrSystemHasL1KeyClaim, "externalID", externalID, "region", region).Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
			})

			t.Run("system already has linked tenant", func(t *testing.T) {
				// given
				externalID, region := registerSystem(t, ctx, sSubj, existingTenantID, false)

				tenant := validTenant()
				err := createTenantInDB(ctx, db, tenant)
				assert.NoError(t, err)

				// clean up
				defer func() {
					cleanupSystem(t, ctx, sSubj, externalID, existingTenantID, region, false)
					err = deleteTenantFromDB(ctx, db, tenant)
					assert.NoError(t, err)
				}()

				// when
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: externalID, Region: region},
					},
					TenantId: tenant.ID.String(),
				})

				// then
				expErr := service.ErrorWithParams(service.ErrSystemIsLinkedToTenant, "externalID", externalID, "region", region).Error()
				assert.Error(t, err)
				assert.Equal(t, expErr, err.Error())
				assert.Nil(t, res)
			})

			t.Run("should rollback transaction if any system fails to link", func(t *testing.T) {
				// given
				// system1 is already linked to the tenant
				sys1ExternalID, sys1Region := registerSystem(t, ctx, sSubj, existingTenantID, false)

				sys2ExternalID, sys2Region := registerSystem(t, ctx, sSubj, "", false)

				defer func() {
					cleanupSystem(t, ctx, sSubj, sys1ExternalID, existingTenantID, sys1Region, false)
					err = deleteSystem(ctx, sSubj, sys2ExternalID, sys2Region)
					assert.NoError(t, err)
				}()

				// when
				tenantID := existingTenantID
				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: sys1ExternalID, Region: sys1Region},
						{ExternalId: sys2ExternalID, Region: sys2Region},
					},
					TenantId: tenantID,
				})

				// then
				expErr := service.ErrorWithParams(service.ErrSystemIsLinkedToTenant, "externalID", sys1ExternalID, "region", sys1Region).Error()
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
				externalID, systemRegion := registerSystem(t, ctx, sSubj, "", false)
				// when
				tenantID := existingTenantID

				// clean up
				defer cleanupSystem(t, ctx, sSubj, externalID, tenantID, systemRegion, false)

				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: externalID, Region: systemRegion},
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

			t.Run("linking multiple systems in one transaction", func(t *testing.T) {
				// given
				sys1ExternalID, sys1Region := registerSystem(t, ctx, sSubj, "", false)
				sys2ExternalID, sys2Region := registerSystem(t, ctx, sSubj, "", false)
				tenantID := existingTenantID
				defer func() {
					cleanupSystem(t, ctx, sSubj, sys1ExternalID, tenantID, sys1Region, false)
					cleanupSystem(t, ctx, sSubj, sys2ExternalID, tenantID, sys2Region, false)
				}()

				res, err := sSubj.LinkSystemsToTenant(ctx, &systemgrpc.LinkSystemsToTenantRequest{
					SystemIdentifiers: []*systemgrpc.SystemIdentifier{
						{ExternalId: sys1ExternalID, Region: sys1Region},
						{ExternalId: sys2ExternalID, Region: sys2Region},
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
				externalID, systemRegion := registerSystem(t, ctx, sSubj, "", false)

				defer func() {
					err := deleteSystem(ctx, sSubj, externalID, systemRegion)
					assert.NoError(t, err)
				}()

				// when
				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					ExternalId: externalID,
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
			t.Run("external ID is in empty", func(t *testing.T) {
				// when
				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					ExternalId: "",
					Region:     "region1",
					Status:     typespb.Status_STATUS_AVAILABLE,
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, model.ErrExternalIDIsEmpty, err)
				assert.Nil(t, res)
			})

			t.Run("system to update is not present in the database", func(t *testing.T) {
				// when
				id := uuid.NewString()
				region := "region1"
				res, err := sSubj.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
					ExternalId: id,
					Region:     region,
					Status:     typespb.Status_STATUS_AVAILABLE,
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrSystemNotFound, err)
				assert.Nil(t, res)
			})
		})
	})

	t.Run("SetSystemLabels", func(t *testing.T) {
		t.Run("should succeed if", func(t *testing.T) {
			t.Run("request is valid", func(t *testing.T) {
				// given
				externalID, systemRegion := registerSystem(t, ctx, sSubj, "", false)

				defer func() {
					err := deleteSystem(ctx, sSubj, externalID, systemRegion)
					assert.NoError(t, err)
				}()

				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
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
				actSys, err := getSystem(t, ctx, sSubj, externalID, systemRegion)
				assert.NoError(t, err)
				assert.NotNil(t, actSys)
				assert.Equal(t, 3, len(actSys.GetLabels()))
				assert.Equal(t, "value12", actSys.GetLabels()["key1"])
				assert.Equal(t, "value2", actSys.GetLabels()["key2"])
				assert.Equal(t, "value3", actSys.GetLabels()["key3"])
			})
		})

		t.Run("should return error if", func(t *testing.T) {
			t.Run("external ID is empty", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: "",
					Region:     "region1",
					Labels: map[string]string{
						"key1": "value1",
					},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, model.ErrExternalIDIsEmpty, err)
				assert.Nil(t, res)
			})
			t.Run("region is empty", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: "externalID1",
					Region:     "",
					Labels: map[string]string{
						"key1": "value1",
					},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, model.ErrRegionIsEmpty, err)
				assert.Nil(t, res)
			})
			t.Run("labels are empty", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: "externalID1",
					Region:     "region1",
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrMissingLabels, err)
				assert.Nil(t, res)
			})
			t.Run("labels keys are empty", func(t *testing.T) {
				// when
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: "externalID1",
					Region:     "region1",
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
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: "externalID1",
					Region:     "region1",
					Labels: map[string]string{
						"key1": "",
					},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, model.ErrLabelsIncludeEmptyString, err)
				assert.Nil(t, res)
			})
			t.Run("system to update is not present in the database", func(t *testing.T) {
				// when
				id := uuid.NewString()
				region := "region1"
				res, err := sSubj.SetSystemLabels(ctx, &systemgrpc.SetSystemLabelsRequest{
					ExternalId: id,
					Region:     region,
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
				externalID, systemRegion := registerSystem(t, ctx, sSubj, "", false)

				defer func() {
					err := deleteSystem(ctx, sSubj, externalID, systemRegion)
					assert.NoError(t, err)
				}()

				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: externalID,
					Region:     systemRegion,
					LabelKeys:  []string{"key1"},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
				actSys, err := getSystem(t, ctx, sSubj, externalID, systemRegion)
				assert.NoError(t, err)
				assert.NotNil(t, actSys)
				assert.Equal(t, 1, len(actSys.GetLabels()))
				assert.Equal(t, "", actSys.GetLabels()["key1"])
			})
			t.Run("label does not exist", func(t *testing.T) {
				// given
				externalID, systemRegion := registerSystem(t, ctx, sSubj, "", false)

				defer func() {
					err := deleteSystem(ctx, sSubj, externalID, systemRegion)
					assert.NoError(t, err)
				}()

				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: externalID,
					Region:     systemRegion,
					LabelKeys:  []string{"key3"},
				})

				// then
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.True(t, res.GetSuccess())
				actSys, err := getSystem(t, ctx, sSubj, externalID, systemRegion)
				assert.NoError(t, err)
				assert.NotNil(t, actSys)
				assert.Equal(t, 2, len(actSys.GetLabels()))
			})
		})

		t.Run("should return error if", func(t *testing.T) {
			t.Run("external ID is empty", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: "",
					Region:     "region1",
					LabelKeys:  []string{"key1"},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, model.ErrExternalIDIsEmpty, err)
				assert.Nil(t, res)
			})
			t.Run("region is empty", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: "externalID1",
					Region:     "",
					LabelKeys:  []string{"key1"},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, model.ErrRegionIsEmpty, err)
				assert.Nil(t, res)
			})
			t.Run("labels keys are empty", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: "externalID1",
					Region:     "region1",
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrMissingLabelKeys, err)
				assert.Nil(t, res)
			})
			t.Run("labels keys have empty value", func(t *testing.T) {
				// when
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: "externalID1",
					Region:     "region1",
					LabelKeys:  []string{""},
				})

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, service.ErrEmptyLabelKeys, err)
				assert.Nil(t, res)
			})
			t.Run("system to update is not present in the database", func(t *testing.T) {
				// when
				id := uuid.NewString()
				region := "region1"
				res, err := sSubj.RemoveSystemLabels(ctx, &systemgrpc.RemoveSystemLabelsRequest{
					ExternalId: id,
					Region:     region,
					LabelKeys:  []string{"key1"},
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

func getSystem(t *testing.T, ctx context.Context, subj systemgrpc.ServiceClient, externalID, region string) (*systemgrpc.System, error) {
	req := &systemgrpc.ListSystemsRequest{
		ExternalId: externalID,
		Region:     region,
	}
	resp, err := subj.ListSystems(ctx, req)
	if err != nil {
		return nil, err
	}
	assert.Len(t, resp.GetSystems(), 1, "Expected exactly one system to be returned")

	return resp.GetSystems()[0], nil
}

func registerSystem(t *testing.T, ctx context.Context, sSubj systemgrpc.ServiceClient, tenantID string, l1KeyClaim bool) (string, string) {
	return registerSystemWithType(t, ctx, sSubj, tenantID, l1KeyClaim, "system")
}

func registerSystemWithType(t *testing.T, ctx context.Context, sSubj systemgrpc.ServiceClient, tenantID string, l1KeyClaim bool, systemType string) (string, string) {
	req := validRegisterSystemReq()
	req.TenantId = tenantID
	req.HasL1KeyClaim = l1KeyClaim
	req.Type = systemType
	res, err := sSubj.RegisterSystem(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.Success)

	return req.GetExternalId(), req.GetRegion()
}

func cleanupSystem(t *testing.T, ctx context.Context, sSubj systemgrpc.ServiceClient, externalID string, tenantID string, region string, l1KeyClaim bool) {
	if l1KeyClaim {
		_, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
			ExternalId: externalID,
			Region:     region,
			TenantId:   tenantID,
			L1KeyClaim: false,
		})
		assert.NoError(t, err)
	}
	if tenantID != "" {
		_, err := sSubj.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
			SystemIdentifiers: []*systemgrpc.SystemIdentifier{
				{
					ExternalId: externalID,
					Region:     region,
				},
			},
		})
		assert.NoError(t, err)
	}
	err := deleteSystem(ctx, sSubj, externalID, region)
	assert.NoError(t, err)
}
