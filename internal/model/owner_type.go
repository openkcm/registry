package model

// OwnerType represents the type of owner for the tenant model.
type OwnerType string

func (o OwnerType) String() string {
	return string(o)
}
