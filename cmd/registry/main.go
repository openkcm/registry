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
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/openkcm/common-sdk/pkg/status"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"

	_ "gorm.io/driver/postgres"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	slogctx "github.com/veqryn/slog-context"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	root "github.com/openkcm/registry"
	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/interceptor"
	"github.com/openkcm/registry/internal/repository/sql"
	"github.com/openkcm/registry/internal/service"
)

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

	orbital := service.NewOrbital()
	err = orbital.Init(ctx, db, repository, cfg.Orbital)
	handleErr("initializing Orbital", err)

	tenantSrv := service.NewTenant(repository, orbital, meters)
	systemSrv := service.NewSystem(repository, meters)

	grpcServer, err := setupGRPCServer(ctx, cfg)
	handleErr("initializing gRPC server", err)

	tenantgrpc.RegisterServiceServer(grpcServer, tenantSrv)
	systemgrpc.RegisterServiceServer(grpcServer, systemSrv)

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

	keepaliveParams := keepalive.ServerParameters{
		MaxConnectionIdle:     cfg.GRPCServer.Attributes.MaxConnectionIdle,
		MaxConnectionAge:      cfg.GRPCServer.Attributes.MaxConnectionAge,
		MaxConnectionAgeGrace: cfg.GRPCServer.Attributes.MaxConnectionAgeGrace,
		Time:                  cfg.GRPCServer.Attributes.Time,
		Timeout:               cfg.GRPCServer.Attributes.Timeout,
	}

	enforcementPolicy := keepalive.EnforcementPolicy{
		MinTime:             cfg.GRPCServer.EfPolMinTime,
		PermitWithoutStream: cfg.GRPCServer.EfPolPermitWithoutStream,
	}

	meter := otel.Meter(
		cfg.Application.Name,
		metric.WithInstrumentationVersion(otel.Version()),
		metric.WithInstrumentationAttributes(otlp.CreateAttributesFrom(cfg.Application)...),
	)

	met, err := interceptor.InitMeters(ctx, &cfg.Application, meter)
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otlp.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			met.UnaryInterceptor,
			rec.UnaryInterceptor,
		),
		grpc.ChainStreamInterceptor(
			met.StreamInterceptor,
			rec.StreamInterceptor,
		),
		grpc.KeepaliveParams(keepaliveParams),
		grpc.KeepaliveEnforcementPolicy(enforcementPolicy),
		grpc.MaxRecvMsgSize(cfg.GRPCServer.MaxRecvMsgSize),
	)

	enableReflection(cfg, grpcServer)

	healthpb.RegisterHealthServer(grpcServer, &health.GRPCServer{})

	return grpcServer, nil
}

func enableReflection(cfg *config.Config, grpcServer *grpc.Server) {
	// Reflection is a protocol that gRPC servers can use to declare the protobuf-defined APIs.
	// Reflection is used by debugging tools like grpcurl or grpcui.
	// See https://grpc.io/docs/guides/reflection/.
	if cfg.DebugMode {
		slog.Info("enabling gRPC reflection")
		reflection.Register(grpcServer)
	}
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

func handleErr(msg string, err error) {
	if err != nil {
		log.Fatalf("error %s: %v", msg, err)
	}
}

func loadConfig() *config.Config {
	cfg := &config.Config{}
	loader := commoncfg.NewLoader(cfg,
		commoncfg.WithPaths(
			"/etc/registry",
			"."),
		commoncfg.WithEnvOverride(""))
	err := loader.LoadConfig()
	handleErr("loading config", err)

	err = commoncfg.UpdateConfigVersion(&cfg.BaseConfig, root.BuildVersion)
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
	grpcCfg := commoncfg.GRPCClient{
		Address:    cfg.GRPCServer.Address,
		Attributes: cfg.GRPCServer.ClientAttributes,
		Pool: commoncfg.GRPCPool{
			InitialCapacity: 1,
			MaxCapacity:     3,
		},
	}
	healthOptions = append(healthOptions, health.WithGRPCServerChecker(grpcCfg))

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
