package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithUserGroups struct {
	UserGroups model.UserGroups `validators:"array"`
}

func TestUserGroupsValidate(t *testing.T) {
	typeWithUserGroups := TypeWithUserGroups{}
	model.RegisterValidatorsForTypes(typeWithUserGroups)
	defer model.ClearGlobalTypeValidators()

	tests := []struct {
		name    string
		input   model.UserGroups
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil slice",
			input:   nil,
			wantErr: false,
		},
		{
			name:    "empty slice",
			input:   model.UserGroups{},
			wantErr: false,
		},
		{
			name:    "contains empty string",
			input:   model.UserGroups{"admin", "", "user"},
			wantErr: true,
			errMsg:  model.FieldContainsEmptyValuesMsg + ": UserGroups",
		},
		{
			name:    "valid groups",
			input:   model.UserGroups{"admin", "user"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typeWithUserGroups.UserGroups = tt.input
			err := model.ValidateField(&typeWithUserGroups, &typeWithUserGroups.UserGroups)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, model.ErrFieldContainsEmptyValues)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			assert.NoError(t, err)
		})
	}
}
