package model_test

import (
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/validation"
)

func TestNewSystem(t *testing.T) {
	externalIDUUID, err := uuid.NewV4()
	require.NoError(t, err)
	externalID := externalIDUUID.String()
	sysType := "APPLICATION"

	sys := model.NewSystem(externalID, sysType)

	assert.Equal(t, externalID, sys.ExternalID)
	assert.Equal(t, sysType, sys.Type)
	assert.Nil(t, sys.TenantID)
	assert.Equal(t, "systems", sys.TableName())
}

func TestSystemTenantLinking(t *testing.T) {
	sys := model.NewSystem("ext-1", "TYPE")
	tenantIDUUID, err := uuid.NewV4()
	require.NoError(t, err)
	tenantID := tenantIDUUID.String()

	assert.False(t, sys.IsLinkedToTenant())
	assert.Nil(t, sys.TenantID)

	sys.LinkTenant(tenantID)

	assert.True(t, sys.IsLinkedToTenant())
	require.NotNil(t, sys.TenantID)
	assert.Equal(t, tenantID, *sys.TenantID)

	emptyTenant := ""
	sys.TenantID = &emptyTenant
	assert.False(t, sys.IsLinkedToTenant())
}

func TestSystemPaginationKey(t *testing.T) {
	sys := model.NewSystem("ext-1", "TYPE")

	keys := sys.PaginationKey()

	assert.Contains(t, keys, repository.IDField)
	assert.Equal(t, sys.ID, keys[repository.IDField])
}

func TestSystemValidations(t *testing.T) {
	v, err := validation.New(validation.Config{
		Models: []validation.Model{&model.System{}},
	})
	assert.NoError(t, err)

	validSystem := *model.NewSystem(uuid.Must(uuid.NewV4()).String(), "Types")

	type mutateSystem func(s model.System) model.System

	tests := []struct {
		name   string
		mutate mutateSystem
		expErr error
	}{
		{
			name: "should return error for empty ExternalID",
			mutate: func(s model.System) model.System {
				s.ExternalID = ""
				s.Type = "type"
				return s
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should return error for empty Type",
			mutate: func(s model.System) model.System {
				s.Type = ""
				s.ExternalID = "externalID"
				return s
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should pass for valid System",
			mutate: func(s model.System) model.System {
				return s
			},
			expErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := tt.mutate(validSystem)
			values, err := validation.GetValues(&system)
			assert.NoError(t, err)

			err = v.ValidateAll(values)

			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}
