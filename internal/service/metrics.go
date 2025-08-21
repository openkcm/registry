package service

import (
	"context"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gorm.io/gorm"

	"github.com/openkcm/registry/internal/model"
)

const (
	AttrRegion       = "region"
	AttrTenantLinked = "tenant_linked"
	AttrStatus       = "status"
	ErrDomainMetrics = "metrics"
)

func InitMeters(ctx context.Context, cfgApp *commoncfg.Application, db *gorm.DB) (*Meters, error) {
	meter := otel.Meter(
		cfgApp.Name,
		metric.WithInstrumentationVersion(otel.Version()),
		metric.WithInstrumentationAttributes(otlp.CreateAttributesFrom(*cfgApp)...),
	)

	var err error

	systemRegistrationCtr, err := createCounter(ctx, meter, "systems.registered", "Counter of system registrations, partitioned by region")
	if err != nil {
		return nil, err
	}

	systemDeletionCtr, err := createCounter(ctx, meter, "systems.deleted", "Counter of system deletions, partitioned by region")
	if err != nil {
		return nil, err
	}

	err = createObservableGauge(ctx, meter, "systems.count", "Gauge of systems, partitioned by region and tenant link status",
		func(ctx context.Context, observer metric.Int64Observer) error {
			return measureSystems(ctx, observer, db)
		})
	if err != nil {
		return nil, err
	}

	tenantRegistrationCtr, err := createCounter(ctx, meter, "tenants.registered", "Counter of tenant registrations, partitioned by region")
	if err != nil {
		return nil, err
	}

	err = createObservableGauge(ctx, meter, "tenants.count", "Gauge of tenants, partitioned by status and region",
		func(ctx context.Context, observer metric.Int64Observer) error {
			return measureTenants(ctx, observer, db)
		})
	if err != nil {
		return nil, err
	}

	return &Meters{
		application:           cfgApp,
		systemRegistrationCtr: systemRegistrationCtr,
		tenantRegistrationCtr: tenantRegistrationCtr,
		systemDeletionCtr:     systemDeletionCtr,
	}, nil
}

func createCounter(ctx context.Context, meter metric.Meter, name string, description string) (metric.Int64Counter, error) {
	ctr, err := meter.Int64Counter(
		name,
		metric.WithDescription(description),
	)
	if err != nil {
		return nil, oops.In(ErrDomainMetrics).
			WithContext(ctx).
			Wrapf(err, "creating %s meter", name)
	}

	return ctr, nil
}

func createObservableGauge(ctx context.Context, meter metric.Meter, name string, description string, callback metric.Int64Callback) error {
	_, err := meter.Int64ObservableGauge(
		name,
		metric.WithDescription(description),
		metric.WithInt64Callback(callback),
	)
	if err != nil {
		return oops.In(ErrDomainMetrics).
			WithContext(ctx).
			Wrapf(err, "creating %s meter", name)
	}

	return nil
}

func measureTenants(ctx context.Context, observer metric.Int64Observer, db *gorm.DB) error {
	var tenantStatus []struct {
		Status string
		Region string
		Count  int64
	}

	err := db.WithContext(ctx).
		Model(&model.Tenant{}).
		Select("status, region, count(*) as count").
		Group("status, region").
		Scan(&tenantStatus).Error
	if err != nil {
		return err
	}

	for _, status := range tenantStatus {
		observer.Observe(status.Count, metric.WithAttributes(
			attribute.String(AttrRegion, status.Region),
			attribute.String(AttrStatus, status.Status)))
	}

	return nil
}

func measureSystems(ctx context.Context, observer metric.Int64Observer, db *gorm.DB) error {
	var systemLinkStatus []struct {
		Linked string
		Region string
		Count  int64
	}

	err := db.WithContext(ctx).
		Model(&model.System{}).
		Select("region, count(*) as count, case when tenant_id = '' then 'false' else 'true' end as linked").
		Group("region, case when tenant_id = '' then 'false' else 'true' end").
		Scan(&systemLinkStatus).Error
	if err != nil {
		return err
	}

	for _, status := range systemLinkStatus {
		observer.Observe(status.Count, metric.WithAttributes(
			attribute.String(AttrRegion, status.Region),
			attribute.String(AttrTenantLinked, status.Linked)))
	}

	return nil
}

type Meters struct {
	application           *commoncfg.Application
	systemRegistrationCtr metric.Int64Counter
	tenantRegistrationCtr metric.Int64Counter
	systemDeletionCtr     metric.Int64Counter
}

func (m *Meters) handleSystemRegistration(ctx context.Context, region string) {
	m.handleCtrInc(ctx, m.systemRegistrationCtr, region)
}

func (m *Meters) handleSystemDeletion(ctx context.Context, region string) {
	m.handleCtrInc(ctx, m.systemDeletionCtr, region)
}

func (m *Meters) handleTenantRegistration(ctx context.Context, region string) {
	m.handleCtrInc(ctx, m.tenantRegistrationCtr, region)
}

func (m *Meters) handleCtrInc(ctx context.Context, ctr metric.Int64Counter, region string) {
	attrs := metric.WithAttributes(
		otlp.CreateAttributesFrom(*m.application,
			attribute.String(AttrRegion, region),
		)...,
	)

	ctr.Add(ctx, 1, attrs)
}
