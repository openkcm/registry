//go:build helmtest

// Package unit_test renders the Registry Helm chart with `helm template` and checks the
// output. It needs the helm command, but no cluster. Run from the repository root:
//
//	go test -count=1 -tags=helmtest ./helmtest/unit/...
package unit_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

const (
	chartPath   = "../../charts/registry"
	releaseName = "registry"
)

// TestMain renders the chart into the namespace "default", whatever the current
// kubeconfig context is.
func TestMain(m *testing.M) {
	err := os.Setenv("HELM_NAMESPACE", "default")
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

// render runs `helm template` with the given --set/-f arguments and returns the output.
func render(t *testing.T, args ...string) string {
	t.Helper()

	cmdArgs := append([]string{"template", releaseName, chartPath}, args...)
	cmd := exec.CommandContext(t.Context(), "helm", cmdArgs...) //nolint:gosec // arguments come from the tests

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.NoError(t, err, "helm template failed: %s", stderr.String())

	return stdout.String()
}

// object decodes the first rendered document of the given kind into out.
func object(t *testing.T, output, kind string, out any) {
	t.Helper()

	dec := yaml.NewDecoder(strings.NewReader(output))
	for {
		var doc yaml.Node

		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err)

		var header struct {
			Kind string `yaml:"kind"`
		}

		require.NoError(t, doc.Decode(&header))

		if header.Kind == kind {
			require.NoError(t, doc.Decode(out))
			return
		}
	}

	t.Fatalf("no %s in the rendered chart:\n%s", kind, output)
}

// configMap is the part of the configuration ConfigMap that the tests check.
type configMap struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Immutable bool              `yaml:"immutable"`
	Data      map[string]string `yaml:"data"`
}

// deployment is the part of the Deployment that the tests check.
type deployment struct {
	Spec struct {
		Template struct {
			Metadata struct {
				Annotations map[string]string `yaml:"annotations"`
			} `yaml:"metadata"`
			Spec struct {
				Containers []struct {
					Name  string `yaml:"name"`
					Image string `yaml:"image"`
				} `yaml:"containers"`
				Volumes []struct {
					Name      string `yaml:"name"`
					ConfigMap *struct {
						Name string `yaml:"name"`
					} `yaml:"configMap"`
				} `yaml:"volumes"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// renderConfigMap renders the chart and returns its configuration ConfigMap.
func renderConfigMap(t *testing.T, args ...string) configMap {
	t.Helper()

	var cm configMap
	object(t, render(t, args...), "ConfigMap", &cm)

	return cm
}

// renderDeployment renders the chart and returns its Deployment.
func renderDeployment(t *testing.T, args ...string) deployment {
	t.Helper()

	var d deployment
	object(t, render(t, args...), "Deployment", &d)

	return d
}

// appVersion returns the chart's appVersion, the default image tag.
func appVersion(t *testing.T) string {
	t.Helper()

	chart, err := os.ReadFile(chartPath + "/Chart.yaml")
	require.NoError(t, err)

	var meta struct {
		AppVersion string `yaml:"appVersion"`
	}

	require.NoError(t, yaml.Unmarshal(chart, &meta))
	require.NotEmpty(t, meta.AppVersion, "Chart.yaml has no appVersion")

	return meta.AppVersion
}
