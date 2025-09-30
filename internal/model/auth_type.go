package model

// AuthType represents the type of auth.
type AuthType string

// String returns the string representation of the AuthType.
func (at AuthType) String() string {
	return string(at)
}
