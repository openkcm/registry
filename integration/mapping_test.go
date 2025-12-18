//go:build integration
// +build integration

package integration_test

import (
	"testing"

	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	"github.com/openkcm/registry/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/status"
)

func TestMappingService(t *testing.T) {
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

	t.Run("UnmapSystemFromTenant", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("malformed request", func(t *testing.T) {
				tests := []struct {
					name string
					req  *mappinggrpc.UnmapSystemFromTenantRequest
					err  error
				}{
					{
						name: "missing externalID",
						err:  service.ErrValidationFailed,
						req: &mappinggrpc.UnmapSystemFromTenantRequest{
							TenantId: existingTenantID,
							Type:     allowedSystemType,
						},
					},
					{
						name: "missing type",
						err:  service.ErrValidationFailed,
						req: &mappinggrpc.UnmapSystemFromTenantRequest{
							TenantId:   existingTenantID,
							ExternalId: validRandID(),
						},
					},
					{
						name: "invalid type",
						err:  service.ErrValidationFailed,
						req: &mappinggrpc.UnmapSystemFromTenantRequest{
							TenantId:   existingTenantID,
							ExternalId: validRandID(),
							Type:       "INVALID_TYPE",
						},
					},
					{
						name: "missing tenantID",
						err:  service.ErrNoTenantID,
						req: &mappinggrpc.UnmapSystemFromTenantRequest{
							ExternalId: validRandID(),
							Type:       allowedSystemType,
						},
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						res, err := mSubj.UnmapSystemFromTenant(ctx, tt.req)
						assert.Error(t, err)
						assert.Nil(t, res)
						assert.Equal(t, status.Code(err), status.Code(tt.err))
					})
				}
			})
			t.Run("system not present in DB", func(t *testing.T) {
				res, err := mSubj.UnmapSystemFromTenant(ctx, &mappinggrpc.UnmapSystemFromTenantRequest{
					ExternalId: validRandID(),
					Type:       allowedSystemType,
					TenantId:   existingTenantID,
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.ErrorIs(t, err, service.ErrSystemNotFound)
			})
			t.Run("system is mapped to a different tenant", func(t *testing.T) {
				systemID, systemType, region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, systemID, existingTenantID, systemType, region, false)

				res, err := mSubj.UnmapSystemFromTenant(ctx, &mappinggrpc.UnmapSystemFromTenantRequest{
					ExternalId: systemID,
					Type:       systemType,
					TenantId:   validRandID(),
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, status.Code(err), status.Code(service.ErrSystemIsNotLinkedToTenant))
			})
			t.Run("regional system has active L1 key claim", func(t *testing.T) {
				systemID, systemType, region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, true, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, systemID, existingTenantID, systemType, region, true)
				res, err := mSubj.UnmapSystemFromTenant(ctx, &mappinggrpc.UnmapSystemFromTenantRequest{
					ExternalId: systemID,
					Type:       systemType,
					TenantId:   existingTenantID,
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, status.Code(err), status.Code(service.ErrSystemHasL1KeyClaim))
			})
		})
		t.Run("should unmap system from tenant successfully", func(t *testing.T) {
			systemID, systemType, region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)
			defer cleanupSystem(t, ctx, sSubj, mSubj, systemID, "", systemType, region, false)

			res, err := mSubj.UnmapSystemFromTenant(ctx, &mappinggrpc.UnmapSystemFromTenantRequest{
				ExternalId: systemID,
				Type:       systemType,
				TenantId:   existingTenantID,
			})
			assert.NoError(t, err)
			assert.NotNil(t, res)
		})
	})

	t.Run("MapSystemToTenant", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("malformed request", func(t *testing.T) {
				tests := []struct {
					name string
					req  *mappinggrpc.MapSystemToTenantRequest
					err  error
				}{
					{
						name: "missing externalID",
						err:  service.ErrValidationFailed,
						req: &mappinggrpc.MapSystemToTenantRequest{
							TenantId: existingTenantID,
							Type:     allowedSystemType,
						},
					},
					{
						name: "missing type",
						err:  service.ErrValidationFailed,
						req: &mappinggrpc.MapSystemToTenantRequest{
							TenantId:   existingTenantID,
							ExternalId: validRandID(),
						},
					},
					{
						name: "invalid type",
						err:  service.ErrValidationFailed,
						req: &mappinggrpc.MapSystemToTenantRequest{
							TenantId:   existingTenantID,
							ExternalId: validRandID(),
							Type:       "INVALID_TYPE",
						},
					},
					{
						name: "missing tenantID",
						err:  service.ErrNoTenantID,
						req: &mappinggrpc.MapSystemToTenantRequest{
							ExternalId: validRandID(),
							Type:       allowedSystemType,
						},
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						res, err := mSubj.MapSystemToTenant(ctx, tt.req)
						assert.Error(t, err)
						assert.Nil(t, res)
						assert.Equal(t, status.Code(err), status.Code(tt.err))
					})
				}
			})
			t.Run("tenant does not exist", func(t *testing.T) {
				res, err := mSubj.MapSystemToTenant(ctx, &mappinggrpc.MapSystemToTenantRequest{
					ExternalId: validRandID(),
					Type:       allowedSystemType,
					TenantId:   validRandID(),
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.ErrorIs(t, err, service.ErrTenantNotFound)
			})
			t.Run("system is already mapped to another tenant", func(t *testing.T) {
				tenant := validTenant()
				err = createTenantInDB(ctx, db, tenant)
				assert.NoError(t, err)

				systemID, systemType, region := registerRegionalSystem(t, ctx, sSubj, tenant.ID, false, allowedSystemType, nil, nil)
				defer func() {
					cleanupSystem(t, ctx, sSubj, mSubj, systemID, tenant.ID, systemType, region, false)
					assert.NoError(t, deleteTenantFromDB(ctx, db, tenant))
				}()
				res, err := mSubj.MapSystemToTenant(ctx, &mappinggrpc.MapSystemToTenantRequest{
					ExternalId: systemID,
					Type:       systemType,
					TenantId:   existingTenantID,
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.Equal(t, status.Code(err), status.Code(service.ErrSystemIsLinkedToTenant))
			})
		})
		t.Run("should map system to tenant successfully", func(t *testing.T) {
			t.Run("when L1 key claims is false", func(t *testing.T) {
				systemID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, systemID, existingTenantID, systemType, region, false)

				res, err := mSubj.MapSystemToTenant(ctx, &mappinggrpc.MapSystemToTenantRequest{
					ExternalId: systemID,
					Type:       systemType,
					TenantId:   existingTenantID,
				})
				assert.NoError(t, err)
				assert.NotNil(t, res)
			})
			t.Run("system not present in DB", func(t *testing.T) {
				externalID := validRandID()
				res, err := mSubj.MapSystemToTenant(ctx, &mappinggrpc.MapSystemToTenantRequest{
					ExternalId: externalID,
					Type:       allowedSystemType,
					TenantId:   existingTenantID,
				})
				assert.NoError(t, err)
				assert.NotNil(t, res)

				defer func() {
					res, err := mSubj.UnmapSystemFromTenant(ctx, &mappinggrpc.UnmapSystemFromTenantRequest{
						ExternalId: externalID,
						Type:       allowedSystemType,
						TenantId:   existingTenantID,
					})
					assert.NoError(t, err)
					assert.NotNil(t, res)
					assert.True(t, res.Success)

					err = deleteSystemInDB(ctx, db, externalID, allowedSystemType)
					assert.NoError(t, err)
				}()
				system, err := getSystemFromDB(ctx, db, externalID, allowedSystemType)
				assert.NoError(t, err)
				assert.NotNil(t, system)
				assert.Equal(t, existingTenantID, *system.TenantID)
			})
		})
	})

	t.Run("Get", func(t *testing.T) {
		t.Run("should return error if", func(t *testing.T) {
			t.Run("malformed request", func(t *testing.T) {
				tests := []struct {
					name string
					req  *mappinggrpc.GetRequest
					err  error
				}{
					{
						name: "missing externalID",
						err:  service.ErrValidationFailed,
						req: &mappinggrpc.GetRequest{
							Type: allowedSystemType,
						},
					},
					{
						name: "missing type",
						err:  service.ErrValidationFailed,
						req: &mappinggrpc.GetRequest{
							ExternalId: validRandID(),
						},
					},
					{
						name: "invalid type",
						err:  service.ErrValidationFailed,
						req: &mappinggrpc.GetRequest{
							ExternalId: validRandID(),
							Type:       "INVALID_TYPE",
						},
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						res, err := mSubj.Get(ctx, tt.req)
						assert.Error(t, err)
						assert.Nil(t, res)
						assert.Equal(t, status.Code(err), status.Code(tt.err))
					})
				}
			})
			t.Run("system not present in DB", func(t *testing.T) {
				res, err := mSubj.Get(ctx, &mappinggrpc.GetRequest{
					ExternalId: validRandID(),
					Type:       allowedSystemType,
				})
				assert.Error(t, err)
				assert.Nil(t, res)
				assert.ErrorIs(t, err, service.ErrSystemNotFound)
			})
		})
		t.Run("should get mapping successfully", func(t *testing.T) {
			t.Run("when system is mapped to tenant", func(t *testing.T) {
				systemID, systemType, region := registerRegionalSystem(t, ctx, sSubj, existingTenantID, false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, systemID, existingTenantID, systemType, region, false)

				res, err := mSubj.Get(ctx, &mappinggrpc.GetRequest{
					ExternalId: systemID,
					Type:       systemType,
				})
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.Equal(t, existingTenantID, res.TenantId)
			})
			t.Run("when system is not mapped to tenant", func(t *testing.T) {
				systemID, systemType, region := registerRegionalSystem(t, ctx, sSubj, "", false, allowedSystemType, nil, nil)
				defer cleanupSystem(t, ctx, sSubj, mSubj, systemID, "", systemType, region, false)

				res, err := mSubj.Get(ctx, &mappinggrpc.GetRequest{
					ExternalId: systemID,
					Type:       systemType,
				})
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.Equal(t, "", res.TenantId)
			})
		})
	})
}
