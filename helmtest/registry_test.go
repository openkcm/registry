//go:build helmtest

package helmtest

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	namespace = "default"
)

func TestRegistryPodReady(t *testing.T) {
	t.Log("Testing if registry pod is ready...")

	// Wait for registry pod to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app.kubernetes.io/name=registry",
		"--namespace", namespace,
		"--timeout=300s")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("kubectl wait output: %s", string(output))
		require.NoError(t, err, "Registry pod did not become ready within timeout")
	}

	t.Log("Registry pod is ready")

	// Additional verification: Check pod status
	cmd = exec.Command("kubectl", "get", "pods",
		"-l", "app.kubernetes.io/name=registry",
		"--namespace", namespace,
		"-o", "jsonpath={.items[0].status.phase}")

	output, err = cmd.Output()
	require.NoError(t, err, "Failed to get pod status")

	podPhase := string(output)
	require.Equal(t, "Running", podPhase, "Registry pod is not in Running phase")

	t.Logf("Registry pod is in %s phase", podPhase)
}
