package model_test

import (
	"testing"

	"github.com/openkcm/registry/internal/model"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUserGroupsValidate(t *testing.T) {
	tests := []struct {
		name     string
		input    model.UserGroups
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name:     "nil slice",
			input:    nil,
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "empty slice",
			input:    model.UserGroups{},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "contains empty string",
			input:    model.UserGroups{"admin", "", "user"},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name:    "valid groups",
			input:   model.UserGroups{"admin", "user"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				st, _ := status.FromError(err)
				assert.Equal(t, tt.wantCode, st.Code())
				return
			}
			assert.NoError(t, err)
		})
	}
}
