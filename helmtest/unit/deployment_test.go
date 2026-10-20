//go:build helmtest

package unit_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const digest = "sha256:0e072dddd1f7f8fc8909a2ca6f65e76c5f0d2fcfb8be47935ae3457e8bbceb20"

// TestImage checks the image reference: the release image with the chart's appVersion by
// default, and the digest instead of the tag when a digest is set.
func TestImage(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "default image is the release image tagged with the chart's appVersion",
			expected: "ghcr.io/openkcm/registry/images/registry:" + appVersion(t),
		},
		{
			name:     "image.tag overrides the appVersion",
			args:     []string{"--set", "image.tag=v1.2.3"},
			expected: "ghcr.io/openkcm/registry/images/registry:v1.2.3",
		},
		{
			name:     "a digest wins over the tag",
			args:     []string{"--set", "image.tag=v1.2.3", "--set", "image.digest=" + digest},
			expected: "ghcr.io/openkcm/registry/images/registry@" + digest,
		},
		{
			name:     "an empty registry uses the repository as the full image name",
			args:     []string{"--set", "image.registry=", "--set", "image.repository=registry", "--set", "image.tag=dev"},
			expected: "registry:dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := renderDeployment(t, tt.args...)
			require.Len(t, d.Spec.Template.Spec.Containers, 1)
			assert.Equal(t, tt.expected, d.Spec.Template.Spec.Containers[0].Image)
		})
	}
}

// TestChecksumConfig checks that the checksum/config annotation, which restarts the pods,
// changes with the configuration and only with it.
func TestChecksumConfig(t *testing.T) {
	checksum := func(t *testing.T, args ...string) string {
		t.Helper()

		sum := renderDeployment(t, args...).Spec.Template.Metadata.Annotations["checksum/config"]
		require.NotEmpty(t, sum, "checksum/config annotation is missing")

		return sum
	}

	defaults := checksum(t)

	assert.NotEqual(t, defaults, checksum(t, "--set", "config.logger.level=debug"),
		"a configuration change must change checksum/config")
	assert.Equal(t, defaults, checksum(t, "--set", "replicaCount=3"),
		"replicaCount must not change checksum/config")
}

// TestConfigMapName checks the name of the configuration ConfigMap and that the Deployment
// mounts it. An immutable ConfigMap gets a hash of the configuration in its name.
func TestConfigMapName(t *testing.T) {
	hashedName := regexp.MustCompile(`^registry-config-[0-9a-f]{10}$`)

	mountedConfigMap := func(t *testing.T, d deployment) string {
		t.Helper()

		for _, v := range d.Spec.Template.Spec.Volumes {
			if v.Name == "config" && v.ConfigMap != nil {
				return v.ConfigMap.Name
			}
		}

		t.Fatal("the Deployment has no config volume")

		return ""
	}

	t.Run("default ConfigMap is mutable and named after the release", func(t *testing.T) {
		cm := renderConfigMap(t)
		assert.Equal(t, "registry-config", cm.Metadata.Name)
		assert.False(t, cm.Immutable)
		assert.Equal(t, cm.Metadata.Name, mountedConfigMap(t, renderDeployment(t)))
	})

	for _, flag := range []string{"configMap.immutable=true", "config.isImmutable=true"} {
		t.Run("immutable ConfigMap gets a hashed name with "+flag, func(t *testing.T) {
			cm := renderConfigMap(t, "--set", flag)
			assert.Regexp(t, hashedName, cm.Metadata.Name)
			assert.True(t, cm.Immutable)
			assert.Equal(t, cm.Metadata.Name, mountedConfigMap(t, renderDeployment(t, "--set", flag)))

			changed := renderConfigMap(t, "--set", flag, "--set", "config.logger.level=debug")
			assert.Regexp(t, hashedName, changed.Metadata.Name)
			assert.NotEqual(t, cm.Metadata.Name, changed.Metadata.Name,
				"a configuration change must change the name of an immutable ConfigMap")
		})
	}
}
