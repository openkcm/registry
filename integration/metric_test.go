//go:build integration
// +build integration

package integration_test

import (
	"context"
	"errors"
	"testing"

	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/service"
)

// These regions act as identifiers for metric tests, preventing other tests from interfering.
const (
	tenantMetricsRegion = "region-tenant"
	systemMetricsRegion = "region-system"
)

func TestTenantMetrics(t *testing.T) {
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer conn.Close()

	subj := tenantgrpc.NewServiceClient(conn)

	scraper, err := newMetricScraper()
	require.NoError(t, err)

	ctx := t.Context()
	db, err := startDB()
	require.NoError(t, err)

	err = initTenantMetrics(ctx, subj, db)
	assert.NoError(t, err)

	t.Run("Register counter", func(t *testing.T) {
		metric := createMetric(t, "tenants_registered", "region", tenantMetricsRegion)

		t.Run("increase", func(t *testing.T) {
			t.Run("should happen for registered tenants", func(t *testing.T) {
				// Given
				totalBefore, err := getSafeMetric(ctx, scraper, metric)
				assert.NoError(t, err)
				req := validRegisterTenantReq()
				req.Region = tenantMetricsRegion

				// When
				_, err = subj.RegisterTenant(ctx, req)
				assert.NoError(t, err)

				defer func() {
					err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req.GetId()})
					assert.NoError(t, err)
				}()

				// Then
				totalAfter, err := getSafeMetric(ctx, scraper, metric)
				assert.NoError(t, err)
				assert.Equal(t, totalBefore+1, totalAfter)
			})

			t.Run("should not happen if error occurs", func(t *testing.T) {
				// Given
				totalBefore, err := getSafeMetric(ctx, scraper, metric)
				assert.NoError(t, err)

				// When
				req := validRegisterTenantReq()
				req.OwnerId = ""
				req.Region = tenantMetricsRegion
				_, err = subj.RegisterTenant(ctx, req)
				assert.Error(t, err)

				// Then
				totalAfter, err := getSafeMetric(ctx, scraper, metric)
				assert.NoError(t, err)
				assert.Equal(t, totalBefore, totalAfter)
			})
		})
	})

	t.Run("Status gauge", func(t *testing.T) {
		metricProvisioningTenants := createMetric(t, "tenants_count", "region", tenantMetricsRegion, "status", tenantgrpc.Status_STATUS_PROVISIONING.String())

		t.Run("increase", func(t *testing.T) {
			t.Run("should happen for registered tenants", func(t *testing.T) {
				// Given
				activeBefore, err := getSafeMetric(ctx, scraper, metricProvisioningTenants)
				assert.NoError(t, err)
				req := validRegisterTenantReq()
				req.Region = tenantMetricsRegion

				// When
				_, err = subj.RegisterTenant(ctx, req)
				assert.NoError(t, err)

				defer func() {
					err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req.GetId()})
					assert.NoError(t, err)
				}()

				// Then
				assert.NoError(t, err)
				assertCounterInc(t, scraper, ctx, metricProvisioningTenants, activeBefore)
			})

			t.Run("should not happen if error occurs", func(t *testing.T) {
				// Given
				activeBefore, err := getSafeMetric(ctx, scraper, metricProvisioningTenants)
				assert.NoError(t, err)
				req := validRegisterTenantReq()
				req.OwnerId = ""
				req.Region = tenantMetricsRegion

				// When
				_, err = subj.RegisterTenant(ctx, req)
				assert.Error(t, err)

				// Then
				assertCounterEqual(t, scraper, ctx, metricProvisioningTenants, activeBefore)
			})
		})

		t.Run("update", func(t *testing.T) {
			metricActiveTenants := createMetric(t, "tenants_count", "region", tenantMetricsRegion, "status", tenantgrpc.Status_STATUS_ACTIVE.String())
			metricBlockingTenants := createMetric(t, "tenants_count", "region", tenantMetricsRegion, "status", tenantgrpc.Status_STATUS_BLOCKING.String())

			t.Run("should happen if status changes", func(t *testing.T) {
				// Given
				tenant := validTenant()
				tenant.Region = tenantMetricsRegion
				err = createTenantInDB(ctx, db, tenant)
				assert.NoError(t, err)
				defer func() {
					err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: tenant.ID})
					assert.NoError(t, err)
				}()

				activeBefore, err := getSafeMetric(ctx, scraper, metricActiveTenants)
				assert.NoError(t, err)
				blockingBefore, err := getSafeMetric(ctx, scraper, metricBlockingTenants)
				assert.NoError(t, err)

				// When
				_, err = subj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
					Id: tenant.ID,
				})

				// Then
				assert.NoError(t, err)
				assertCounterDec(t, scraper, ctx, metricActiveTenants, activeBefore)
				assertCounterInc(t, scraper, ctx, metricBlockingTenants, blockingBefore)
			})

			t.Run("should not happen if error occurs", func(t *testing.T) {
				// Given
				req := validRegisterTenantReq()
				req.Region = tenantMetricsRegion
				_, err := subj.RegisterTenant(ctx, req)
				assert.NoError(t, err)
				defer func() {
					err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req.GetId()})
					assert.NoError(t, err)
				}()

				activeBefore, err := getSafeMetric(ctx, scraper, metricActiveTenants)
				assert.NoError(t, err)
				blockingBefore, err := getSafeMetric(ctx, scraper, metricBlockingTenants)
				assert.NoError(t, err)

				// When
				_, err = subj.BlockTenant(ctx, &tenantgrpc.BlockTenantRequest{
					Id: "",
				})
				assert.Error(t, err)

				// Then
				assertCounterEqual(t, scraper, ctx, metricActiveTenants, activeBefore)
				assertCounterEqual(t, scraper, ctx, metricBlockingTenants, blockingBefore)
			})
		})
	})
}

