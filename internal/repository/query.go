package repository

import (
	"log/slog"
	"maps"
	"slices"
)

const (
	IDField         QueryField = "id"
	NameField       QueryField = "name"
	RegionField     QueryField = "region"
	TenantIDField   QueryField = "tenant_id"
	ExternalIDField QueryField = "external_id"
	OwnerIDField    QueryField = "owner_id"
	OwnerTypeField  QueryField = "owner_type"
	StatusField     QueryField = "status"
	CreatedAtField  QueryField = "created_at"

	NotEmpty QueryFieldValue = "not_empty"
	Empty    QueryFieldValue = "empty"
)

// CompositeKey is a collection of QueryField and matching value that are collectively used to find a record.
type CompositeKey map[QueryField]any

// NewCompositeKey creates and returns a new CompositeKey.
func NewCompositeKey() CompositeKey {
	return make(CompositeKey)
}

// Where adds a condition to the CompositeKey.
func (c CompositeKey) Where(q QueryField, v any) CompositeKey {
	c[q] = v
	return c
}

func (c CompositeKey) Validate() error {
	if len(c) == 0 {
		return ErrInvalidFieldName
	}

	whitelist := []string{ExternalIDField, RegionField, IDField, CreatedAtField}
	for column := range c {
		if !slices.Contains(whitelist, column) {
			return ErrInvalidFieldName
		}
	}

	return nil
}

type Query struct {
	// the resource type the query is for
	resource Resource

	// Limit is a max size of returned elements.
	Limit int

	// Paginator handles pagination for list operations if not nil.
	Paginator Paginator

	// OrderFields are the fields used for ordering the results when paginating
	OrderFields []QueryField

	// CompositeKeys  form the where part of the Query
	CompositeKeys []CompositeKey
}

type QueryField = string

type QueryFieldValue = string

// NewQuery creates and returns a new empty query.
func NewQuery(resource Resource) *Query {
	return &Query{
		resource:      resource,
		CompositeKeys: make([]CompositeKey, 0),
	}
}

// Where adds the given CompositeKey to the query.
func (q *Query) Where(compositeKeys ...CompositeKey) *Query {
	q.CompositeKeys = append(q.CompositeKeys, compositeKeys...)
	return q
}

// SetLimit sets the limit value for the query.
func (q *Query) SetLimit(limit int) *Query {
	q.Limit = limit
	return q
}

// ApplyPagination adds pagination parameters if they are provided.
func (q *Query) ApplyPagination(limit int32, token string) error {
	queryLimit := DefaultPaginationLimit
	if limit > 0 {
		queryLimit = min(maxPaginationLimit, int(limit))
	}

	q.Limit = queryLimit

	q.Paginator = Paginator{
		OrderFields: slices.Sorted(maps.Keys(q.resource.PaginationKey())),
	}

	if token == "" {
		return nil
	}

	pageInfo, err := DecodePageToken(token)
	if err != nil {
		slog.Error("failed to decode page token", slog.Any("err", err))
		return err
	}

	q.Paginator.PageInfo = pageInfo

	return nil
}
