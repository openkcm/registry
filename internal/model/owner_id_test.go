package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithOwnerID struct {
	OwnerID model.OwnerID `validators:"non-empty"`
}

func TestOwnerIDValidation(t *testing.T) {
	typeWithOwnerID := TypeWithOwnerID{}
	model.RegisterValidatorsForTypes(typeWithOwnerID)
	defer model.ClearGlobalTypeValidators()

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
			typeWithOwnerID.OwnerID = test.ownerID
			err := model.ValidateField(&typeWithOwnerID, &typeWithOwnerID.OwnerID)

			// then
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, model.ErrFieldValueMustNotBeEmpty)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
