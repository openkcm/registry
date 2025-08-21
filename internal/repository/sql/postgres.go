package sql

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/model"
)

// StartDB starts DB connection and runs migrations.
func StartDB(ctx context.Context, dbConf config.DB) (*gorm.DB, error) {
	dbCon, err := startDBConnection(dbConf)
	if err != nil {
		slog.Error("failed to initialize DB connection", slog.Any("err", err))
		return nil, err
	}

	dbCon.WithContext(ctx)
	slog.Info("DB connection done")

	if err = Migrate(dbCon); err != nil {
		slog.Error("failed to run migrations", slog.Any("err", err))
		return nil, err
	}

	slog.Info("DB migration done")

	return dbCon, nil
}

// startDBConnection initializes and returns a database connection using the provided configuration.
func startDBConnection(conf config.DB) (*gorm.DB, error) {
	dsn, err := GetDataSourceName(conf)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
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
	return db.AutoMigrate(&model.System{}, &model.Tenant{})
}
