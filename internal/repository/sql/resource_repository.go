package sql

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/openkcm/registry/internal/repository"
)

const (
	pqUniqueViolationErrCode = "23505" // see https://www.postgresql.org/docs/14/errcodes-appendix.html
)

var (
	ErrUnknownTypeForJSONBField = errors.New("unknown type for jsonb field")
)

// ResourceRepository represents the repository for managing Resource data.
type ResourceRepository struct {
	db *gorm.DB
}

// NewRepository creates and returns a new instance of ResourceRepository.
func NewRepository(db *gorm.DB) *ResourceRepository {
	return &ResourceRepository{
		db: db,
	}
}

// Create adds meta information and stores a Resource.
func (r ResourceRepository) Create(ctx context.Context, resource repository.Resource) error {
	result := r.db.WithContext(ctx).Create(resource)
	if result.Error != nil {
		slog.Error("error creating resource", slog.Any("err", result.Error))

		var pgError *pgconn.PgError
		if errors.As(result.Error, &pgError) && pgError.Code == pqUniqueViolationErrCode {
			return &repository.UniqueConstraintError{
				Detail: pgError.Detail,
			}
		}

		return result.Error
	}

	return nil
}

// List retrieves records from the database based on the provided query parameters and model.
func (r ResourceRepository) List(ctx context.Context, result any, query repository.Query) error {
	dbQuery := r.db.WithContext(ctx).Model(result)
	dbQuery, err := applyQuery(dbQuery, query)
	if err != nil {
		slog.Error("error applying query for listing resources", slog.Any("err", err))
		return err
	}

	err = dbQuery.Find(result).Error
	if err != nil {
		slog.Error("error listing resources", slog.Any("err", err))
		return err
	}

	return nil
}

// Delete removes the Resource.
//
// It returns true if a record was deleted successfully,
// false if there was no record to delete,
// and error if there was an error during the deletion.
func (r ResourceRepository) Delete(ctx context.Context, resource repository.Resource) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.Returning{}).Delete(resource)
	if result.Error != nil {
		slog.Error("error deleting resource", slog.Any("err", result.Error))
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}

// Find fill given Resource with data, if found. Given Resource is used as query data.
func (r ResourceRepository) Find(ctx context.Context, resource repository.Resource) (bool, error) {
	result := r.db.WithContext(ctx).Limit(1).Find(resource)
	if result.Error != nil {
		slog.Error("error finding a resource", slog.Any("err", result.Error))
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}

// Patch will patch the resource with primary key as the where condition.
//
// It returns true if a record was patched successfully,
// and error if there was an error during the patch.
func (r ResourceRepository) Patch(ctx context.Context, resource repository.Resource) (bool, error) {
	db := r.db.WithContext(ctx).Clauses(clause.Returning{}).Updates(resource)
	if db.Error != nil {
		slog.Error("error updating resource", slog.Any("err", db.Error))
		return false, db.Error
	}

	return db.RowsAffected > 0, nil
}

// PatchAll will update all the resources that matches the query.
// It returns the number of affected rows
// and error if there was an error during the patch operation.
func (r ResourceRepository) PatchAll(ctx context.Context, resource repository.Resource, result any, query repository.Query) (int64, error) {
	db := r.db.WithContext(ctx).Model(result).Clauses(clause.Returning{})
	db, err := applyQuery(db, query)
	if err != nil {
		slog.Error("error applying query for updating resources", slog.Any("err", err))
		return 0, err
	}

	db = db.Updates(resource)
	if db.Error != nil {
		slog.Error("error updating resources", slog.Any("err", db.Error))
		return db.RowsAffected, db.Error
	}

	return db.RowsAffected, nil
}

// Transaction will give transaction locking on particular rows.
// txFunc is a type TransactionFunc where we can define the transactional logic.
// if txFunc return no error then transaction is committed,
// else if txFunc return error then transaction is rolled back.
// Note: please dont use Goroutines inside the txFunc as this might lead to panic.
func (r ResourceRepository) Transaction(ctx context.Context, txFunc repository.TransactionFunc) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		errorChan := make(chan error)

		go func() {
			errorChan <- txFunc(ctx, NewRepository(tx.Clauses(clause.Locking{Strength: "UPDATE"})))
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errorChan:
			return err
		}
	})
}

// applyQuery applies the query to the database.
func applyQuery(db *gorm.DB, query repository.Query) (*gorm.DB, error) {
	if len(query.CompositeKeys) > 0 {
		baseQuery := db.Session(&gorm.Session{NewDB: true})

		for i, ck := range query.CompositeKeys {
			tx, err := handleCompositeKey(db, ck)
			if err != nil {
				return nil, err
			}
			if i == 0 {
				baseQuery = baseQuery.Where(tx)
				continue
			}

			baseQuery = baseQuery.Or(tx)
		}

		db = db.Where(baseQuery)
	}

	if query.Limit <= 0 {
		query.Limit = repository.DefaultPaginationLimit
	}

	return handlePagination(query.Paginator, db).Limit(query.Limit), nil
}

// handleCompositeKey applies the composite key to the query.
func handleCompositeKey(db *gorm.DB, compositeKey repository.CompositeKey) (*gorm.DB, error) {
	tx := db.Session(&gorm.Session{NewDB: true})

	for field, value := range compositeKey {
		var err error
		tx, err = handleQueryField(tx, field, value)
		if err != nil {
			return nil, err
		}
	}

	return tx, nil
}

// handleQueryField applies the query field to the query.
func handleQueryField(tx *gorm.DB, field repository.QueryField, value any) (*gorm.DB, error) {
	switch value {
	case repository.NotEmpty:
		tx = tx.Where(field+" IS NOT NULL").Where(field+" != ?", "")
	case repository.Empty:
		tx = tx.Where(field+" IS NULL OR "+field+" = ?", "")
	default:
		switch reflect.ValueOf(value).Kind() { //nolint:exhaustive
		case reflect.Slice:
		case reflect.Array:
			tx = tx.Where(field+" IN ?", value)
		case reflect.Map:
			labels, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%w: %T", ErrUnknownTypeForJSONBField, value)
			}
			for k, v := range labels {
				tx = tx.Where(field+" ->> ? = ?", k, v)
			}
		default:
			tx = tx.Where(field+" = ?", value)
		}
	}
	return tx, nil
}

// handlePagination applies pagination to the query.
func handlePagination(paginator repository.Paginator, db *gorm.DB) *gorm.DB {
	orderBy := []string{repository.CreatedAtField + " DESC"}
	for _, val := range paginator.OrderFields {
		orderBy = append(orderBy, val+" DESC")
	}

	db = db.Order(strings.Join(orderBy, ", "))

	if paginator.PageInfo == nil {
		return db
	}

	pageInfo := paginator.PageInfo
	fields := []repository.QueryField{repository.CreatedAtField}

	args := make([]any, 0, len(pageInfo.LastKey)+1)
	for field := range pageInfo.LastKey {
		fields = append(fields, field)
	}

	slices.Sort(fields)
	placeholderSlice := make([]string, len(fields))

	for i, field := range fields {
		placeholderSlice[i] = "?"

		if field == repository.CreatedAtField {
			args = append(args, pageInfo.LastCreatedAt)
			continue
		}

		args = append(args, pageInfo.LastKey[field])
	}

	condition := fmt.Sprintf("(%s) < (%s)", strings.Join(fields, ", "), strings.Join(placeholderSlice, ", "))

	return db.Where(condition, args...)
}
