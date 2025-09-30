package model

// AuthStatus represents the status of auth.
type AuthStatus string

// String returns the string representation of the AuthStatus.
func (as AuthStatus) String() string {
	return string(as)
}
