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

// startConfig renders the chart with the given --set/-f arguments and starts the Registry's
// configuration handling on the rendered config.yaml exactly as the service does at startup
// (cmd/registry/main.go): the strict loader, Validate and validation.New with the Registry's
// models.
func startConfig(t *testing.T, args ...string) error {
	t.Helper()

	cfg, err := loadConfig(t, renderConfigYAML(t, args...))
	if err != nil {
		return err
	}

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

	return err
}

// TestConfigStrict checks that the Registry starts with the configuration rendered from the
// default values, the README example and every example documented under `config` in
// values.yaml (testdata/values-all-config*.yaml).
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
		{
			name: "every documented setting, first example of each alternative",
			args: func(*testing.T) []string { return []string{"-f", "testdata/values-all-config.yaml"} },
		},
		{
			name: "second example of each alternative",
			args: func(*testing.T) []string { return []string{"-f", "testdata/values-all-config-alt1.yaml"} },
		},
		{
			name: "third example of each alternative",
			args: func(*testing.T) []string { return []string{"-f", "testdata/values-all-config-alt2.yaml"} },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, startConfig(t, tt.args(t)...))
		})
	}

	t.Run("the loader rejects unknown keys", func(t *testing.T) {
		_, err := loadConfig(t, renderConfigYAML(t, "--set", "config.database.notARegistrySetting=value"))
		require.ErrorContains(t, err, "invalid keys")
	})
}

// TestConfigStrictRejects checks limits that values.yaml documents for `config`: the Registry
// does not start with a value outside them.
func TestConfigStrictRejects(t *testing.T) {
	const allConfig = "testdata/values-all-config.yaml"

	// target returns --set-json for one orbital target with the given connection type and
	// auth block (JSON).
	target := func(connectionType, auth string) string {
		return `config.orbital.targets=[{"region":"emea","connection":{"type":"` + connectionType +
			`","amqp":{"url":"amqps://broker:5671","target":"/queues/a","source":"/queues/b"},` +
			`"auth":` + auth + `}}]`
	}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "connection type other than amqp",
			args:    []string{"--set-json", target("kafka", `{"type":"none"}`)},
			wantErr: "connection type is not supported",
		},
		{
			name:    "auth type other than mtls and none",
			args:    []string{"--set-json", target("amqp", `{"type":"basic"}`)},
			wantErr: "authentication type is not supported",
		},
		{
			name: "mtls without caFile",
			args: []string{"--set-json", target("amqp",
				`{"type":"mtls","mtls":{"certFile":"/tls.crt","keyFile":"/tls.key"}}`)},
			wantErr: "CA file must not be empty",
		},
		{
			name: "amqp target without source",
			args: []string{"--set-json", `config.orbital.targets=[{"region":"emea","connection":` +
				`{"type":"amqp","amqp":{"url":"amqps://broker:5671","target":"/queues/a"},` +
				`"auth":{"type":"none"}}}]`},
			wantErr: "source must not be empty",
		},
		{
			name:    "taskLimitNum 0",
			args:    []string{"--set", "config.orbital.taskLimitNum=0"},
			wantErr: "task limit number must be greater than zero",
		},
		{
			name:    "negative confirmJobAfter",
			args:    []string{"--set", "config.orbital.confirmJobAfter=-1s"},
			wantErr: "confirm job delay must be equal or greater than zero",
		},
		{
			name: "unknown worker name",
			args: []string{"--set-json",
				`config.orbital.workers=[{"name":"cleanup","noOfWorkers":1,"execInterval":"1s","timeout":"5s"}]`},
			wantErr: "worker name is not supported",
		},
		{
			name: "worker without timeout",
			args: []string{"--set-json",
				`config.orbital.workers=[{"name":"reconcile","noOfWorkers":1,"execInterval":"1s"}]`},
			wantErr: "timeout must be greater than zero",
		},
		{
			name: "validation of an unknown field without skipIfNotExists",
			args: []string{"--set-json",
				`config.validations=[{"id":"Tenant.Contact","constraints":[{"type":"non-empty"}]}]`},
			wantErr: "id must exist: Tenant.Contact",
		},
		{
			name: "unknown constraint type",
			args: []string{"--set-json",
				`config.validations=[{"id":"Tenant.Region","constraints":[{"type":"enum"}]}]`},
			wantErr: "unknown constraint type",
		},
		{
			name: "list constraint without allowList",
			args: []string{"--set-json",
				`config.validations=[{"id":"Tenant.Region","constraints":[{"type":"list","spec":{}}]}]`},
			wantErr: "constraint allow list is missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := startConfig(t, append([]string{"-f", allConfig}, tt.args...)...)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
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
