package tests

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_MigratorBooter_Boot(t *testing.T) {
	t.Run("should no-op if not configured to boot", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		migratorMock := mocks.NewMockMigrator(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			require.NoError(t, factory.Store("my_migrator", migratorMock))
		}))

		assert.NoError(t, app.Boot())
	})

	t.Run("should return migrator retrieval error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorBoot, true)
		_ = config.Set(flam.PathMigrators, flam.Bag{
			"my_migrator": flam.Bag{
				"driver": flam.MigratorDriverDefault}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should execute upAll to migrations if configured to boot", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorBoot, true)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		migratorMock := mocks.NewMockMigrator(ctrl)
		migratorMock.EXPECT().UpAll().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			require.NoError(t, factory.Store("my_migrator", migratorMock))
		}))

		assert.NoError(t, app.Boot())
	})

	t.Run("should return upAll execution error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathMigratorBoot, true)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("expected error")
		migratorMock := mocks.NewMockMigrator(ctrl)
		migratorMock.EXPECT().UpAll().Return(expectedErr)
		require.NoError(t, app.Container().Invoke(func(factory flam.MigratorFactory) {
			require.NoError(t, factory.Store("my_migrator", migratorMock))
		}))

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})
}
