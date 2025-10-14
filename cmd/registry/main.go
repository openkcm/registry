package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/openkcm/common-sdk/pkg/status"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	_ "gorm.io/driver/postgres"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/interceptor"
	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository/sql"
	"github.com/openkcm/registry/internal/service"
	"github.com/openkcm/registry/internal/validation"
)

var BuildInfo = "{}"

func main() {
	ctx := context.Background()

	cfg := loadConfig()
	err := cfg.Validate()
	handleErr("validating config", err)

	initLogger(cfg)

	initOTLP(ctx, cfg)

	// Status server initialization
	go startStatusServer(cfg, ctx)

	db := initDB(ctx, cfg)

	meters, err := service.InitMeters(ctx, &cfg.Application, db)
	handleErr("initializing meters", err)

	repository := sql.NewRepository(db)

	orbital, err := service.NewOrbital(ctx, db, cfg.Orbital)
	handleErr("initializing Orbital", err)

	validations := initValidations(cfg.Validations)
	handleErr("initializing validations", err)

	tenantSrv := service.NewTenant(repository, orbital, meters)
	systemSrv := service.NewSystem(repository, meters)
	authSrv := service.NewAuth(repository, orbital, validations)

	grpcServer, err := setupGRPCServer(ctx, cfg)
	handleErr("initializing gRPC server", err)

	tenantgrpc.RegisterServiceServer(grpcServer, tenantSrv)
	systemgrpc.RegisterServiceServer(grpcServer, systemSrv)
	authgrpc.RegisterServiceServer(grpcServer, authSrv)

	startGRPCServer(ctx, cfg, grpcServer)
}

func startGRPCServer(ctx context.Context, cfg *config.Config, grpcServer *grpc.Server) {
	var lc net.ListenConfig

	lis, err := lc.Listen(ctx, "tcp", cfg.GRPCServer.Address)

	handleErr("starting server", err)
	slogctx.Info(ctx, "gRPC server is listening", "address", cfg.GRPCServer.Address)

	// Handle server shutdown gracefully when the process is terminated.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		grpcServer.GracefulStop()
		slogctx.Info(ctx, "gRPC server is stopped")
	}()

	err = grpcServer.Serve(lis)
	handleErr("listening to gRPC requests", err)
}

func setupGRPCServer(ctx context.Context, cfg *config.Config) (*grpc.Server, error) {
	rec := interceptor.NewRecover()

	meter := otel.Meter(
		cfg.Application.Name,
		metric.WithInstrumentationVersion(otel.Version()),
		metric.WithInstrumentationAttributes(otlp.CreateAttributesFrom(cfg.Application)...),
	)

	met, err := interceptor.InitMeters(ctx, &cfg.Application, meter)
	if err != nil {
		return nil, err
	}

	// Create a new gRPC server
	grpcServer := commongrpc.NewServer(ctx, &cfg.GRPCServer.GRPCServer,
		grpc.ChainUnaryInterceptor(
			met.UnaryInterceptor,
			rec.UnaryInterceptor,
		),
		grpc.ChainStreamInterceptor(
			met.StreamInterceptor,
			rec.StreamInterceptor,
		),
	)

	return grpcServer, nil
}

func initDB(ctx context.Context, cfg *config.Config) *gorm.DB {
	db, err := sql.StartDB(ctx, cfg.Database)
	handleErr("starting database", err)

	return db
}

func initOTLP(ctx context.Context, cfg *config.Config) {
	err := otlp.Init(ctx, &cfg.Application, &cfg.Telemetry, &cfg.Logger, otlp.WithLogger(slog.Default()))
	handleErr("starting OpenTelemetry", err)
}

func initLogger(cfg *config.Config) {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	handleErr("initializing logger", err)
}

func initValidations(configFields []validation.ConfigField) *validation.Validation {
	v, err := validation.New(configFields...)
	handleErr("initializing validation", err)

	sources := make([]map[validation.ID]struct{}, 0, 1)

	for _, s := range []validation.Struct{&model.Auth{}} {
		v.AddStructFields(s.Fields()...)

		ids, err := validation.GetIDs(s)
		handleErr("getting IDs", err)
		sources = append(sources, ids)
	}

	err = v.CheckIDs(sources...)
	handleErr("validating IDs", err)

	return v
}

func handleErr(msg string, err error) {
	if err != nil {
		log.Fatalf("error %s: %v", msg, err)
	}
}

func loadConfig() *config.Config {
	cfg := &config.Config{}
	loader := commoncfg.NewLoader(cfg,
		commoncfg.WithPaths("/etc/registry", "."),
		commoncfg.WithEnvOverride(""))
	err := loader.LoadConfig()
	handleErr("loading config", err)

	err = commoncfg.UpdateConfigVersion(&cfg.BaseConfig, BuildInfo)
	handleErr("loading build version into config", err)

	return cfg
}

func startStatusServer(cfg *config.Config, ctx context.Context) {
	liveness := status.WithLiveness(
		health.NewHandler(
			health.NewChecker(health.WithDisabledAutostart()),
		),
	)

	healthOptions := make([]health.Option, 0)
	healthOptions = append(healthOptions,
		health.WithDisabledAutostart(),
		health.WithStatusListener(func(ctx context.Context, state health.State) {
			slogctx.Info(ctx, "readiness status changed", "status", state.Status, "checkStates", state.CheckState)
		}),
	)

	// Add gRPC health server checker
	cfg.GRPCServer.Client.Address = cfg.GRPCServer.Address
	healthOptions = append(healthOptions,
		health.WithGRPCServerChecker(cfg.GRPCServer.Client),
	)

	// database health check
	dsn, err := sql.GetDataSourceName(cfg.Database)
	handleErr("getting data source name", err)

	healthOptions = append(healthOptions,
		health.WithDatabaseChecker("pgx", dsn))

	readiness := status.WithReadiness(
		health.NewHandler(
			health.NewChecker(healthOptions...),
		),
	)

	// Start the status server
	err = status.Start(ctx, &cfg.BaseConfig, liveness, readiness)
	if err != nil {
		slogctx.Error(ctx, "Failure on the status server", "error", err)

		_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	}
}
