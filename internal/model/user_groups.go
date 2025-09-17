package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrMarshalUserGroupValue = errors.New("failed to marshal user_group value")

type (
	UserGroups []string //nolint:recvcheck
)

// Validate validates given UserGroups of the tenant.
func (u UserGroups) Validate(ctx ValidationContext) error {
	// TODO decide if nil/empty slice is allowed
	// as currently it is at tenant creation time and
	// not at user group update time
	//if len(u) == 0 {
	//	return status.Error(codes.InvalidArgument, "UserGroups is empty")
	//}
	for _, group := range u {
		if strings.ReplaceAll(group, " ", "") == "" {
			return status.Error(codes.InvalidArgument, "UserGroups should not have empty values")
		}
	}

	if ctx == nil {
		return nil
	}

	if err := ctx.ValidateField([]string(u)); err != nil {
		return err
	}

	return nil
}

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
