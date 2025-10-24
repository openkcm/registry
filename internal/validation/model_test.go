package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/validation"
)

type Model struct {
	Field string     `validationID:"Model.Field"`
	Child ChildModel `validationID:"Model.Child"`
	Map   Map        `validationID:"Model.Map"`
}

func (m Model) Validations() []validation.Field {
	return []validation.Field{}
}

type ChildModel struct {
	Field string `validationID:"Field"`
}

type Map map[string]string

func (m Map) Map() map[string]any {
	res := make(map[string]any)
	for k, v := range m {
		res[k] = v
	}
	return res
}

func TestGetIDs(t *testing.T) {
	// when
	ids, err := validation.GetIDs(Model{})

	// then
	assert.NoError(t, err)

	expIDs := []validation.ID{
		"Model.Field",
		"Model.Child",
		"Model.Child.Field",
		"Model.Map",
	}
	for _, id := range expIDs {
		_, exists := ids[id]
		assert.True(t, exists, "expected ID %s to be present", id)
	}
}

func TestGetValuesByID(t *testing.T) {
	// given
	m := Model{
		Field: "value",
		Child: ChildModel{
			Field: "childValue",
		},
		Map: Map{
			"Key": "mapValue",
		},
	}

	// when
	valuesByID, err := validation.GetValues(m)

	// then
	assert.NoError(t, err)
	assert.Equal(t, "value", valuesByID["Model.Field"])
	assert.Equal(t, "childValue", valuesByID["Model.Child.Field"])
	assert.Equal(t, "mapValue", valuesByID["Model.Map.Key"])
}

func TestGetValuesByID_NilMap(t *testing.T) {
	// given
	m := Model{
		Map: nil,
	}

	// when
	valuesByID, err := validation.GetValues(m)

	// then
	assert.NoError(t, err)
	assert.Nil(t, valuesByID["Model.Map"])
}
