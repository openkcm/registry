package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrMarshalUserGroupValue = errors.New("failed to marshal user_group value")

type (
	UserGroups []string //nolint:recvcheck
)

// Value implements the driver.Valuer interface.
func (u UserGroups) Value() (driver.Value, error) {
	if u == nil {
		return nil, nil //nolint:nilnil
	}
	return json.Marshal(u)
}

// Scan implements the sql.Scanner interface.
func (u *UserGroups) Scan(src any) error {
	if src == nil {
		*u = nil
		return nil
	}

	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("%w: %v", ErrMarshalUserGroupValue, src)
	}

	return json.Unmarshal(bytes, u)
}
