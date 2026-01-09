package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/validation"
)

type Model struct {
	Field string            `validationID:"Model.Field"`
	Child ChildModel        `validationID:"Model.Child"`
	Map   map[string]string `validationID:"Model.Map"`
}

func (m Model) Validations() []validation.Field {
	return []validation.Field{}
}

type ChildModel struct {
	Field string `validationID:"Field"`
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

	// Map keys should NOT be flattened into IDs
	_, exists := ids["Model.Map.Key"]
	assert.False(t, exists, "Model.Map.Key should NOT be present as an ID")
}

func TestGetValuesByID(t *testing.T) {
	// given
	m := Model{
		Field: "value",
		Child: ChildModel{
			Field: "childValue",
		},
		Map: map[string]string{
			"Key": "mapValue",
		},
	}

	// when
	valuesByID, err := validation.GetValues(m)

	// then
	assert.NoError(t, err)
	assert.Equal(t, "value", valuesByID["Model.Field"])
	assert.Equal(t, "childValue", valuesByID["Model.Child.Field"])

	// Map should be returned as the whole map, not flattened
	mapVal, ok := valuesByID["Model.Map"].(map[string]string)
	assert.True(t, ok, "Model.Map should be a map[string]string")
	assert.Equal(t, "mapValue", mapVal["Key"])

	// Map keys should NOT be flattened
	_, exists := valuesByID["Model.Map.Key"]
	assert.False(t, exists, "Model.Map.Key should NOT exist as a separate value")
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
