package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/validation"
)

type MockModel struct {
	Fields []validation.Field
}

func (m *MockModel) Validations() []validation.Field {
	return m.Fields
}

func TestNew(t *testing.T) {
	// given
	tests := []struct {
		name   string
		config validation.Config
		expErr error
	}{
		{
			name: "should return error for invalid config field",
			config: validation.Config{
				Fields: []validation.ConfigField{
					{
						ID: "",
					},
				},
			},
			expErr: validation.ErrEmptyID,
		},
		{
			name: "should return error for invalid model validation",
			config: validation.Config{
				Models: []validation.Model{
					&MockModel{
						Fields: []validation.Field{
							{
								ID: "",
							},
						},
					},
				},
			},
			expErr: validation.ErrEmptyID,
		},
		{
			name: "should return error for unknown validation ID",
			config: validation.Config{
				Fields: []validation.ConfigField{
					{
						ID: "Unknown.ID",
						Constraints: []validation.Constraint{
							{
								Type: validation.ConstraintTypeNonEmpty,
							},
						},
					},
				},
			},
			expErr: validation.ErrIDMustExist,
		},
		{
			name:   "should pass for empty config",
			config: validation.Config{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			v, err := validation.New(tt.config)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				assert.Nil(t, v)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, v)
		})
	}
}

func TestRegisterConfig(t *testing.T) {
	// given
	v, err := validation.New(validation.Config{})
	assert.NoError(t, err)

	validConfigField := validation.ConfigField{
		ID: "Field",
		Constraints: []validation.Constraint{
			{
				Type: validation.ConstraintTypeNonEmpty,
			},
		},
	}

	tests := []struct {
		name   string
		config validation.ConfigField
		expErr error
	}{
		{
			name: "should return error for empty ID",
			config: validation.ConfigField{
				ID: "",
			},
			expErr: validation.ErrEmptyID,
		},
		{
			name: "should return error for invalid constraint",
			config: validation.ConfigField{
				ID: "Field",
				Constraints: []validation.Constraint{
					{
						Type: "",
					},
				},
			},
			expErr: validation.ErrEmptyConstraintType,
		},
		{
			name:   "should register valid config field",
			config: validConfigField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			err := v.RegisterConfig(tt.config)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)

			// assert non-empty constraint is applied
			err = v.Validate(tt.config.ID, "")
			assert.Error(t, err)

			err = v.Validate(tt.config.ID, "value")
			assert.NoError(t, err)
		})
	}

	t.Run("should append constraints for existing ID", func(t *testing.T) {
		// given
		validConfigField = validation.ConfigField{
			ID: validConfigField.ID,
			Constraints: []validation.Constraint{
				{
					Type: validation.ConstraintTypeList,
					Spec: &validation.ConstraintSpec{
						AllowList: []string{"allowedValue"},
					},
				},
			},
		}

		// when
		err := v.RegisterConfig(validConfigField)

		// then
		assert.NoError(t, err)

		// assert both constraints are applied
		err = v.Validate(validConfigField.ID, "")
		assert.Error(t, err) // non-empty constraint

		err = v.Validate(validConfigField.ID, "notAllowedValue")
		assert.Error(t, err) // list constraint

		err = v.Validate(validConfigField.ID, "allowedValue")
		assert.NoError(t, err)
	})
}

func TestRegister(t *testing.T) {
	// given
	v, err := validation.New(validation.Config{})
	assert.NoError(t, err)

	validField := validation.Field{
		ID: "Field",
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	}

	tests := []struct {
		name   string
		field  validation.Field
		expErr error
	}{
		{
			name: "should return error for empty ID",
			field: validation.Field{
				ID: "",
			},
			expErr: validation.ErrEmptyID,
		},
		{
			name: "should return error for missing validators",
			field: validation.Field{
				ID:         "Field",
				Validators: []validation.Validator{},
			},
			expErr: validation.ErrValidatorsMissing,
		},
		{
			name:  "should register valid field",
			field: validField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			err := v.Register(tt.field)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)

			// assert non-empty constraint is applied
			err = v.Validate(tt.field.ID, "")
			assert.Error(t, err)

			err = v.Validate(tt.field.ID, "value")
			assert.NoError(t, err)
		})
	}

	t.Run("should append validators for existing ID", func(t *testing.T) {
		// given
		validField = validation.Field{
			ID: validField.ID,
			Validators: []validation.Validator{
				validation.ListConstraint{
					AllowList: []string{"allowedValue"},
				},
			},
		}

		// when
		err := v.Register(validField)

		// then
		assert.NoError(t, err)

		// assert both validators are applied
		err = v.Validate(validField.ID, "")
		assert.Error(t, err) // non-empty constraint

		err = v.Validate(validField.ID, "notAllowedValue")
		assert.Error(t, err) // list constraint

		err = v.Validate(validField.ID, "allowedValue")
		assert.NoError(t, err)
	})
}

