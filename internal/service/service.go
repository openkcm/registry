package service

import (
	"context"
	"time"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
)

const defaultTranTimeout = time.Second * 10

// assertTenantExist checks if a tenant exists in the database by tenant_id.
// It returns an error if the tenant does not exist.
func assertTenantExist(ctx context.Context, r repository.Repository, tenantID string) error {
	tenant := &model.Tenant{ID: tenantID}

	found, err := r.Find(ctx, tenant)
	if err != nil {
		return ErrTenantSelect
	}

	if !found {
		return ErrTenantNotFound
	}

	return nil
}
