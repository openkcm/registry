package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/validation"
)

type Struct struct {
	Field string      `validationID:"Struct.Field"`
	Child ChildStruct `validationID:"Struct.Child"`
	Map   Map         `validationID:"Struct.Map"`
}

func (s Struct) Fields() []validation.StructField {
	return []validation.StructField{}
}

type ChildStruct struct {
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
	ids, err := validation.GetIDs(Struct{})

	// then
	assert.NoError(t, err)

	expIDs := []validation.ID{
		"Struct.Field",
		"Struct.Child",
		"Struct.Child.Field",
		"Struct.Map",
	}
	for _, id := range expIDs {
		_, exists := ids[id]
		assert.True(t, exists, "expected ID %s to be present", id)
	}
}

func TestGetValuesByID(t *testing.T) {
	// given
	s := Struct{
		Field: "value",
		Child: ChildStruct{
			Field: "childValue",
		},
		Map: Map{
			"Key": "mapValue",
		},
	}

	// when
	valuesByID, err := validation.GetValuesByID(s)

	// then
	assert.NoError(t, err)
	assert.Equal(t, "value", valuesByID["Struct.Field"])
	assert.Equal(t, "childValue", valuesByID["Struct.Child.Field"])
	assert.Equal(t, "mapValue", valuesByID["Struct.Map.Key"])
}

func TestGetValuesByID_NilMap(t *testing.T) {
	// given
	s := Struct{
		Map: nil,
	}

	// when
	valuesByID, err := validation.GetValuesByID(s)

	// then
	assert.NoError(t, err)
	assert.Nil(t, valuesByID["Struct.Map"])
}
