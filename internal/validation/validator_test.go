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

func TestNonEmptyValConstraint(t *testing.T) {
	// given
	tests := []struct {
		name       string
		constraint validation.NonEmptyValConstraint
		value      any
		expErr     error
	}{
		{
			name:       "should return error for non-map value",
			constraint: validation.NonEmptyValConstraint{},
			value:      "not-a-map",
			expErr:     validation.ErrWrongType,
		},
		{
			name:       "should return error for map with an empty VAL",
			constraint: validation.NonEmptyValConstraint{},
			value:      Map{"KEY1": "value1", "key2": ""},
			expErr:     validation.ErrValueEmpty,
		},
		{
			name:       "should return nil for map with non-empty keys",
			constraint: validation.NonEmptyValConstraint{},
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

func TestMapKeysConstraint(t *testing.T) {
	// given
	tests := []struct {
		name      string
		keys      []validation.MapKeySpec
		value     any
		expErr    error
		expErrMsg string
	}{
		{
			name: "should return error for non-map value",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: true},
			},
			value:  "not-a-map",
			expErr: validation.ErrWrongType,
		},
		{
			name: "should return error when required key is missing",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: true},
			},
			value:  map[string]string{"other": "value"},
			expErr: validation.ErrKeyMissing,
		},
		{
			name: "should return nil when required key is present",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: true},
			},
			value:  map[string]string{"issuer": "https://example.com"},
			expErr: nil,
		},
		{
			name: "should return nil when required key is present with empty value and no non-empty constraint",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: true},
			},
			value:  map[string]string{"issuer": ""},
			expErr: nil,
		},
		{
			name: "should return nil when optional key is missing",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: false},
			},
			value:  map[string]string{"other": "value"},
			expErr: nil,
		},
		{
			name: "should return nil when optional key is present with valid value",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: false},
			},
			value:  map[string]string{"issuer": "https://example.com"},
			expErr: nil,
		},
		{
			name: "should return error when nested non-empty constraint fails",
			keys: []validation.MapKeySpec{
				{
					Name:     "issuer",
					Required: true,
					Constraints: []validation.Constraint{
						{Type: validation.ConstraintTypeNonEmpty},
					},
				},
			},
			value:  map[string]string{"issuer": ""},
			expErr: validation.ErrValueEmpty,
		},
		{
			name: "should return nil when nested non-empty constraint passes",
			keys: []validation.MapKeySpec{
				{
					Name:     "issuer",
					Required: true,
					Constraints: []validation.Constraint{
						{Type: validation.ConstraintTypeNonEmpty},
					},
				},
			},
			value:  map[string]string{"issuer": "https://example.com"},
			expErr: nil,
		},
		{
			name: "should return error when nested list constraint fails",
			keys: []validation.MapKeySpec{
				{
					Name:     "type",
					Required: true,
					Constraints: []validation.Constraint{
						{
							Type: validation.ConstraintTypeList,
							Spec: &validation.ConstraintSpec{
								AllowList: []string{"oidc", "saml"},
							},
						},
					},
				},
			},
			value:  map[string]string{"type": "basic"},
			expErr: validation.ErrValueNotAllowed,
		},
		{
			name: "should return nil when nested list constraint passes",
			keys: []validation.MapKeySpec{
				{
					Name:     "type",
					Required: true,
					Constraints: []validation.Constraint{
						{
							Type: validation.ConstraintTypeList,
							Spec: &validation.ConstraintSpec{
								AllowList: []string{"oidc", "saml"},
							},
						},
					},
				},
			},
			value:  map[string]string{"type": "oidc"},
			expErr: nil,
		},
		{
			name: "should return error when nested regex constraint fails",
			keys: []validation.MapKeySpec{
				{
					Name:     "issuer",
					Required: true,
					Constraints: []validation.Constraint{
						{
							Type: validation.ConstraintTypeRegex,
							Spec: &validation.ConstraintSpec{
								Pattern: "^https://",
							},
						},
					},
				},
			},
			value:  map[string]string{"issuer": "http://insecure.com"},
			expErr: validation.ErrValueNotAllowed,
		},
		{
			name: "should return nil when nested regex constraint passes",
			keys: []validation.MapKeySpec{
				{
					Name:     "issuer",
					Required: true,
					Constraints: []validation.Constraint{
						{
							Type: validation.ConstraintTypeRegex,
							Spec: &validation.ConstraintSpec{
								Pattern: "^https://",
							},
						},
					},
				},
			},
			value:  map[string]string{"issuer": "https://secure.com"},
			expErr: nil,
		},
		{
			name: "should validate multiple keys",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: true},
				{Name: "app_tid", Required: true},
			},
			value:  map[string]string{"issuer": "https://example.com", "app_tid": "tid123"},
			expErr: nil,
		},
		{
			name: "should return error when one of multiple required keys is missing",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: true},
				{Name: "app_tid", Required: true},
			},
			value:  map[string]string{"issuer": "https://example.com"},
			expErr: validation.ErrKeyMissing,
		},
		{
			name: "should allow extra keys not in spec",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: true},
			},
			value:  map[string]string{"issuer": "https://example.com", "extra": "allowed"},
			expErr: nil,
		},
		{
			name: "should work with map[string]any",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: true},
			},
			value:  map[string]any{"issuer": "https://example.com"},
			expErr: nil,
		},
		{
			name: "should work with Map interface",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: true},
			},
			value:  Map{"issuer": "https://example.com"},
			expErr: nil,
		},
		{
			name: "should skip nested constraints when optional key is missing",
			keys: []validation.MapKeySpec{
				{
					Name:     "issuer",
					Required: false,
					Constraints: []validation.Constraint{
						{Type: validation.ConstraintTypeNonEmpty},
					},
				},
			},
			value:  map[string]string{"other": "value"},
			expErr: nil,
		},
		{
			name: "should run nested constraints when optional key is present",
			keys: []validation.MapKeySpec{
				{
					Name:     "issuer",
					Required: false,
					Constraints: []validation.Constraint{
						{Type: validation.ConstraintTypeNonEmpty},
					},
				},
			},
			value:  map[string]string{"issuer": ""},
			expErr: validation.ErrValueEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			constraint, err := validation.NewMapKeysConstraint(tt.keys)
			assert.NoError(t, err)
			assert.NotNil(t, constraint)

			err = constraint.Validate(tt.value)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestNewMapKeysConstraint(t *testing.T) {
	// given
	tests := []struct {
		name   string
		keys   []validation.MapKeySpec
		expErr error
	}{
		{
			name: "should return error when key name is empty",
			keys: []validation.MapKeySpec{
				{Name: ""},
			},
			expErr: validation.ErrConstraintKeyNameMissing,
		},
		{
			name: "should return error when nested constraint is invalid",
			keys: []validation.MapKeySpec{
				{
					Name: "issuer",
					Constraints: []validation.Constraint{
						{Type: "unknown"},
					},
				},
			},
			expErr: validation.ErrUnknownConstraintType,
		},
		{
			name: "should return constraint for valid keys",
			keys: []validation.MapKeySpec{
				{Name: "issuer", Required: true},
				{Name: "app_tid", Required: false},
			},
			expErr: nil,
		},
		{
			name: "should return constraint for keys with nested constraints",
			keys: []validation.MapKeySpec{
				{
					Name:     "issuer",
					Required: true,
					Constraints: []validation.Constraint{
						{Type: validation.ConstraintTypeNonEmpty},
					},
				},
			},
			expErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			constraint, err := validation.NewMapKeysConstraint(tt.keys)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				assert.Nil(t, constraint)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, constraint)
		})
	}
}
