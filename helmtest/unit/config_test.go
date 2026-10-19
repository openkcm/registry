//go:build helmtest

package unit_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/validation"
)

// loadConfig writes the rendered config.yaml to a directory and loads it the way the
// Registry does at startup (cmd/registry/main.go): the strict loader, which rejects unknown
// keys, with environment variable overrides, then Validate.
func loadConfig(t *testing.T, configYAML string) (*config.Config, error) {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYAML), 0o600))

	cfg := &config.Config{}

	err := commoncfg.NewLoader(cfg,
		commoncfg.WithPaths(dir),
		commoncfg.WithEnvOverride("")).LoadConfig()
	if err != nil {
		return nil, err
	}

	err = cfg.Validate()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// readmeValues writes the example values file of the chart's README to a file and
// returns its path.
func readmeValues(t *testing.T) string {
	t.Helper()

	readme, err := os.ReadFile(chartPath + "/README.md")
	require.NoError(t, err)

	_, rest, found := strings.Cut(string(readme), "Create a values file")
	require.True(t, found, "README.md has no values file example")

	_, rest, found = strings.Cut(rest, "```yaml\n")
	require.True(t, found, "README.md has no values file example")

	example, _, found := strings.Cut(rest, "```")
	require.True(t, found, "the values file example of README.md is not closed")

	path := filepath.Join(t.TempDir(), "values.yaml")
	require.NoError(t, os.WriteFile(path, []byte(example), 0o600)) //nolint:gosec // path is in the test's temporary directory

	return path
}

// TestConfigStrict renders the chart and starts the Registry's configuration handling on
// the rendered config.yaml exactly as the service does at startup: the strict loader,
// Validate and validation.New with the Registry's models.
func TestConfigStrict(t *testing.T) {
	tests := []struct {
		name string
		args func(t *testing.T) []string
	}{
		{
			name: "default values",
			args: func(*testing.T) []string { return nil },
		},
		{
			name: "values file example of the README",
			args: func(t *testing.T) []string {
				t.Helper()
				return []string{"-f", readmeValues(t)}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadConfig(t, renderConfigYAML(t, tt.args(t)...))
			require.NoError(t, err)

			_, err = validation.New(validation.Config{
				Fields: cfg.Validations,
				Models: []validation.Model{
					&model.Tenant{},
					&model.Auth{},
					&model.RegionalSystem{},
					&model.System{},
					&model.TenantConfig{},
				},
			})
			require.NoError(t, err)
		})
	}

	t.Run("the loader rejects unknown keys", func(t *testing.T) {
		_, err := loadConfig(t, renderConfigYAML(t, "--set", "config.database.notARegistrySetting=value"))
		require.ErrorContains(t, err, "invalid keys")
	})
}

// TestConfigReadmeValidations checks that the values file example of the README keeps every
// default validation rule: its validations list replaces the default list.
func TestConfigReadmeValidations(t *testing.T) {
	defaults, err := loadConfig(t, renderConfigYAML(t))
	require.NoError(t, err)

	example, err := loadConfig(t, renderConfigYAML(t, "-f", readmeValues(t)))
	require.NoError(t, err)

	assert.Subset(t, example.Validations, defaults.Validations)
}

// TestConfigLoggerLevelField checks that log records carry the level in the field
// `level`. The Registry's own default name of that field is `info`.
func TestConfigLoggerLevelField(t *testing.T) {
	cfg, err := loadConfig(t, renderConfigYAML(t))
	require.NoError(t, err)

	var out bytes.Buffer

	handler, err := logger.InitHandlerWithWriter(&out, cfg.Logger, cfg.Application)
	require.NoError(t, err)

	slog.New(handler).Info("hello")

	var record map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &record), out.String())
	assert.Equal(t, "INFO", record["level"])
	assert.NotContains(t, record, "info")
}
