package service

import (
	"context"

	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

var (
	MapError         = mapError
	UnlinkAllSystems = unlinkAllSystems
)

// NewAuthForTest builds an Auth without touching orbital, so that unit tests
// can drive its behaviour against a fake repository.
func NewAuthForTest(repo repository.Repository, orbital *Orbital, validation *validation.Validation) *Auth {
	return &Auth{
		repo:       repo,
		orbital:    orbital,
		validation: validation,
	}
}

// NewTenantForTest builds a Tenant without registering orbital job handlers,
// so that unit tests can drive its behaviour against a fake repository.
func NewTenantForTest(repo repository.Repository, validation *validation.Validation) *Tenant {
	return &Tenant{
		repo:       repo,
		validation: validation,
	}
}

// NoopRepo is a minimal repository.Repository that does nothing, for use in tests
// that only exercise code paths which do not reach the repository.
type NoopRepo struct{}

func (NoopRepo) Create(_ context.Context, _ repository.Resource) error         { return nil }
func (NoopRepo) List(_ context.Context, _ any, _ repository.Query) error       { return nil }
func (NoopRepo) Delete(_ context.Context, _ repository.Resource) (bool, error) { return false, nil }
func (NoopRepo) Find(_ context.Context, _ repository.Resource) (bool, error)   { return false, nil }
func (NoopRepo) Patch(_ context.Context, _ repository.Resource) (bool, error)  { return true, nil }
func (NoopRepo) PatchAll(_ context.Context, _ repository.Resource, _ any, _ repository.Query) (int64, error) {
	return 0, nil
}
func (NoopRepo) Transaction(_ context.Context, fn repository.TransactionFunc) error {
	return fn(context.Background(), NoopRepo{})
}
