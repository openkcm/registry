//go:build integration
// +build integration

package integration_test

import (
	"os"
	"time"
)

// getReconciliationTimeout returns an appropriate timeout for reconciliation waits.
// In CI environments, we use a longer timeout to account for resource contention.
func getReconciliationTimeout() time.Duration {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return 45 * time.Second
	}
	return 15 * time.Second
}

// getInitialPollInterval returns the starting poll interval for reconciliation checks.
func getInitialPollInterval() time.Duration {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return 200 * time.Millisecond
	}
	return 100 * time.Millisecond
}

// maxPollInterval is the maximum interval for exponential backoff in reconciliation polling.
const maxPollInterval = 2 * time.Second
