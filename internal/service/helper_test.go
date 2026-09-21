package service_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"

	stdsql "database/sql"
	orbsql "github.com/openkcm/orbital/store/sql"

	"github.com/openkcm/registry/internal/service"
)

const (
	testDBUser     = "postgres"
	testDBPassword = "secret"
	testDBName     = "orbital"
	testDBSSLMode  = "disable"
)

var testDBPort string

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase(testDBName),
		postgres.WithUsername(testDBUser),
		postgres.WithPassword(testDBPassword),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Printf("failed to start PostgreSQL container: %v", err)
		os.Exit(1)
	}

	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		log.Printf("failed to get mapped port: %v", err)
		os.Exit(1)
	}
	testDBPort = mappedPort.Port()

	code := m.Run()

	if err := container.Terminate(ctx); err != nil {
		log.Printf("failed to terminate container: %v", err)
	}
	os.Exit(code)
}

// newTestOrbital creates a *service.Orbital backed by a real PostgreSQL store.
// The caller receives the underlying *stdsql.DB to run verification queries.
// The jobs table is cleared automatically when the test ends.
func newTestOrbital(t *testing.T) (*service.Orbital, *stdsql.DB) {
	t.Helper()
	ctx := t.Context()

	connStr := fmt.Sprintf("host=localhost port=%s user=%s password=%s dbname=%s sslmode=%s",
		testDBPort, testDBUser, testDBPassword, testDBName, testDBSSLMode)
	db, err := stdsql.Open("pgx", connStr)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM jobs")
		_ = db.Close()
	})

	store, err := orbsql.New(ctx, db)
	require.NoError(t, err)

	repo := orbital.NewRepository(store)
	manager, err := orbital.NewManager(repo,
		func(_ context.Context, _ orbital.Job, _ orbital.TaskResolverCursor) (orbital.TaskResolverResult, error) {
			return orbital.ContinueTaskResolver(), nil
		},
	)
	require.NoError(t, err)

	return service.NewOrbitalWithManagerForTest(manager), db
}

// assertJobPrepared verifies that exactly one job with the given externalID and type
// exists in the orbital jobs table.
func assertJobPrepared(t *testing.T, db *stdsql.DB, externalID, jobType string) {
	t.Helper()
	var count int
	err := db.QueryRowContext(t.Context(),
		"SELECT COUNT(*) FROM jobs WHERE external_id = $1 AND type = $2",
		externalID, jobType,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "expected exactly one job for externalID=%s type=%s", externalID, jobType)
}
