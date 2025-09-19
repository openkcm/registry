package model_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/model"
)

func TestIsEmpty(t *testing.T) {
	tests := map[string]struct {
		value    any
		expected bool
	}{
		"nil value":        {value: nil, expected: true},
		"empty string":     {value: "", expected: true},
		"non-empty string": {value: "hello", expected: false},
		"empty slice":      {value: []string{}, expected: true},
		"non-empty slice":  {value: []string{"item"}, expected: false},
		"empty map":        {value: map[string]string{}, expected: true},
		"non-empty map":    {value: map[string]string{"key": "value"}, expected: false},
		"nil pointer":      {value: (*string)(nil), expected: true},
		"non-nil pointer":  {value: &[]string{"test"}[0], expected: false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := model.IsEmpty(reflect.ValueOf(test.value))
			assert.Equal(t, test.expected, result)
		})
	}
}