func TestSystemMetrics(t *testing.T) {
	conn, err := newGRPCClientConn()
	require.NoError(t, err)
	defer conn.Close()

	sSubj := systemgrpc.NewServiceClient(conn)
	mSub := mappinggrpc.NewServiceClient(conn)

	scraper, err := newMetricScraper()
	require.NoError(t, err)

	ctx := t.Context()
	db, err := startDB()
	require.NoError(t, err)

	err = initSystemMetrics(ctx, sSubj)
	assert.NoError(t, err)

	t.Run("Register counter", func(t *testing.T) {
		metricSystems := createMetric(t, "systems_registered", "region", systemMetricsRegion)

		t.Run("increase", func(t *testing.T) {
			t.Run("should happen for registered systems", func(t *testing.T) {
				// Given
				systemsBefore, err := getSafeMetric(ctx, scraper, metricSystems)
				assert.NoError(t, err)
				req := validRegisterSystemReq()
				req.Region = systemMetricsRegion

				// When
				resp, err := sSubj.RegisterSystem(ctx, req)
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.True(t, resp.Success)

				defer cleanupSystem(t, ctx, sSubj, mSub, req.ExternalId, req.TenantId, req.Type, req.Region, req.HasL1KeyClaim)

				// Then
				assertCounterInc(t, scraper, ctx, metricSystems, systemsBefore)
			})

			t.Run("should not happen if error occurs", func(t *testing.T) {
				// Given
				systemsBefore, err := getSafeMetric(ctx, scraper, metricSystems)
				assert.NoError(t, err)
				req := validRegisterSystemReq()
				req.ExternalId = ""
				req.Region = systemMetricsRegion

				// When
				_, err = sSubj.RegisterSystem(ctx, req)
				assert.Error(t, err)

				// Then
				assertCounterEqual(t, scraper, ctx, metricSystems, systemsBefore)
			})
		})
	})

	t.Run("Delete counter", func(t *testing.T) {
		metricDeletedSystems := createMetric(t, "systems_deleted", "region", systemMetricsRegion)

		t.Run("increase", func(t *testing.T) {
			t.Run("should happen for deleted systems", func(t *testing.T) {
				// Given
				req := validRegisterSystemReq()
				req.Region = systemMetricsRegion
				_, err := sSubj.RegisterSystem(ctx, req)
				assert.NoError(t, err)

				deletedSystemsBefore, err := getSafeMetric(ctx, scraper, metricDeletedSystems)
				assert.NoError(t, err)

				// When
				_, err = sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: req.ExternalId,
					Type:       req.Type,
					Region:     req.Region,
				})
				assert.NoError(t, err)

				// Then
				assertCounterInc(t, scraper, ctx, metricDeletedSystems, deletedSystemsBefore)
			})

			t.Run("should not happen if error occurs", func(t *testing.T) {
				// Given
				deletedSystemsBefore, err := getSafeMetric(ctx, scraper, metricDeletedSystems)
				assert.NoError(t, err)

				// When
				_, err = sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{})
				assert.Error(t, err)

				// Then
				assertCounterEqual(t, scraper, ctx, metricDeletedSystems, deletedSystemsBefore)
			})
		})
	})

	t.Run("Link status gauge", func(t *testing.T) {
		// Given
		metricLinked := createMetric(
			t,
			"systems_count",
			service.AttrTenantLinked, "true",
		)

		metricUnlinked := createMetric(
			t,
			"systems_count",
			service.AttrTenantLinked, "false",
		)

		tenant := validTenant()
		err = createTenantInDB(ctx, db, tenant)
		assert.NoError(t, err)

		assert.NoError(t, err)
		defer func() {
			err = deleteTenantFromDB(ctx, db, tenant)
			assert.NoError(t, err)
		}()

		t.Run("increase", func(t *testing.T) {
			t.Run("should happen", func(t *testing.T) {
				t.Run("for registered systems without linked tenant", func(t *testing.T) {
					// Given
					req := validRegisterSystemReq()
					req.Region = systemMetricsRegion

					// When
					_, err := sSubj.RegisterSystem(ctx, req)
					assert.NoError(t, err)
					defer cleanupSystem(t, ctx, sSubj, mSub, req.ExternalId, req.TenantId, req.Type, req.Region, req.HasL1KeyClaim)

					// Then
					agg, err := getSafeMetric(ctx, scraper, metricUnlinked)
					assert.NoError(t, err)
					assert.Equal(t, 1, agg)
				})

				t.Run("for registered systems with linked tenant", func(t *testing.T) {
					// Given
					req := validRegisterSystemReq()
					req.Region = systemMetricsRegion
					req.TenantId = tenant.ID

					// When
					_, err := sSubj.RegisterSystem(ctx, req)
					assert.NoError(t, err)
					defer cleanupSystem(t, ctx, sSubj, mSub, req.ExternalId, req.TenantId, req.Type, req.Region, req.HasL1KeyClaim)

					// Then
					agg, err := getSafeMetric(ctx, scraper, metricLinked)
					assert.NoError(t, err)
					assert.Equal(t, 1, agg)
				})
			})

			t.Run("should not happen", func(t *testing.T) {
				t.Run("for registered systems without linked tenant if error occurs", func(t *testing.T) {
					// Given
					req := validRegisterSystemReq()
					req.Region = ""

					// When
					_, err := sSubj.RegisterSystem(ctx, req)
					assert.Error(t, err)

					// Then
					unlinked, err := getSafeMetric(ctx, scraper, metricUnlinked)
					assert.NoError(t, err)
					assert.Equal(t, 0, unlinked)
				})

				t.Run("for registered systems with linked tenant if error occurs", func(t *testing.T) {
					// Given
					req := validRegisterSystemReq()
					req.Region = ""
					req.TenantId = tenant.ID

					// When
					_, err := sSubj.RegisterSystem(ctx, req)
					assert.Error(t, err)

					// Then
					linked, err := getSafeMetric(ctx, scraper, metricLinked)
					assert.NoError(t, err)
					assert.Equal(t, 0, linked)
				})
			})
		})

		t.Run("update", func(t *testing.T) {
			t.Run("should happen", func(t *testing.T) {
				t.Run("for linking", func(t *testing.T) {
					// When
					externalID := validRandID()
					_, err = mSub.MapSystemToTenant(ctx, &mappinggrpc.MapSystemToTenantRequest{
						ExternalId: externalID,
						Type:       allowedSystemType,
						TenantId:   tenant.ID,
					})

					assert.NoError(t, err)
					defer func() {
						res, err := mSub.UnmapSystemFromTenant(ctx, &mappinggrpc.UnmapSystemFromTenantRequest{
							ExternalId: externalID,
							Type:       allowedSystemType,
							TenantId:   tenant.ID,
						})
						assert.NoError(t, err)
						assert.True(t, res.Success)

						assert.NoError(t, deleteSystemInDB(ctx, db, externalID, allowedSystemType))
					}()

					// Then
					linked, err := getSafeMetric(ctx, scraper, metricLinked)
					assert.NoError(t, err)
					assert.Equal(t, 1, linked)
					unlinked, err := getSafeMetric(ctx, scraper, metricUnlinked)
					assert.NoError(t, err)
					assert.Equal(t, 0, unlinked)
				})

				t.Run("for unlinking", func(t *testing.T) {
					// Given
					req := validRegisterSystemReq()
					req.Region = systemMetricsRegion
					req.TenantId = tenant.ID
					_, err := sSubj.RegisterSystem(ctx, req)
					assert.NoError(t, err)

					// When
					_, err = mSub.UnmapSystemFromTenant(ctx, &mappinggrpc.UnmapSystemFromTenantRequest{
						ExternalId: req.ExternalId,
						Type:       req.Type,
						TenantId:   req.TenantId,
					})

					assert.NoError(t, err)
					defer cleanupSystem(t, ctx, sSubj, mSub, req.ExternalId, "", req.Type, req.Region, req.HasL1KeyClaim)

					// Then
					linked, err := getSafeMetric(ctx, scraper, metricLinked)
					assert.NoError(t, err)
					assert.Equal(t, 0, linked)
					unlinked, err := getSafeMetric(ctx, scraper, metricUnlinked)
					assert.NoError(t, err)
					assert.Equal(t, 1, unlinked)
				})
			})

			t.Run("should not happen", func(t *testing.T) {
				t.Run("for linking if error occurs", func(t *testing.T) {
					// Given
					req := validRegisterSystemReq()
					req.Region = systemMetricsRegion
					_, err := sSubj.RegisterSystem(ctx, req)
					assert.NoError(t, err)
					defer cleanupSystem(t, ctx, sSubj, mSub, req.ExternalId, "", req.Type, req.Region, req.HasL1KeyClaim)

					// When
					_, err = mSub.MapSystemToTenant(ctx, &mappinggrpc.MapSystemToTenantRequest{
						Type:     req.Type,
						TenantId: tenant.ID,
					})
					assert.Error(t, err)

					// Then
					linked, err := getSafeMetric(ctx, scraper, metricLinked)
					assert.NoError(t, err)
					assert.Equal(t, 0, linked)
					unlinked, err := getSafeMetric(ctx, scraper, metricUnlinked)
					assert.NoError(t, err)
					assert.Equal(t, 1, unlinked)
				})

				t.Run("for unlinking if error occurs", func(t *testing.T) {
					// Given
					req := validRegisterSystemReq()
					req.TenantId = tenant.ID
					req.Region = systemMetricsRegion

					_, err := sSubj.RegisterSystem(ctx, req)
					assert.NoError(t, err)
					defer cleanupSystem(t, ctx, sSubj, mSub, req.ExternalId, req.TenantId, req.Type, req.Region, req.HasL1KeyClaim)

					// When
					_, err = mSub.UnmapSystemFromTenant(ctx, &mappinggrpc.UnmapSystemFromTenantRequest{})
					assert.Error(t, err)

					// Then
					linked, err := getSafeMetric(ctx, scraper, metricLinked)
					assert.NoError(t, err)
					assert.Equal(t, 1, linked)
					unlinked, err := getSafeMetric(ctx, scraper, metricUnlinked)
					assert.NoError(t, err)
					assert.Equal(t, 0, unlinked)
				})
			})
		})

		t.Run("decrease", func(t *testing.T) {
			t.Run("should happen for deleted systems", func(t *testing.T) {
				// Given
				req := validRegisterSystemReq()
				req.Region = systemMetricsRegion
				_, err := sSubj.RegisterSystem(ctx, req)
				assert.NoError(t, err)

				// When
				_, err = sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
					ExternalId: req.ExternalId,
					Type:       req.Type,
					Region:     req.Region,
				})

				// Then
				unlinked, err := getSafeMetric(ctx, scraper, metricUnlinked)
				assert.NoError(t, err)
				assert.Equal(t, 0, unlinked)
			})

			t.Run("should not happen if error occurs", func(t *testing.T) {
				// When
				_, err = sSubj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{})
				assert.Error(t, err)

				// Then
				unlinked, err := getSafeMetric(ctx, scraper, metricUnlinked)
				assert.NoError(t, err)
				assert.Equal(t, 0, unlinked)
			})
		})
	})
}

