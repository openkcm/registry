package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/registry/internal/model"
)

func TestOwnerIDValidation(t *testing.T) {
	tests := map[string]struct {
		ownerID   model.OwnerID
		expectErr bool
		errCode   codes.Code
	}{
		"Valid OwnerID": {
			ownerID:   "valid-owner-id",
			expectErr: false,
		},
		"Empty OwnerID": {
			ownerID:   "",
			expectErr: true,
			errCode:   codes.InvalidArgument,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when
			err := test.ownerID.Validate(model.EmptyValidationContext)

			// then
			if test.expectErr {
				assert.Error(t, err)
				st, _ := status.FromError(err)
				assert.Equal(t, test.errCode, st.Code())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
