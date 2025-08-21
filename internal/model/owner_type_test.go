package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/registry/internal/model"
)

func TestOwnerTypeValidation(t *testing.T) {
	tests := map[string]struct {
		ownerType model.OwnerType
		expectErr bool
		errCode   codes.Code
	}{
		"Valid OwnerType - CustomerID": {
			ownerType: "owner type",
			expectErr: false,
		},
		"Invalid OwnerType": {
			ownerType: "",
			expectErr: true,
			errCode:   codes.InvalidArgument,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when
			err := test.ownerType.Validate()

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
