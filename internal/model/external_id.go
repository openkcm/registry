package model

// ExternalID represents the external ID of the system.
type ExternalID string

func (e ExternalID) String() string {
	return string(e)
}
