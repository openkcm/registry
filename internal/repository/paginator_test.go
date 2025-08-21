package repository_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/repository"
)

func TestPaginator(t *testing.T) {
	t.Run("should fail empty token", func(t *testing.T) {
		// given
		pageToken := ""

		// when
		pageInfo, err := repository.DecodePageToken(pageToken)

		// then
		assert.Equal(t, repository.ErrInvalidPaginationToken, err)
		assert.Nil(t, pageInfo)
	})

	t.Run("should fail invalid token", func(t *testing.T) {
		// given
		pageToken := "invalid-token"

		// when
		pageInfo, err := repository.DecodePageToken(pageToken)

		// then
		assert.Equal(t, repository.ErrInvalidPaginationToken, err)
		assert.Nil(t, pageInfo)
	})

	t.Run("should fail with invalid composite key field", func(t *testing.T) {
		// given
		lastKey := repository.CompositeKey{
			"invalidField": "value",
		}
		pageInfo := &repository.PageInfo{
			LastCreatedAt: time.Now(),
			LastKey:       lastKey,
		}

		// when
		encodedToken, err := pageInfo.Encode()

		// then
		assert.Error(t, err)
		assert.Equal(t, repository.ErrInvalidFieldName, err)
		assert.Empty(t, encodedToken)
	})

	t.Run("should fail with empty composite key", func(t *testing.T) {
		// given
		lastKey := repository.CompositeKey{}
		pageInfo := &repository.PageInfo{
			LastCreatedAt: time.Now(),
			LastKey:       lastKey,
		}

		// when
		encodedToken, err := pageInfo.Encode()

		// then
		assert.Error(t, err)
		assert.Equal(t, repository.ErrInvalidFieldName, err)
		assert.Empty(t, encodedToken)
	})

	t.Run("should succeed with valid token", func(t *testing.T) {
		// given
		lastKey := repository.CompositeKey{
			repository.IDField:         uuid.New().String(),
			repository.RegionField:     "region",
			repository.ExternalIDField: "external-id",
		}
		originalPageInfo := &repository.PageInfo{
			LastCreatedAt: time.Now(),
			LastKey:       lastKey,
		}

		// when
		encodedToken, err := originalPageInfo.Encode()
		assert.NoError(t, err)

		decodedPageInfo, err := repository.DecodePageToken(encodedToken)

		// then
		assert.NoError(t, err)
		assert.NotNil(t, decodedPageInfo)
		assert.Equal(t, originalPageInfo.LastCreatedAt.Format(time.RFC3339Nano), decodedPageInfo.LastCreatedAt.Format(time.RFC3339Nano))
		assert.Len(t, decodedPageInfo.LastKey, len(originalPageInfo.LastKey))

		for key, value := range originalPageInfo.LastKey {
			assert.Equal(t, value, decodedPageInfo.LastKey[key])
		}
	})
}
