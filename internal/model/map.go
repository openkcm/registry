package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/openkcm/registry/internal/validation"
)

// Map represents a map of string key-value pairs.
//
// This type is used to enable JSONB storage in the database
// and to validate map fields in models.
type Map map[string]string //nolint:recvcheck

var _ validation.Map = Map{}

// Value implements the driver.Valuer interface.
func (m Map) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil //nolint:nilnil
	}
	return json.Marshal(m)
}

// Scan implements the sql.Scanner interface.
func (m *Map) Scan(src any) error {
	if src == nil {
		*m = nil
		return nil
	}

	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("%w: %v", ErrMarshalUserGroupValue, src)
	}

	return json.Unmarshal(bytes, m)
}

// Map converts model.Map to a map[string]any.
func (m Map) Map() map[string]any {
	res := make(map[string]any, len(m))
	for k, v := range m {
		res[k] = v
	}
	return res
}
