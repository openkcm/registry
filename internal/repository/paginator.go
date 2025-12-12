package repository

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"time"
)

var (
	ErrInvalidPaginationToken = errors.New("token is invalid")
	ErrFailedToEncodeToken    = errors.New("failed to encode token")
	ErrInvalidFieldName       = errors.New("invalid field name in pagination token")
)

const (
	DefaultPaginationLimit = 50
	maxPaginationLimit     = 1000
)

// Paginator stores the composite key as a single token.
type Paginator struct {
	PageInfo    *PageInfo
	OrderFields []QueryField
}

type PageInfo struct {
	LastCreatedAt time.Time    `json:"lastCreatedAt"`
	LastKey       CompositeKey `json:"lastKey"`
}

// Encode encodes the PageInfo as a page token.
func (p PageInfo) Encode() (string, error) {
	if err := p.validate(); err != nil {
		return "", err
	}

	jsonPaginator, err := json.Marshal(p)
	if err != nil {
		return "", ErrFailedToEncodeToken
	}

	return base64.StdEncoding.EncodeToString(jsonPaginator), nil
}

func (p PageInfo) validate() error {
	if len(p.LastKey) == 0 {
		return ErrInvalidFieldName
	}

	allowList := []string{ExternalIDField, RegionField, IDField, CreatedAtField, SystemIDField}
	for column := range p.LastKey {
		if !slices.Contains(allowList, column) {
			return ErrInvalidFieldName
		}
	}

	return nil
}

// DecodePageToken decodes the token back to a PageInfo struct.
func DecodePageToken(encodedToken string) (*PageInfo, error) {
	bytes, err := base64.StdEncoding.DecodeString(encodedToken)
	if err != nil {
		return nil, ErrInvalidPaginationToken
	}

	decoded := &PageInfo{}

	err = json.Unmarshal(bytes, decoded)
	if err != nil {
		return nil, ErrInvalidPaginationToken
	}

	if err := decoded.validate(); err != nil {
		return nil, err
	}

	return decoded, nil
}
