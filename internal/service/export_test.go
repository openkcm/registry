package service

import (
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

var (
	MapError = mapError
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
