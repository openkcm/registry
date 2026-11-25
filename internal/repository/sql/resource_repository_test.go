package sql_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"github.com/openkcm/registry/internal/repository"
	sqlrepo "github.com/openkcm/registry/internal/repository/sql"
)

// noopDialector is a minimal gorm.Dialector for unit testing without a real database.
type noopDialector struct{}

func (noopDialector) Name() string { return "noop" }
func (d noopDialector) Initialize(db *gorm.DB) error {
	callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{})
	return nil
}
func (noopDialector) Migrator(*gorm.DB) gorm.Migrator                     { return nil }
func (noopDialector) DataTypeOf(*schema.Field) string                     { return "text" }
func (noopDialector) DefaultValueOf(*schema.Field) clause.Expression      { return clause.Expr{SQL: "NULL"} }
func (noopDialector) BindVarTo(w clause.Writer, _ *gorm.Statement, _ any) { _ = w.WriteByte('?') }
func (noopDialector) QuoteTo(w clause.Writer, s string)                   { _, _ = w.WriteString(s) }
func (noopDialector) Explain(s string, _ ...any) string                   { return s }

type testRecord struct{ ID string }

func (testRecord) TableName() string                            { return "records" }
func (testRecord) PaginationKey() map[repository.QueryField]any { return nil }

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(noopDialector{}, &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestHandleQueryField(t *testing.T) {
	t.Run("slice generates IN clause", func(t *testing.T) {
		// given
		db := newTestDB(t)

		// when
		result := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			tx, err := sqlrepo.HandleQueryField(tx, "status", []string{"active", "pending"})
			require.NoError(t, err)
			return tx.Find(&[]testRecord{})
		})

		// then
		assert.Contains(t, result, "status IN")
	})

	t.Run("scalar generates equality clause", func(t *testing.T) {
		// given
		db := newTestDB(t)

		// when
		result := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			tx, err := sqlrepo.HandleQueryField(tx, "id", "abc-123")
			require.NoError(t, err)
			return tx.Find(&[]testRecord{})
		})

		// then
		assert.Contains(t, result, "id = ")
	})

	t.Run("NotEmpty generates IS NOT NULL clause", func(t *testing.T) {
		// given
		db := newTestDB(t)

		// when
		result := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			tx, err := sqlrepo.HandleQueryField(tx, "name", repository.NotEmpty)
			require.NoError(t, err)
			return tx.Find(&[]testRecord{})
		})

		// then
		assert.Contains(t, result, "name IS NOT NULL")
	})

	t.Run("Empty generates IS NULL clause", func(t *testing.T) {
		// given
		db := newTestDB(t)

		// when
		result := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			tx, err := sqlrepo.HandleQueryField(tx, "name", repository.Empty)
			require.NoError(t, err)
			return tx.Find(&[]testRecord{})
		})

		// then
		assert.Contains(t, result, "name IS NULL")
	})

	t.Run("map generates JSONB operator clause", func(t *testing.T) {
		// given
		db := newTestDB(t)

		// when
		result := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			tx, err := sqlrepo.HandleQueryField(tx, "labels", map[string]any{"env": "prod"})
			require.NoError(t, err)
			return tx.Find(&[]testRecord{})
		})

		// then
		assert.Contains(t, result, "labels ->>")
	})

	t.Run("invalid map type returns error", func(t *testing.T) {
		// given
		db := newTestDB(t)

		// when
		_, err := sqlrepo.HandleQueryField(db, "labels", map[string]string{"key": "val"})

		// then
		assert.ErrorIs(t, err, sqlrepo.ErrUnknownTypeForJSONBField)
	})
}

// errSimulatedDB is a sentinel used by newTestDBWithError.
var errSimulatedDB = errors.New("simulated database error")

// newTestDBWithError returns a gorm.DB that injects errSimulatedDB via a
// registered callback before the given operation type runs.
func newTestDBWithError(t *testing.T, operation string) *gorm.DB {
	t.Helper()
	db := newTestDB(t)
	inject := func(d *gorm.DB) { _ = d.AddError(errSimulatedDB) }
	const name = "test:inject_error"
	switch operation {
	case "create":
		require.NoError(t, db.Callback().Create().Before("gorm:begin_transaction").Register(name, inject))
	case "query":
		require.NoError(t, db.Callback().Query().Before("gorm:query").Register(name, inject))
	case "delete":
		require.NoError(t, db.Callback().Delete().Before("gorm:begin_transaction").Register(name, inject))
	case "update":
		require.NoError(t, db.Callback().Update().Before("gorm:begin_transaction").Register(name, inject))
	}
	return db
}

// invalidQuery returns a Query whose CompositeKey contains a map[string]string
// value, which triggers ErrUnknownTypeForJSONBField inside applyQuery.
func invalidQuery() repository.Query {
	q := repository.NewQuery(&testRecord{})
	q.Where(repository.NewCompositeKey().Where("labels", map[string]string{"k": "v"}))
	return *q
}

func TestResourceRepository_Create_DBError(t *testing.T) {
	repo := sqlrepo.NewRepository(newTestDBWithError(t, "create"))
	err := repo.Create(context.Background(), &testRecord{ID: "1"})
	require.Error(t, err)
}

func TestResourceRepository_List_ApplyQueryError(t *testing.T) {
	repo := sqlrepo.NewRepository(newTestDB(t))
	err := repo.List(context.Background(), &[]testRecord{}, invalidQuery())
	require.Error(t, err)
}

func TestResourceRepository_List_FindError(t *testing.T) {
	repo := sqlrepo.NewRepository(newTestDBWithError(t, "query"))
	q := repository.NewQuery(&testRecord{})
	err := repo.List(context.Background(), &[]testRecord{}, *q)
	require.Error(t, err)
}

func TestResourceRepository_Delete_DBError(t *testing.T) {
	repo := sqlrepo.NewRepository(newTestDBWithError(t, "delete"))
	_, err := repo.Delete(context.Background(), &testRecord{ID: "1"})
	require.Error(t, err)
}

func TestResourceRepository_Find_DBError(t *testing.T) {
	repo := sqlrepo.NewRepository(newTestDBWithError(t, "query"))
	found, err := repo.Find(context.Background(), &testRecord{ID: "1"})
	require.Error(t, err)
	assert.False(t, found)
}

func TestResourceRepository_Patch_DBError(t *testing.T) {
	repo := sqlrepo.NewRepository(newTestDBWithError(t, "update"))
	patched, err := repo.Patch(context.Background(), &testRecord{ID: "1"})
	require.Error(t, err)
	assert.False(t, patched)
}

func TestResourceRepository_PatchAll_ApplyQueryError(t *testing.T) {
	repo := sqlrepo.NewRepository(newTestDB(t))
	_, err := repo.PatchAll(context.Background(), &testRecord{ID: "1"}, &[]testRecord{}, invalidQuery())
	require.Error(t, err)
}

func TestResourceRepository_PatchAll_UpdateError(t *testing.T) {
	repo := sqlrepo.NewRepository(newTestDBWithError(t, "update"))
	q := repository.NewQuery(&testRecord{})
	_, err := repo.PatchAll(context.Background(), &testRecord{ID: "1"}, &[]testRecord{}, *q)
	require.Error(t, err)
}
