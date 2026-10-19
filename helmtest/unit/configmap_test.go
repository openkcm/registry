//go:build helmtest

package unit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// configSections are the top-level sections templates/configmap.yaml writes, in order.
var configSections = []string{
	"application", "status", "logger", "telemetry", "grpcServer", "database", "orbital", "validations",
}

// renderConfigYAML renders the chart and returns config.yaml of the configuration ConfigMap.
func renderConfigYAML(t *testing.T, args ...string) string {
	t.Helper()

	cfg, ok := renderConfigMap(t, args...).Data["config.yaml"]
	require.True(t, ok, "the ConfigMap has no config.yaml")

	return cfg
}

// topLevelKeys returns the top-level keys of a YAML document, in order.
func topLevelKeys(t *testing.T, doc string) []string {
	t.Helper()

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(doc), &root))
	require.Len(t, root.Content, 1)

	mapping := root.Content[0]
	require.Equal(t, yaml.MappingNode, mapping.Kind)

	keys := make([]string, 0, len(mapping.Content)/2)
	for i := 0; i < len(mapping.Content); i += 2 {
		keys = append(keys, mapping.Content[i].Value)
	}

	return keys
}

// TestConfigSections checks that config.yaml has exactly the sections the Registry reads,
// and that keys under config that are not listed in templates/configmap.yaml are not written.
func TestConfigSections(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		assert.Equal(t, configSections, topLevelKeys(t, renderConfigYAML(t)))
	})

	t.Run("keys not listed are not written", func(t *testing.T) {
		cfg := renderConfigYAML(t,
			"--set", "config.isImmutable=true",
			"--set", "config.notARegistrySetting.key=value",
		)
		assert.Equal(t, configSections, topLevelKeys(t, cfg))
		assert.NotContains(t, cfg, "isImmutable")
		assert.NotContains(t, cfg, "notARegistrySetting")
	})
}
