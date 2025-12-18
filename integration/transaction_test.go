//go:build integration
// +build integration

package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
	"github.com/openkcm/registry/internal/repository/sql"
)

var errSomething = errors.New("just throwing error :)")

func TestExecuteTransactionError(t *testing.T) {
	// given
	db, err := startDB()
	require.NoError(t, err)
	subj := sql.NewRepository(db)
	ctx := t.Context()

	expSys1 := model.NewSystem(validRandID(), allowedSystemType)

	err = db.Create(&expSys1).Error
	assert.NoError(t, err)
	defer db.Delete(expSys1)

	expSys2 := model.NewSystem(validRandID(), allowedSystemType)
	err = db.Create(&expSys2).Error
	assert.NoError(t, err)
	defer db.Delete(expSys2)

	t.Run("should time out if transaction takes more than defined timeout", func(t *testing.T) {
		// given
		ctxTimeOut, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		// when
		err := subj.Transaction(ctxTimeOut,
			func(ctx context.Context, r repository.Repository) error {
				// Find by Id
				_, err := r.Find(ctx, &model.System{ExternalID: expSys1.ExternalID})
				if err != nil {
					return err
				}

				// sleeping for timeout
				time.Sleep(time.Hour)

				// Patching the model
				tenantID := ""
				_, err = r.Patch(ctx, &model.System{
					ExternalID: expSys1.ExternalID,
					TenantID:   &tenantID,
				})
				return err
			})
		// then
		assert.Equal(t, context.DeadlineExceeded, err)
	})

	t.Run("should able to do transaction for the same row after a error in first transaction", func(t *testing.T) {
		// when
		err := subj.Transaction(ctx,
			func(ctx context.Context, r repository.Repository) error {
				// Find by Id
				_, err := r.Find(ctx, &model.System{ID: expSys1.ID})
				if err != nil {
					return err
				}
				return errSomething
			})

		// then
		assert.Error(t, err)

		// given
		newTenantID := uuid.New().String()

		// when
		err = subj.Transaction(ctx,
			func(ctx context.Context, r repository.Repository) error {
				// Find by Id
				_, err := r.Find(ctx, &model.System{ID: expSys1.ID})
				if err != nil {
					return err
				}
				// Patching the model
				_, err = r.Patch(ctx, &model.System{
					ID:       expSys1.ID,
					TenantID: &newTenantID,
				})
				return err
			})

		// then
		assert.NoError(t, err)

		actSys := model.System{
			ID: expSys1.ID,
		}
		_, err = subj.Find(ctx, &actSys)
		assert.NoError(t, err)
		assert.Equal(t, newTenantID, *actSys.TenantID)
	})
}

func TestExecuteTransactionRaceConditions(t *testing.T) {
	// given
	db, err := startDB()
	require.NoError(t, err)
	subj := sql.NewRepository(db)
	ctx := t.Context()

	expSys1 := model.NewSystem(validRandID(), allowedSystemType)

	err = db.Create(&expSys1).Error
	assert.NoError(t, err)
	defer db.Delete(expSys1)

	expSys2 := model.NewSystem(validRandID(), allowedSystemType)

	err = db.Create(&expSys2).Error
	assert.NoError(t, err)
	defer db.Delete(expSys2)

	t.Run("select all records without transaction should not wait for the 1st transaction", func(t *testing.T) {
		// given
		transactor1 := make(chan string)
		defer close(transactor1)

		transactor2 := make(chan string)
		defer close(transactor2)

		wg := sync.WaitGroup{}
		wg.Add(2)

		newTenantID := uuid.New().String()
		go func() {
			assert.Equal(t, "1st transaction start", <-transactor1)
			defer wg.Done()
			err := subj.Transaction(ctx,
				func(ctx context.Context, r repository.Repository) error {
					// Find by Id
					_, err := r.Find(ctx, &model.System{ID: expSys1.ID})
					if err != nil {
						return err
					}

					// waiting for transaction
					transactor1 <- "1st transaction waiting before patch"
					assert.Equal(t, "1st transaction continue patch", <-transactor1)
					// Patching the model
					_, err = r.Patch(ctx, &model.System{
						ID:       expSys1.ID,
						TenantID: &newTenantID,
					})
					return err
				})
			assert.NoError(t, err)
			transactor1 <- "1st transaction finished"
		}()

		go func() {
			assert.Equal(t, "2nd db list start", <-transactor2)
			defer wg.Done()
			var result []model.System
			err := subj.List(ctx, &result, *repository.NewQuery(&model.System{}))

			// then
			assert.NoError(t, err)
			assert.Len(t, result, 2)
			transactor2 <- "2nd db list finished"
		}()

		// when
		transactor1 <- "1st transaction start"
		assert.Equal(t, "1st transaction waiting before patch", <-transactor1)
		transactor2 <- "2nd db list start"
		assert.Equal(t, "2nd db list finished", <-transactor2)
		transactor1 <- "1st transaction continue patch"
		assert.Equal(t, "1st transaction finished", <-transactor1)

		// then
		wg.Wait()
		actSys := model.System{
			ID: expSys1.ID,
		}
		_, err = subj.Find(ctx, &actSys)
		assert.NoError(t, err)
		assert.Equal(t, newTenantID, *actSys.TenantID)
	})

	t.Run("second transaction should wait when updating the same record", func(t *testing.T) {
		// given
		transactor1 := make(chan string)
		defer close(transactor1)

		transactor2 := make(chan string)
		defer close(transactor2)

		wg := sync.WaitGroup{}
		wg.Add(2)

		// when
		newTenantID1 := uuid.New().String()
		go func() {
			assert.Equal(t, "1st transaction start", <-transactor1)
			defer wg.Done()
			err := subj.Transaction(ctx,
				func(ctx context.Context, r repository.Repository) error {
					// Find by Id
					_, err := r.Find(ctx, &model.System{ID: expSys1.ID})
					if err != nil {
						return err
					}

					transactor1 <- "1st transaction wait before patch"
					// waiting for transaction
					assert.Equal(t, "1st transaction continue patch", <-transactor1)
					// Patching the model
					_, err = r.Patch(ctx, &model.System{
						ID:       expSys1.ID,
						TenantID: &newTenantID1,
					})
					transactor1 <- "1st transaction finish"
					return err
				})
			assert.NoError(t, err)
		}()

		newTenantID2 := uuid.New().String()
		go func() {
			assert.Equal(t, "2nd transaction start", <-transactor2)
			defer wg.Done()
			err := subj.Transaction(ctx,
				func(ctx context.Context, r repository.Repository) error {
					transactor2 <- "2nd transaction before lock"
					// Find by Id
					actSys := model.System{ID: expSys1.ID}
					_, err := r.Find(ctx, &actSys)
					if err != nil {
						return err
					}
					transactor2 <- "2nd transaction after lock"
					// then
					assert.Equal(t, newTenantID1, *actSys.TenantID)

					// Patching the model
					_, err = r.Patch(ctx, &model.System{
						ID:       expSys1.ID,
						TenantID: &newTenantID2,
					})
					return err
				})
			assert.NoError(t, err)
			transactor2 <- "2nd transaction finish"
		}()

		// when
		transactor1 <- "1st transaction start"
		assert.Equal(t, "1st transaction wait before patch", <-transactor1)
		transactor2 <- "2nd transaction start"
		assert.Equal(t, "2nd transaction before lock", <-transactor2)
		transactor1 <- "1st transaction continue patch"
		assert.Equal(t, "1st transaction finish", <-transactor1)
		assert.Equal(t, "2nd transaction after lock", <-transactor2)
		assert.Equal(t, "2nd transaction finish", <-transactor2)

		// then
		wg.Wait()
		actSys := model.System{
			ID: expSys1.ID,
		}
		_, err = subj.Find(ctx, &actSys)
		assert.NoError(t, err)
		assert.Equal(t, newTenantID2, *actSys.TenantID)
	})
}