// initTenantMetrics initializes metrics specific to tenants,
// ensuring they are available prior to executing the tests.
func initTenantMetrics(
	ctx context.Context,
	subj tenantgrpc.ServiceClient,
	db *gorm.DB,
) error {
	req := validRegisterTenantReq()
	req.Region = tenantMetricsRegion
	_, err := subj.RegisterTenant(ctx, req)
	if err != nil {
		return err
	}

	defer func() {
		err = deleteTenantFromDB(ctx, db, &model.Tenant{ID: req.GetId()})
	}()

	return err
}

// initSystemMetrics initializes metrics specific to systems,
// ensuring they are available prior to executing the tests.
func initSystemMetrics(
	ctx context.Context,
	subj systemgrpc.ServiceClient,
) error {
	req := validRegisterSystemReq()
	req.Region = systemMetricsRegion
	_, err := subj.RegisterSystem(ctx, req)
	if err != nil {
		return err
	}

	_, err = subj.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
		ExternalId: req.ExternalId,
		Type:       allowedSystemType,
		Region:     req.Region,
	})
	if err != nil {
		return err
	}

	return nil
}

func assertCounterEqual(t *testing.T, scraper *metricScraper, ctx context.Context, m metric, before int) {
	t.Helper()
	after, err := getSafeMetric(ctx, scraper, m)
	assert.NoError(t, err)
	assert.Equal(t, before, after)
}

func assertCounterInc(t *testing.T, scraper *metricScraper, ctx context.Context, m metric, before int) {
	t.Helper()
	after, err := getSafeMetric(ctx, scraper, m)
	assert.NoError(t, err)
	assert.Equal(t, before+1, after)
}

func assertCounterDec(t *testing.T, scraper *metricScraper, ctx context.Context, m metric, before int) {
	t.Helper()
	after, err := getSafeMetric(ctx, scraper, m)
	assert.NoError(t, err)
	assert.Equal(t, before-1, after)
}

// getSafeMetric is a helper function that retrieves a metric value. As metrics
// are not available when their value is 0, the method returns 0 if the metric
// is not found.
func getSafeMetric(ctx context.Context, scraper *metricScraper, metric metric) (int, error) {
	m, err := scraper.scrape(ctx, metric)
	if err != nil && errors.Is(err, errMetricNotFound) {
		return 0, nil
	}
	return m, err
}
