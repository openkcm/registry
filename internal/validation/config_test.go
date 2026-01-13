package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/validation"
)

func TestGetValidators(t *testing.T) {
	// given
	tests := []struct {
		name        string
		constraints []validation.Constraint
		expErr      error
	}{
		{
			name:        "should return error when constraints are empty",
			constraints: []validation.Constraint{},
			expErr:      validation.ErrConstraintsMissing,
		},
		{
			name: "should return error when constraint is invalid",
			constraints: []validation.Constraint{
				{
					Type: "unknown",
				},
			},
			expErr: validation.ErrUnknownConstraintType,
		},
		{
			name: "should return validator for valid constraint",
			constraints: []validation.Constraint{
				{
					Type: validation.ConstraintTypeNonEmpty,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			validators, err := validation.GetValidators(tt.constraints)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
			assert.NotEmpty(t, validators)
		})
	}
}

func TestGetValidator(t *testing.T) {
	// given
	tests := []struct {
		name         string
		constraint   validation.Constraint
		expErr       error
		expValidator validation.Validator
	}{
		{
			name: "should return error for empty constraint type",
			constraint: validation.Constraint{
				Type: "",
			},
			expErr: validation.ErrEmptyConstraintType,
		},
		{
			name: "should return error for unknown constraint type",
			constraint: validation.Constraint{
				Type: "unknown",
			},
			expErr: validation.ErrUnknownConstraintType,
		},
		{
			name: "should return error when spec is missing for list constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeList,
			},
			expErr: validation.ErrConstraintSpecMissing,
		},
		{
			name: "should return error when allow list is missing for list constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeList,
				Spec: &validation.ConstraintSpec{},
			},
			expErr: validation.ErrConstraintAllowListMissing,
		},
		{
			name: "should return validator for valid list constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeList,
				Spec: &validation.ConstraintSpec{
					AllowList: []string{"a", "b"},
				},
			},
			expValidator: validation.ListConstraint{},
		},
		{
			name: "should return validator for valid regex constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeRegex,
				Spec: &validation.ConstraintSpec{
					Pattern: "^KMS_(TenantAdministrator|TenantAuditor)_[A-Za-z0-9-]+$",
				},
			},
			expValidator: &validation.RegexConstraint{},
		},
		{
			name: "should return an error when pattern is empty for regex validator",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeRegex,
				Spec: &validation.ConstraintSpec{},
			},
			expErr: validation.ErrConstraintPatternMissing,
		},
		{
			name: "should return validator for valid non-empty constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeNonEmpty,
			},
			expValidator: validation.NonEmptyConstraint{},
		},
		{
			name: "should return validator for valid non-empty-keys constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeNonEmptyKeys,
			},
			expValidator: validation.NonEmptyKeysConstraint{},
		},
		{
			name: "should return validator for valid non-empty-vals constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeNonEmptyVals,
			},
			expValidator: validation.NonEmptyValConstraint{},
		},
		{
			name: "should return error when spec is missing for map-keys constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeMapKeys,
			},
			expErr: validation.ErrConstraintSpecMissing,
		},
		{
			name: "should return error when keys are missing for map-keys constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeMapKeys,
				Spec: &validation.ConstraintSpec{},
			},
			expErr: validation.ErrConstraintKeysMissing,
		},
		{
			name: "should return error when key name is empty for map-keys constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeMapKeys,
				Spec: &validation.ConstraintSpec{
					Keys: []validation.MapKeySpec{
						{Name: ""},
					},
				},
			},
			expErr: validation.ErrConstraintKeyNameMissing,
		},
		{
			name: "should return error when nested constraint is invalid for map-keys constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeMapKeys,
				Spec: &validation.ConstraintSpec{
					Keys: []validation.MapKeySpec{
						{
							Name:     "issuer",
							Required: true,
							Constraints: []validation.Constraint{
								{Type: "unknown"},
							},
						},
					},
				},
			},
			expErr: validation.ErrUnknownConstraintType,
		},
		{
			name: "should return validator for valid map-keys constraint",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeMapKeys,
				Spec: &validation.ConstraintSpec{
					Keys: []validation.MapKeySpec{
						{Name: "issuer", Required: true},
					},
				},
			},
			expValidator: &validation.MapKeysConstraint{},
		},
		{
			name: "should return validator for map-keys constraint with nested constraints",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeMapKeys,
				Spec: &validation.ConstraintSpec{
					Keys: []validation.MapKeySpec{
						{
							Name:     "issuer",
							Required: false,
							Constraints: []validation.Constraint{
								{Type: validation.ConstraintTypeNonEmpty},
							},
						},
						{
							Name:     "application_id",
							Required: true,
							Constraints: []validation.Constraint{
								{
									Type: validation.ConstraintTypeList,
									Spec: &validation.ConstraintSpec{
										AllowList: []string{"id1", "id2"},
									},
								},
							},
						},
					},
				},
			},
			expValidator: &validation.MapKeysConstraint{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			validator, err := tt.constraint.GetValidator()

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
			assert.IsType(t, tt.expValidator, validator)
			assert.NotNil(t, validator)
		})
	}
}
