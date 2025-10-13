package model

// OwnerID represents the owner ID of the tenant model.
type OwnerID string

func (o OwnerID) String() string {
	return string(o)
}
