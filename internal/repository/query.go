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
	SystemIDField   QueryField = "system_id"
	OwnerIDField    QueryField = "owner_id"
	OwnerTypeField  QueryField = "owner_type"
	CreatedAtField  QueryField = "created_at"
	TypeField       QueryField = "type"
	LabelsField     QueryField = "labels"

	NotEmpty QueryFieldValue = "not_empty"
	Empty    QueryFieldValue = "empty"

	System FieldName = "System"
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

type Join struct {
	Resource Resource
	OnColumn QueryField
	Column   QueryField
}

type Query struct {
	// the Resource type the query is for
	Resource Resource

	// Limit is a max size of returned elements.
	Limit int

	// Paginator handles pagination for list operations if not nil.
	Paginator Paginator

	// OrderFields are the fields used for ordering the results when paginating
	OrderFields []QueryField

	// CompositeKeys  form the where part of the Query
	CompositeKeys []CompositeKey

	// Joins are the resources to be joined with the main resource
	Joins []Join

	// Preloads are the field names to be preloaded with the main resource
	Preloads []FieldName
}

type QueryField = string

type QueryFieldValue = string

type FieldName = string

// NewQuery creates and returns a new empty query.
func NewQuery(resource Resource) *Query {
	return &Query{
		Resource:      resource,
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
		OrderFields: slices.Sorted(maps.Keys(q.Resource.PaginationKey())),
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

// Populate fills the Preloads slice with field names that are needed to fetched with the main resource.
func (q *Query) Populate(fieldNames ...string) {
	q.Preloads = fieldNames
}
