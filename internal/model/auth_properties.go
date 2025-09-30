package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// AuthProperties represents the properties of auth.
type AuthProperties map[string]string //nolint:recvcheck

// Value implements the driver.Valuer interface.
func (ap AuthProperties) Value() (driver.Value, error) {
	if ap == nil {
		return nil, nil //nolint:nilnil
	}
	return json.Marshal(ap)
}

// Scan implements the sql.Scanner interface.
func (ap *AuthProperties) Scan(src any) error {
	if src == nil {
		*ap = nil
		return nil
	}

	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("%w: %v", ErrMarshalUserGroupValue, src)
	}

	return json.Unmarshal(bytes, ap)
}
