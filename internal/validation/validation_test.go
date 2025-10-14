package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/validation"
)

func TestCheckIDs(t *testing.T) {
	// given
	tests := []struct {
		name   string
		fields []validation.StructField
		ids    map[validation.ID]struct{}
		expErr error
	}{
		{
			name:   "should return nil for empty fields and ids",
			fields: []validation.StructField{},
			ids:    map[validation.ID]struct{}{},
			expErr: nil,
		},
		{
			name: "should return nil if all ids exists",
			fields: []validation.StructField{
				{ID: "Field1"},
				{ID: "Field2"},
			},
			ids: map[validation.ID]struct{}{
				"Field1": {},
				"Field2": {},
			},
			expErr: nil,
		},
		{
			name: "should return error for missing ids",
			fields: []validation.StructField{
				{ID: "Field1"},
				{ID: "Field2"},
			},
			ids: map[validation.ID]struct{}{
				"Field1": {},
			},
			expErr: validation.ErrMissingID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			v, err := validation.New()
			assert.NoError(t, err)
			v.AddStructFields(tt.fields...)

			// when
			err = v.CheckIDs(tt.ids)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}

	t.Run("should consider omitIDCheck flag", func(t *testing.T) {
		// given
		v, err := validation.New()
		assert.NoError(t, err)
		err = v.AddConfigFields(validation.ConfigField{
			ID:          "Field1",
			OmitIDCheck: true,
		})
		assert.NoError(t, err)

		// when
		err = v.CheckIDs()

		// then
		assert.NoError(t, err)
	})
}

func TestValidate(t *testing.T) {
	// given
	v, err := validation.New()
	assert.NoError(t, err)

	tests := []struct {
		name       string
		constraint validation.Constraint
		value      any
		expErr     error
	}{
		{
			name: "should validate non-empty constraint with non-empty string",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeNonEmpty,
			},
			value:  "non-empty",
			expErr: nil,
		},
		{
			name: "should return error for non-empty constraint with empty string",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeNonEmpty,
			},
			value:  "",
			expErr: validation.ErrValueEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := validation.ID(tt.name)

			err = v.AddConfigFields(validation.ConfigField{
				ID: id,
				Constraints: []validation.Constraint{
					tt.constraint,
				},
			})
			assert.NoError(t, err)

			// when
			err = v.Validate(id, tt.value)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}

	t.Run("should ignore not existing ID", func(t *testing.T) {
		// when
		err = v.Validate("non-existing-id", "value")

		// then
		assert.NoError(t, err)
	})
}

func TestValidateAll(t *testing.T) {
	// given
	v, err := validation.New()
	assert.NoError(t, err)

	tests := []struct {
		name       string
		constraint validation.Constraint
		value      any
		expErr     error
	}{
		{
			name: "should validate non-empty constraint with non-empty string",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeNonEmpty,
			},
			value:  "non-empty",
			expErr: nil,
		},
		{
			name: "should return error for non-empty constraint with empty string",
			constraint: validation.Constraint{
				Type: validation.ConstraintTypeNonEmpty,
			},
			value:  "",
			expErr: validation.ErrValueEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := validation.ID(tt.name)

			err = v.AddConfigFields(validation.ConfigField{
				ID: id,
				Constraints: []validation.Constraint{
					tt.constraint,
				},
			})
			assert.NoError(t, err)

			// when
			err = v.ValidateAll(map[validation.ID]any{
				id: tt.value,
			})

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}
