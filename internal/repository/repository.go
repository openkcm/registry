package repository

import (
	"context"
)

// TransactionFunc is func signature for ExecTransaction.
type TransactionFunc func(context.Context, Repository) error

// Repository defines the interface for Repository operations.
type Repository interface {
	Create(ctx context.Context, resource Resource) error
	List(ctx context.Context, result any, query Query) error
	Delete(ctx context.Context, resource Resource) (bool, error)
	Find(ctx context.Context, resource Resource) (bool, error)
	Patch(ctx context.Context, resource Resource) (bool, error)
	// Count(ctx context.Context, resource Resource, query Query) (int64, error)
	PatchAll(ctx context.Context, resource Resource, result any, query Query) (int64, error)
	Transaction(ctx context.Context, txFunc TransactionFunc) error
}

// Resource defines the interface for Resource operations.
type Resource interface {
	TableName() string
	PaginationKey() map[QueryField]any
}

// UniqueConstraintError represents an error caused by a violation of a unique constraint in the database.
type UniqueConstraintError struct {
	Detail string
}

// Error returns an error message describing the unique constraint violation.
func (u *UniqueConstraintError) Error() string {
	return "resource must be unique: " + u.Detail
}