func TestExecuteTransactionWithoutRaceConditions(t *testing.T) {
	// given
	db, err := startDB()
	require.NoError(t, err)
	subj := sql.NewRepository(db)
	ctx := t.Context()

	expSys1 := model.NewSystem(validRandID(), allowedSystemType)

	err = db.Create(&expSys1).Error
	assert.NoError(t, err)
	defer db.Delete(expSys1)

	expSys2 := model.NewSystem(validRandID(), allowedSystemType)

	err = db.Create(&expSys2).Error
	assert.NoError(t, err)
	defer db.Delete(expSys2)

	t.Run("second transaction should not wait when updating different record", func(t *testing.T) {
		// given
		transactor1 := make(chan string)
		defer close(transactor1)

		transactor2 := make(chan string)
		defer close(transactor2)

		wg := sync.WaitGroup{}
		wg.Add(2)

		// when
		newTenantID1 := uuid.New().String()
		go func() {
			assert.Equal(t, "1st transaction start", <-transactor1)
			defer wg.Done()
			err := subj.Transaction(ctx,
				func(ctx context.Context, r repository.Repository) error {
					// Find by Id
					actSys := model.System{ID: expSys1.ID}
					_, err := r.Find(ctx, &actSys)
					if err != nil {
						return err
					}

					transactor1 <- "1st transaction wait before patch"
					assert.Equal(t, "1st transaction continue patch", <-transactor1)
					// Patching the model
					_, err = r.Patch(ctx, &model.System{
						ID:       expSys1.ID,
						TenantID: &newTenantID1,
					})
					return err
				})
			assert.NoError(t, err)
			transactor1 <- "1st transaction finish"
		}()

		newTenantID2 := uuid.New().String()
		go func() {
			assert.Equal(t, "2nd transaction start", <-transactor2)
			defer wg.Done()
			err := subj.Transaction(ctx,
				func(ctx context.Context, r repository.Repository) error {
					// Find by Id
					actSys := model.System{
						ID: expSys2.ID,
					}
					_, err := r.Find(ctx, &actSys)
					if err != nil {
						return err
					}

					transactor2 <- "2nd transaction wait before patch"
					assert.Equal(t, "2nd transaction continue patch", <-transactor2)
					// Patching the model
					_, err = r.Patch(ctx, &model.System{
						ID:       expSys2.ID,
						TenantID: &newTenantID2,
					})
					return err
				})
			assert.NoError(t, err)

			transactor2 <- "2nd transaction finish"
		}()

		// when
		transactor1 <- "1st transaction start"
		transactor2 <- "2nd transaction start"
		assert.Equal(t, "1st transaction wait before patch", <-transactor1)
		assert.Equal(t, "2nd transaction wait before patch", <-transactor2)
		transactor1 <- "1st transaction continue patch"
		transactor2 <- "2nd transaction continue patch"
		assert.Equal(t, "1st transaction finish", <-transactor1)
		assert.Equal(t, "2nd transaction finish", <-transactor2)

		// then
		wg.Wait()
		actSys1 := model.System{
			ID: expSys1.ID,
		}
		_, err = subj.Find(ctx, &actSys1)
		assert.NoError(t, err)
		assert.Equal(t, newTenantID1, *actSys1.TenantID)

		actSys2 := model.System{
			ID: expSys2.ID,
		}
		_, err = subj.Find(ctx, &actSys2)
		assert.NoError(t, err)
		assert.Equal(t, newTenantID2, *actSys2.TenantID)
	})
}
