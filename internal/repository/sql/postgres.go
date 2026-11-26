package sql

import (
	"context"
	"fmt"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/model"
)

// StartDB starts DB connection and runs migrations.
func StartDB(ctx context.Context, dbConf config.DB) (*gorm.DB, error) {
	dbCon, err := startDBConnection(dbConf)
	if err != nil {
		slogctx.Error(ctx, "failed to initialize DB connection", "error", err)
		return nil, err
	}

	dbCon = dbCon.WithContext(ctx)
	slogctx.Info(ctx, "DB connection done")

	if err = Migrate(dbCon); err != nil {
		slogctx.Error(ctx, "failed to run migrations", "error", err)
		return nil, err
	}

	slogctx.Info(ctx, "DB migration done")

	return dbCon, nil
}

// startDBConnection initializes and returns a database connection using the provided configuration.
func startDBConnection(conf config.DB) (*gorm.DB, error) {
	dsn, err := GetDataSourceName(conf)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.LogLevel(conf.LogLevel)),
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func GetDataSourceName(conf config.DB) (string, error) {
	password, err := commoncfg.LoadValueFromSourceRef(conf.Password)
	if err != nil {
		return "", err
	}

	user, err := commoncfg.LoadValueFromSourceRef(conf.User)
	if err != nil {
		return "", err
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s", conf.Host, user, password, conf.Name, conf.Port)

	return dsn, nil
}

// Migrate runs DB migrations.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&model.System{}, &model.RegionalSystem{}, &model.Tenant{}, &model.Auth{}, &model.TenantConfig{})
}
