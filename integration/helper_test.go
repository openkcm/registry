//go:build integration

package integration_test

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	_ "google.golang.org/grpc/health"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	typespb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository/sql"
	"github.com/openkcm/registry/internal/service"
)

var (
	ErrMissingSvrPort = errors.New("server port is missing")
)

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

	err = db.AutoMigrate(&model.Tenant{}, &model.System{}, model.Auth{})
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
		ID:        model.ID(validRandID()),
		Region:    "region",
		OwnerID:   "owner123",
		OwnerType: "owner_type",
		Status:    model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()),
		Role:      model.Role(tenantgrpc.Role_ROLE_LIVE.String()),
	}
}

func validAuth() *model.Auth {
	return &model.Auth{
		ExternalID: model.ExternalID(validRandID()),
		TenantID:   model.ID(validRandID()),
		Type:       "auth_typ",
		Status:     model.AuthStatus(authgrpc.AuthStatus_AUTH_STATUS_APPLIED.String()),
	}
}

func validRegisterTenantReq() *tenantgrpc.RegisterTenantRequest {
	return &tenantgrpc.RegisterTenantRequest{
		Name:      "SuccessFactor",
		Id:        validRandID(),
		Region:    "region",
		OwnerId:   "owner123",
		OwnerType: "owner_type",
		Role:      tenantgrpc.Role_ROLE_LIVE,
	}
}

func validRegisterSystemReq() *systemgrpc.RegisterSystemRequest {
	return &systemgrpc.RegisterSystemRequest{
		ExternalId: validRandID(),
		L2KeyId:    "key123",
		Status:     typespb.Status_STATUS_AVAILABLE,
		Region:     "region",
		Type:       "system",
		Labels: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
}

func unlinkSystemFromTenant(ctx context.Context, s systemgrpc.ServiceClient, externalID, region string) error {
	_, err := s.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
		ExternalId: externalID,
		Region:     region,
		Status:     typespb.Status_STATUS_AVAILABLE,
	})
	if err != nil {
		return err
	}

	_, err = s.UnlinkSystemsFromTenant(ctx, &systemgrpc.UnlinkSystemsFromTenantRequest{
		SystemIdentifiers: []*systemgrpc.SystemIdentifier{
			{
				ExternalId: externalID,
				Region:     region,
			},
		},
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
	err := deleteTenantJobFromDB(db, tenant.ID.String())
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

func deleteSystem(ctx context.Context, s systemgrpc.ServiceClient, externalID, region string) error {
	_, err := s.UpdateSystemStatus(ctx, &systemgrpc.UpdateSystemStatusRequest{
		ExternalId: externalID,
		Region:     region,
		Status:     typespb.Status_STATUS_AVAILABLE,
	})
	if err != nil && !errors.Is(err, service.ErrSystemNotFound) {
		return err
	}
	_, err = s.DeleteSystem(ctx, &systemgrpc.DeleteSystemRequest{
		ExternalId: externalID,
		Region:     region,
	})
	return err
}

// deleteTenantJobFromDB deletes all orbital jobs related to the tenant with the given tenantId.
func deleteTenantJobFromDB(db *gorm.DB, tenantId string) error {
	type Job struct {
		ID   string
		Data []byte
	}

	var jobs []Job
	if err := db.Table("jobs").Select("id, data").Find(&jobs).Error; err != nil {
		return err
	}
	for _, job := range jobs {
		var tenant tenantgrpc.Tenant
		if err := proto.Unmarshal(job.Data, &tenant); err != nil {
			continue // ignore jobs that cannot be unmarshalled
		}
		if tenant.Id == tenantId {
			if err := db.Table("tasks").Where("job_id = ?", job.ID).Delete(nil).Error; err != nil {
				return err
			}
			if err := db.Table("jobs").Where("id = ?", job.ID).Delete(nil).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
