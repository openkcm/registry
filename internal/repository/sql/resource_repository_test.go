package sql_test

import (
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

func (testRecord) TableName() string { return "records" }

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
