package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrMarshalLabelValue = errors.New("failed to marshal label value")
)

// Labels are key/value pairs attached to resources such as tenants.
// Labels enable clients to map their own organizational structure onto resources
// in a loosely coupled fashion.
type Labels map[string]string

// Value implements the driver.Valuer interface.
func (l *Labels) Value() (driver.Value, error) {
	if *l == nil {
		return nil, nil //nolint:nilnil
	}

	return json.Marshal(l)
}

// Scan implements the sql.Scanner interface.
func (l *Labels) Scan(src any) error {
	if src == nil {
		*l = make(Labels)
		return nil
	}

	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("%w: %v", ErrMarshalLabelValue, src)
	}

	return json.Unmarshal(bytes, l)
}
