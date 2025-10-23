package model

// ID represents the ID of a resource.
type ID string

func (i ID) String() string {
	return string(i)
}
