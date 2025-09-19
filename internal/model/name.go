package model

// Name represents the name of the tenant.
type Name string

func (n Name) String() string {
	return string(n)
}
