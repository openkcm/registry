package model_test

import (
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
)

type TypeWithLabels struct {
	Labels model.Labels `validators:"map"`
}

func TestLabelsValidation(t *testing.T) {
	typeWithLabels := TypeWithLabels{}
	model.RegisterValidatorsForTypes(typeWithLabels)
	defer model.ClearGlobalTypeValidators()

	tests := map[string]struct {
		labels    model.Labels
		expectErr bool
		err       error
	}{
		"Valid labels": {
			labels: map[string]string{
				"datacenter": "eu10",
			},
			expectErr: false,
		},
		"Empty key": {
			labels: map[string]string{
				"": "eu10",
			},
			expectErr: true,
			err:       model.ErrFieldContainsEmptyKeys,
		},
		"Empty value": {
			labels: map[string]string{
				"datacenter": "",
			},
			expectErr: true,
			err:       model.ErrFieldContainsEmptyValues,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			typeWithLabels.Labels = test.labels
			err := model.ValidateField(&typeWithLabels, &typeWithLabels.Labels)
			if test.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLabelsValue(t *testing.T) {
	tests := map[string]struct {
		labels    model.Labels
		expectErr bool
		expected  driver.Value
	}{
		"Nil labels": {
			labels:    nil,
			expectErr: false,
			expected:  nil,
		},
		"Valid labels": {
			labels:    model.Labels{"key": "value", "foo": "bar"},
			expectErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			val, err := tc.labels.Value()
			if tc.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tc.labels != nil {
				expectedBytes, err := json.Marshal(tc.labels)
				if err != nil {
					t.Fatalf("failed to marshal labels: %v", err)
				}

				assert.Equal(t, expectedBytes, val)
			} else {
				assert.Nil(t, val)
			}
		})
	}
}

func TestLabelsScan(t *testing.T) {
	tests := map[string]struct {
		src       interface{}
		expectErr bool
		expected  model.Labels
	}{
		"Nil src": {
			src:       nil,
			expectErr: false,
			expected:  model.Labels{},
		},
		"Valid JSON": {
			src:       []byte(`{"key":"value","foo":"bar"}`),
			expectErr: false,
			expected:  model.Labels{"key": "value", "foo": "bar"},
		},
		"Invalid type": {
			src:       "not a []byte",
			expectErr: true,
		},
		"Invalid JSON": {
			src:       []byte(`invalid json`),
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var l model.Labels

			err := l.Scan(tc.src)
			if tc.expectErr {
				assert.Error(t, err, "expected error but got none")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, l)
			}
		})
	}
}
