package model_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/validation"
)

func TestAuthToProto(t *testing.T) {
	// given
	key := "key"
	auth := model.Auth{
		ExternalID: "external-id",
		TenantID:   "tenant-id",
		Type:       "auth-type",
		Properties: map[string]string{
			key: "value",
		},
		Status: pb.AuthStatus_AUTH_STATUS_APPLIED.String(),
	}

	// when
	authProto := auth.ToProto()

	// then
	assert.Equal(t, auth.ExternalID, authProto.ExternalId)
	assert.Equal(t, auth.TenantID, authProto.TenantId)
	assert.Equal(t, auth.Type, authProto.Type)
	assert.Equal(t, auth.Properties[key], authProto.Properties[key])
	assert.Equal(t, pb.AuthStatus_AUTH_STATUS_APPLIED, authProto.Status)
}

func TestAuthValidationIDs(t *testing.T) {
	// given
	authType := reflect.TypeFor[model.Auth]()

	var tagValidationIDs []string
	for i := range authType.NumField() {
		field := authType.Field(i)
		if validationID := field.Tag.Get(validation.TagName); validationID != "" {
			tagValidationIDs = append(tagValidationIDs, validationID)
		}
	}

	constants := map[validation.ID]struct{}{
		model.AuthExternalIDValidationID: {},
		model.AuthTenantIDValidationID:   {},
		model.AuthTypeValidationID:       {},
		model.AuthPropertiesValidationID: {},
		model.AuthStatusValidationID:     {},
	}

	// then
	for _, tagID := range tagValidationIDs {
		_, exists := constants[validation.ID(tagID)]
		assert.True(t, exists)
	}
}

func TestAuthValidations(t *testing.T) {
	// given
	v, err := validation.New(validation.Config{
		Models: []validation.Model{&model.Auth{}},
	})
	assert.NoError(t, err)

	validAuth := model.Auth{
		ExternalID: "external-id",
		TenantID:   "tenant-id",
		Type:       "auth-type",
		Properties: map[string]string{
			"key": "value",
		},
		Status: pb.AuthStatus_AUTH_STATUS_APPLIED.String(),
	}

	type mutateAuth func(a model.Auth) model.Auth

	tests := []struct {
		name   string
		mutate mutateAuth
		expErr error
	}{
		{
			name: "should return error for empty ExternalID",
			mutate: func(a model.Auth) model.Auth {
				a.ExternalID = ""
				return a
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should return error for empty TenantID",
			mutate: func(a model.Auth) model.Auth {
				a.TenantID = ""
				return a
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should return error for empty Type",
			mutate: func(a model.Auth) model.Auth {
				a.Type = ""
				return a
			},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should return error for invalid Status",
			mutate: func(a model.Auth) model.Auth {
				a.Status = "invalid-status"
				return a
			},
			expErr: validation.ErrValueNotAllowed,
		},
		{
			name: "should return error for empty Properties keys",
			mutate: func(a model.Auth) model.Auth {
				a.Properties = map[string]string{
					"": "value",
				}
				return a
			},
			expErr: validation.ErrKeyEmpty,
		},
		{
			name: "should pass for valid Auth",
			mutate: func(a model.Auth) model.Auth {
				return a
			},
			expErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			auth := tt.mutate(validAuth)
			valuesByID, err := validation.GetValues(&auth)
			assert.NoError(t, err)

			err = v.ValidateAll(valuesByID)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestAuthStatusConstraint(t *testing.T) {
	// given
	constraint := model.AuthStatusConstraint{}
	tests := []struct {
		name   string
		value  any
		expErr error
	}{
		{
			name:   "should return error for non-string value",
			value:  123,
			expErr: validation.ErrWrongType,
		},
		{
			name:   "should return error for invalid status",
			value:  "invalid-status",
			expErr: validation.ErrValueNotAllowed,
		},
		{
			name:   "should return nil for valid status",
			value:  pb.AuthStatus_AUTH_STATUS_APPLIED.String(),
			expErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			err := constraint.Validate(tt.value)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}
