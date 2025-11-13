package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/validation"
)

func TestListConstraint(t *testing.T) {
	// given
	tests := []struct {
		name       string
		constraint validation.ListConstraint
		value      any
		expErr     error
	}{
		{
			name: "should return error for non-string value",
			constraint: validation.ListConstraint{
				AllowList: []string{"value1", "value2"},
			},
			value:  123,
			expErr: validation.ErrWrongType,
		},
		{
			name: "should return error for value not in allowlist",
			constraint: validation.ListConstraint{
				AllowList: []string{"value1", "value2"},
			},
			value:  "value3",
			expErr: validation.ErrValueNotAllowed,
		},
		{
			name: "should return nil for value in allowlist",
			constraint: validation.ListConstraint{
				AllowList: []string{"value1", "value2"},
			},
			value:  "value1",
			expErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			err := tt.constraint.Validate(tt.value)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestNonEmptyConstraint(t *testing.T) {
	// given
	tests := []struct {
		name       string
		constraint validation.NonEmptyConstraint
		value      any
		expErr     error
	}{
		{
			name:       "should return error for non-string value",
			constraint: validation.NonEmptyConstraint{},
			value:      123,
			expErr:     validation.ErrWrongType,
		},

		{
			name:       "should return error for empty string",
			constraint: validation.NonEmptyConstraint{},
			value:      "",
			expErr:     validation.ErrValueEmpty,
		},
		{
			name:       "should return nil for non-empty string",
			constraint: validation.NonEmptyConstraint{},
			value:      "non-empty",
			expErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			err := tt.constraint.Validate(tt.value)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestNonEmptyKeysConstraint(t *testing.T) {
	// given
	tests := []struct {
		name       string
		constraint validation.NonEmptyKeysConstraint
		value      any
		expErr     error
	}{
		{
			name:       "should return error for non-map value",
			constraint: validation.NonEmptyKeysConstraint{},
			value:      "not-a-map",
			expErr:     validation.ErrWrongType,
		},
		{
			name:       "should return error for map with an empty key",
			constraint: validation.NonEmptyKeysConstraint{},
			value:      Map{"": "value1", "key2": "value2"},
			expErr:     validation.ErrKeyEmpty,
		},
		{
			name:       "should return nil for map with non-empty keys",
			constraint: validation.NonEmptyKeysConstraint{},
			value:      Map{"key1": "value1", "key2": "value2"},
			expErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			err := tt.constraint.Validate(tt.value)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestRegExConstraint(t *testing.T) {
	regExValidator, err := validation.NewRegexConstraint("^KMS_(TenantAdministrator|TenantAuditor)_[A-Za-z0-9-]+$")
	assert.NotNil(t, regExValidator)
	assert.NoError(t, err)

	// given
	tests := []struct {
		name       string
		constraint *validation.RegexConstraint
		value      any
		expErr     error
	}{
		{
			name:       "should return error for non-string value",
			constraint: regExValidator,
			value:      123,
			expErr:     validation.ErrWrongType,
		},
		{
			name:       "should return an error when value does not match regex pattern",
			constraint: regExValidator,
			value:      "some value",
			expErr:     validation.ErrValueNotAllowed,
		},
		{
			name:       "should return nil when value matches regex pattern",
			constraint: regExValidator,
			value:      "KMS_TenantAdministrator_0123abc",
			expErr:     nil,
		},
		{
			name:       "should return nil when all elements in string slice matches regex pattern",
			constraint: regExValidator,
			value:      []string{"KMS_TenantAdministrator_0123abc", "KMS_TenantAuditor_0123abc"},
			expErr:     nil,
		},
		{
			name:       "should return nil when value is nil",
			constraint: regExValidator,
			value:      []string(nil),
			expErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			err := tt.constraint.Validate(tt.value)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}
