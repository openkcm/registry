//go:build integration

package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"

	_ "google.golang.org/grpc/health"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	typespb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/registry/integration/operatortest"
	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository/sql"
	"github.com/openkcm/registry/internal/service"
)

// Allowed system values based on the constraints defined in config.yaml.
const (
	// Tenant.OwnerType allowed value.
	allowedOwnerType = "ownerType1"
	// System.Type allowed value.
	allowedSystemType = "application"
	// System.Region allowed value.
	allowedSystemRegion = "region-application"
)

var ErrMissingSvrPort = errors.New("server port is missing")

func loadConfig() (*config.Config, error) {
	cfg := &config.Config{}
	err := commoncfg.LoadConfig(cfg,
		map[string]any{},
		"..",
	)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func startDB() (*gorm.DB, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	db, err := sql.StartDB(context.Background(), cfg.Database)
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&model.Tenant{}, &model.System{}, &model.RegionalSystem{}, model.Auth{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func newGRPCClientConn() (*grpc.ClientConn, error) {
	conf, err := loadConfig()
	if err != nil {
		return nil, err
	}
	port := conf.GRPCServer.Address
	if port == "" {
		return nil, ErrMissingSvrPort
	}
	serverAddr := "localhost" + port

	// healthConfig defines the configuration for client-side health checking.
	// `round_robin` is the only load balancing policy that supports client-side health checking.
	// An empty string for `serviceName` indicates the overall health of a server.
	// For more information, see: https://github.com/grpc/grpc-go/tree/master/examples/features/health
	healthConfig := grpc.WithDefaultServiceConfig(`{
		"loadBalancingPolicy": "round_robin",
		"healthCheckConfig": {
		  "serviceName": ""
		}
	}`)

	return grpc.NewClient(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		healthConfig,
	)
}

func validRandID() string {
	var sb strings.Builder
	sb.WriteString(strings.ReplaceAll(uuid.New().String(), "-", ""))
	sb.WriteString(strings.ReplaceAll(uuid.New().String(), "-", "")[:8])
	return sb.String()
}

func validTenant() *model.Tenant {
	return &model.Tenant{
		Name:      "SuccessFactor",
		ID:        validRandID(),
		Region:    operatortest.Region,
		OwnerID:   "owner123",
		OwnerType: allowedOwnerType,
		Status:    model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()),
		Role:      tenantgrpc.Role_ROLE_LIVE.String(),
	}
}

func validAuth() *model.Auth {
	return &model.Auth{
		ExternalID: validRandID(),
		TenantID:   validRandID(),
		Type:       "oidc",
		Status:     authgrpc.AuthStatus_AUTH_STATUS_APPLIED.String(),
	}
}

func validRegisterTenantReq() *tenantgrpc.RegisterTenantRequest {
	return &tenantgrpc.RegisterTenantRequest{
		Name:      "SuccessFactor",
		Id:        validRandID(),
		Region:    "region",
		OwnerId:   "owner123",
		OwnerType: allowedOwnerType,
		Role:      tenantgrpc.Role_ROLE_LIVE,
	}
}

func validRegisterSystemReq() *systemgrpc.RegisterSystemRequest {
	return &systemgrpc.RegisterSystemRequest{
		ExternalId: validRandID(),
		L2KeyId:    "key123",
		Status:     typespb.Status_STATUS_AVAILABLE,
		Region:     allowedSystemRegion,
		Type:       allowedSystemType,
		Labels: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
}

func unlinkSystemFromTenant(ctx context.Context, s systemgrpc.ServiceClient, m mappinggrpc.ServiceClient, externalID, region, tenantID string) error {
	_, err := s.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
		ExternalId: externalID,
		Type:       allowedSystemType,
		Region:     region,
		Status:     typespb.Status_STATUS_AVAILABLE,
	})
	if err != nil {
		return err
	}

	_, err = m.UnmapSystemFromTenant(ctx, &mappinggrpc.UnmapSystemFromTenantRequest{
		ExternalId: externalID,
		Type:       allowedSystemType,
		TenantId:   tenantID,
	})

	return err
}

func deleteOrbitalResources(ctx context.Context, db *gorm.DB, externalID string) error {
	var jobs []orbital.Job
	err := db.WithContext(ctx).Table("jobs").Find(&jobs).Where("external_id = ?", externalID).Error
	if err != nil {
		return err
	}
	for _, job := range jobs {
		err := db.WithContext(ctx).Table("tasks").Where("job_id = ?", job.ID).Delete(nil).Error
		if err != nil {
			return err
		}
		err = db.WithContext(ctx).Table("job_cursor").Where("id = ?", job.ID).Delete(nil).Error
		if err != nil {
			return err
		}
		err = db.WithContext(ctx).Table("job_event").Where("id = ?", job.ID).Delete(nil).Error
		if err != nil {
			return err
		}
		err = db.WithContext(ctx).Table("jobs").Where("id = ?", job.ID).Delete(nil).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteTenantFromDB(ctx context.Context, db *gorm.DB, tenant *model.Tenant) error {
	err := deleteOrbitalResources(ctx, db, tenant.ID)
	if err != nil {
		return err
	}
	repo := sql.NewRepository(db)
	_, err = repo.Delete(ctx, tenant)
	return err
}

// createTenantInDB creates a tenant in the database.
// It can be used in tests to simulate a tenant being already created and in a specific state.
func createTenantInDB(ctx context.Context, db *gorm.DB, tenant *model.Tenant) error {
	repo := sql.NewRepository(db)
	return repo.Create(ctx, tenant)
}

func createSystemInDB(ctx context.Context, db *gorm.DB, system *model.System) error {
	repo := sql.NewRepository(db)
	return repo.Create(ctx, system)
}

// getSystemFromDB retrieves a system from the database by its ID.
func getSystemFromDB(ctx context.Context, db *gorm.DB, externalID, systemType string) (*model.System, error) {
	repo := sql.NewRepository(db)
	sys := &model.System{
		ExternalID: externalID,
		Type:       systemType,
	}

	found, err := repo.Find(ctx, sys)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	return sys, nil
}

// deleteSystemInDB deletes a system from the database by its ID.
func deleteSystemInDB(ctx context.Context, db *gorm.DB, externalID, systemType string) error {
	repo := sql.NewRepository(db)
	sys, err := getSystemFromDB(ctx, db, externalID, systemType)
	if err != nil {
		return err
	}

	_, err = repo.Delete(ctx, sys)
	return err
}

// deleteSystem deletes a regional system via the gRPC service client.
func deleteSystem(ctx context.Context, s systemgrpc.ServiceClient, externalID, systemType, region string) error {
	_, err := s.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
		ExternalId: externalID,
		Type:       systemType,
		Region:     region,
		Status:     typespb.Status_STATUS_AVAILABLE,
	})
	if err != nil && !errors.Is(err, service.ErrSystemNotFound) {
		return err
	}
	_, err = s.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
		ExternalId: externalID,
		Type:       systemType,
		Region:     region,
	})
	return err
}

func cleanupSystem(t *testing.T, ctx context.Context, sSubj systemgrpc.ServiceClient, mSubj mappinggrpc.ServiceClient, externalID, tenantID, systemType, region string, l1KeyClaim bool) {
	if l1KeyClaim {
		_, err := sSubj.UpdateSystemL1KeyClaim(ctx, &systemgrpc.UpdateSystemL1KeyClaimRequest{
			ExternalId: externalID,
			Type:       systemType,
			Region:     region,
			TenantId:   tenantID,
			L1KeyClaim: false,
		})
		assert.NoError(t, err)
	}
	if tenantID != "" {
		_, err := mSubj.UnmapSystemFromTenant(ctx, &mappinggrpc.UnmapSystemFromTenantRequest{
			ExternalId: externalID,
			Type:       systemType,
			TenantId:   tenantID,
		})
		assert.NoError(t, err)
	}

	err := deleteSystem(ctx, sSubj, externalID, systemType, region)
	assert.NoError(t, err)
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