func TestCheckIDs(t *testing.T) {
	// given
	tests := []struct {
		name   string
		fields []validation.Field
		ids    map[validation.ID]struct{}
		expErr error
	}{
		{
			name:   "should return nil for empty fields and ids",
			fields: []validation.Field{},
			ids:    map[validation.ID]struct{}{},
			expErr: nil,
		},
		{
			name: "should return nil if all ids exists",
			fields: []validation.Field{
				{
					ID: "Field1",
					Validators: []validation.Validator{
						validation.NonEmptyConstraint{},
					},
				},
				{
					ID: "Field2",
					Validators: []validation.Validator{
						validation.NonEmptyConstraint{},
					},
				},
			},
			ids: map[validation.ID]struct{}{
				"Field1": {},
				"Field2": {},
			},
			expErr: nil,
		},
		{
			name: "should return error for missing ids",
			fields: []validation.Field{
				{
					ID: "Field1",
					Validators: []validation.Validator{
						validation.NonEmptyConstraint{},
					},
				},
				{
					ID: "Field2",
					Validators: []validation.Validator{
						validation.NonEmptyConstraint{},
					},
				},
			},
			ids: map[validation.ID]struct{}{
				"Field1": {},
			},
			expErr: validation.ErrIDMustExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			v, err := validation.New(validation.Config{})
			assert.NoError(t, err)
			err = v.Register(tt.fields...)
			assert.NoError(t, err)

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

	t.Run("should consider skipIfNotExists flag", func(t *testing.T) {
		// given
		fieldID := validation.ID("Field1")

		v, err := validation.New(validation.Config{})
		assert.NoError(t, err)
		err = v.RegisterConfig(validation.ConfigField{
			ID:              fieldID,
			SkipIfNotExists: true,
			Constraints: []validation.Constraint{
				{
					Type: validation.ConstraintTypeNonEmpty,
				},
			},
		})
		assert.NoError(t, err)

		// when
		err = v.CheckIDs()

		// then
		assert.NoError(t, err)

		t.Run("and return error when one field has skipIfNotExists false", func(t *testing.T) {
			// given
			err = v.RegisterConfig(validation.ConfigField{
				ID:              fieldID,
				SkipIfNotExists: false,
				Constraints: []validation.Constraint{
					{
						Type: validation.ConstraintTypeNonEmpty,
					},
				},
			})
			assert.NoError(t, err)

			// when
			err = v.CheckIDs()

			// then
			assert.ErrorIs(t, err, validation.ErrIDMustExist)
		})
	})
}

func TestValidate(t *testing.T) {
	// given
	v, err := validation.New(validation.Config{})
	assert.NoError(t, err)
	fieldName := validation.ID("Field")
	err = v.Register(validation.Field{
		ID: fieldName,
		Validators: []validation.Validator{
			validation.NonEmptyConstraint{},
		},
	})
	assert.NoError(t, err)

	tests := []struct {
		name   string
		id     validation.ID
		value  any
		expErr error
	}{
		{
			name: "should return nil for non-registered ID",
			id:   "non-registered-id",
		},
		{
			name:   "should return error for invalid value",
			id:     fieldName,
			value:  "",
			expErr: validation.ErrValueEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			err = v.Validate(tt.id, tt.value)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestValidateAll(t *testing.T) {
	// given
	v, err := validation.New(validation.Config{})
	assert.NoError(t, err)

	field1, field2 := validation.ID("Field1"), validation.ID("Field2")
	for _, id := range []validation.ID{field1, field2} {
		err = v.Register(validation.Field{
			ID: id,
			Validators: []validation.Validator{
				validation.NonEmptyConstraint{},
			},
		})
		assert.NoError(t, err)
	}

	tests := []struct {
		name       string
		valuesByID map[validation.ID]any
		expErr     error
	}{
		{
			name: "should return nil for valid values",
			valuesByID: map[validation.ID]any{
				field1: "value1",
				field2: "value2",
			},
			expErr: nil,
		},
		{
			name: "should return error for invalid values",
			valuesByID: map[validation.ID]any{
				field1: "",
				field2: "value2",
			},
			expErr: validation.ErrValueEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			err = v.ValidateAll(tt.valuesByID)

			// then
			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}
