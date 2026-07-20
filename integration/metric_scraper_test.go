//go:build integration

package integration_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

var (
	errMetricsServerStatus = errors.New("metrics server returns other status than ok")
	errMetricNotFound      = errors.New("metric was not found")
	errMetricWrongFormat   = errors.New("metric is not in expected format")
	errStatusServerPort    = errors.New("status server address is missing")
)

type metricScraper struct {
	addr string
}

type metric struct {
	name    string
	attribs []string
}

func newMetricScraper() (*metricScraper, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	address := cfg.Status.Address
	if address == "" {
		return nil, errStatusServerPort
	}
	if !strings.HasPrefix(address, "http") {
		address = "http://" + address
	}
	return &metricScraper{
		addr: address + "/metrics",
	}, nil
}

// scrape calls the metrics endpoint,
// scans each metric until the given metric is found,
// and then returns the value of the metric.
func (ms metricScraper) scrape(ctx context.Context, metric metric) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ms.addr, nil)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error scraping metrics: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, errMetricsServerStatus
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if !lineMatches(scanner.Text(), metric) {
			continue
		}
		return parseMetricValue(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error scanning metrics: %w", err)
	}

	return 0, errMetricNotFound
}

func lineMatches(line string, m metric) bool {
	if !strings.Contains(line, m.name) {
		return false
	}
	for _, a := range m.attribs {
		if !strings.Contains(line, a) {
			return false
		}
	}
	return true
}

func parseMetricValue(line string) (int, error) {
	split := strings.Split(line, " ")
	if len(split) != 2 {
		return 0, errMetricWrongFormat
	}
	return strconv.Atoi(split[1])
}

func createMetric(t *testing.T, name string, attr ...string) metric {
	t.Helper()
	// Assert that the number of attributes is even
	if len(attr)%2 != 0 {
		t.Fatalf("number of attributes must be even, got %d", len(attr))
	}

	a := make([]string, 0, len(attr)/2)
	for i := 0; i < len(attr); i += 2 {
		a = append(a, attr[i]+`="`+attr[i+1]+`"`) //nolint:gosec // parity of attr is asserted above
	}
	metric := metric{
		name:    name,
		attribs: a,
	}
	return metric
}
